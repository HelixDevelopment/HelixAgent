package netaddr

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"digital.vasic.llmsverifier/pkg/helixendpoint"
	"github.com/stretchr/testify/require"
)

// helixEndpointDialAddressForTest calls the upstream function directly so
// TestDialAddress_MatchesHelixEndpointDialAddress can prove netaddr.DialAddress
// is a verbatim re-export, not a parallel copy that could drift.
func helixEndpointDialAddressForTest(host string, port int) string {
	return helixendpoint.DialAddress(host, port)
}

// STEP 3 (§11.4.146): enumerate the address forms this package's callers
// hand it — bracketed, unbracketed, zone-qualified, hostname, empty,
// malformed — with per-case outcomes. Table-driven so every case is a
// named, independently reportable sub-test.
func TestDialAddress_AddressFormEnumeration(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		port    int
		want    string
		explain string
	}{
		{
			name:    "bracketed_ipv6",
			host:    "[::1]",
			port:    6333,
			want:    "[::1]:6333",
			explain: "the exact HXC-286 killer input: an already-bracketed IPv6 literal must not be double-bracketed nor have its brackets ignored",
		},
		{
			name:    "unbracketed_ipv6",
			host:    "::1",
			port:    6333,
			want:    "[::1]:6333",
			explain: "a bare IPv6 literal must be bracketed so the port separator is unambiguous",
		},
		{
			name: "unbracketed_ipv6_full_form",
			host: "2001:db8::1",
			port: 9042,
			want: "[2001:db8::1]:9042",
		},
		{
			name:    "ipv4_literal",
			host:    "127.0.0.1",
			port:    6379,
			want:    "127.0.0.1:6379",
			explain: "IPv4 has no colon, so it is unaffected by bracketing logic — byte-identical to the naive fmt.Sprintf output",
		},
		{
			name: "hostname",
			host: "qdrant.internal.example",
			port: 6333,
			want: "qdrant.internal.example:6333",
		},
		{
			name:    "zone_qualified_ipv6",
			host:    "fe80::1%eth0",
			port:    5672,
			want:    "[fe80::1%eth0]:5672",
			explain: "net.JoinHostPort treats the whole host as opaque and performs no zone-splitting of its own, so this package inherits neither the llms_verifier first-'%' convention nor a conflicting one — it never disagrees with the Go stdlib because it never parses the zone at all",
		},
		{
			name:    "empty_host",
			host:    "",
			port:    6333,
			want:    "" + ":" + "6333",
			explain: "no default-substitution: an empty host is passed straight through so the caller's own dial fails loudly against the actual empty value, never a silently-swapped placeholder host",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DialAddress(tc.host, tc.port)
			require.Equal(t, tc.want, got, tc.explain)
		})
	}
}

// TestDialAddress_NoDefaultSubstitution is the load-bearing anti-bluff
// assertion for the default-substitution DECISION this package makes: a
// malformed or empty host must NEVER be silently replaced by a different
// host. This is what distinguishes netaddr.DialAddress from
// helixendpoint.BaseURL, and it is exactly the property that matters for a
// third-party service HelixAgent does not own.
func TestDialAddress_NoDefaultSubstitution(t *testing.T) {
	got := DialAddress("", 6333)
	require.NotContains(t, got, "localhost",
		"an empty host must never be silently substituted with HelixAgent's own placeholder host")
	require.Contains(t, got, "6333",
		"the caller's port must survive even when the host is unusable, so the failure is visibly about the HOST")
}

// TestBaseURL_ComposesSchemeOverDialAddress asserts BaseURL is a thin
// composition on top of DialAddress (never a parallel reimplementation of
// the bracket-strip-then-join logic).
//
// The non-http rows are a coverage gap a 2026-08-12 review measured and this
// revision closes. The scheme assertion below was always correct, but EVERY
// case fed it "http", so the scheme parameter was never actually exercised:
// hardcoding the scheme (`scheme + "://"` -> `"http" + "://"`) SURVIVED this
// package's entire suite and was caught only incidentally, one layer up, by
// internal/ports' TestHelpers_FormatsAreCorrect — even though
// internal/ports/ports.go's HTTPSURL calls BaseURL("https", ...) in
// production. A mutant that only a DIFFERENT package's test can kill is not
// covered here (§1.1). Adding a non-http scheme makes the mutation die in
// netaddr's own suite.
func TestBaseURL_ComposesSchemeOverDialAddress(t *testing.T) {
	cases := []struct {
		name   string
		scheme string
		host   string
		port   int
		want   string
	}{
		{"http_bracketed_ipv6", "http", "[::1]", 6333, "http://[::1]:6333"},
		{"http_unbracketed_ipv6", "http", "::1", 8000, "http://[::1]:8000"},
		{"http_hostname", "http", "chroma.internal", 8000, "http://chroma.internal:8000"},
		{"http_zone_qualified_ipv6", "http", "fe80::1%eth0", 5672, "http://[fe80::1%25eth0]:5672"},
		// Non-http schemes: the production shape internal/ports' HTTPSURL uses,
		// plus a non-TLS non-http scheme so the guard is about the PARAMETER
		// rather than about the single literal "https".
		{"https_hostname", "https", "qdrant.internal", 6333, "https://qdrant.internal:6333"},
		{"https_unbracketed_ipv6", "https", "::1", 8100, "https://[::1]:8100"},
		{"https_zone_qualified_ipv6", "https", "fe80::1%eth0", 8100, "https://[fe80::1%25eth0]:8100"},
		{"amqp_hostname", "amqp", "rabbit.internal", 5672, "amqp://rabbit.internal:5672"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BaseURL(tc.scheme, tc.host, tc.port)
			require.Equal(t, tc.want, got)
			require.True(t, strings.HasPrefix(got, tc.scheme+"://"))
		})
	}
}

// TestBaseURL_ZoneQualifiedHostParses is the STEP-3 finding this fix closes:
// a zone-qualified IPv6 host, composed into a URL, must actually be
// parseable by the reader that will consume it — round-tripping back to the
// exact original host. Discovered empirically (not assumed) by the
// address-form enumeration in internal/search/store's RED test, which failed
// against an earlier revision of this package that left the zone raw.
func TestBaseURL_ZoneQualifiedHostParses(t *testing.T) {
	got := BaseURL("http", "fe80::1%eth0", 6333)

	u, err := url.Parse(got)
	require.NoError(t, err, "a zone-qualified BaseURL output %q must be parseable", got)
	require.Equal(t, "fe80::1%eth0", u.Hostname(),
		"the zone must round-trip back to the exact original value")
	require.Equal(t, "6333", u.Port())
}

// TestBaseURL_AlreadyEncodedZoneIsNotDoubleEncoded asserts idempotency: a
// caller that already spelled the zone with its RFC 6874 "%25" prefix must
// not have it encoded a second time into "%2525".
func TestBaseURL_AlreadyEncodedZoneIsNotDoubleEncoded(t *testing.T) {
	got := BaseURL("http", "fe80::1%25eth0", 6333)
	require.Equal(t, "http://[fe80::1%25eth0]:6333", got)
	require.NotContains(t, got, "%2525")

	u, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, "fe80::1%eth0", u.Hostname())
}

// TestDialAddressString_AddressFormEnumeration mirrors
// TestDialAddress_AddressFormEnumeration for the string-port variant used by
// callers whose config types the port as a string (e.g. internal/cache's
// RedisConfig.Port).
func TestDialAddressString_AddressFormEnumeration(t *testing.T) {
	cases := []struct {
		name string
		host string
		port string
		want string
	}{
		{"bracketed_ipv6", "[::1]", "6379", "[::1]:6379"},
		{"unbracketed_ipv6", "::1", "6379", "[::1]:6379"},
		{"ipv4_literal", "127.0.0.1", "6379", "127.0.0.1:6379"},
		{"hostname", "redis.internal", "6379", "redis.internal:6379"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "6379", "[fe80::1%eth0]:6379"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, DialAddressString(tc.host, tc.port))
		})
	}
}

// TestDialAddressForURL_MatchesBaseURLAuthority asserts DialAddressForURL is
// exactly the authority-component half of BaseURL (§11.4.74 — one
// definition, two entry points, never two implementations).
func TestDialAddressForURL_MatchesBaseURLAuthority(t *testing.T) {
	cases := []struct {
		host string
		port int
	}{
		{"[::1]", 6333}, {"::1", 6333}, {"fe80::1%eth0", 5672},
		{"127.0.0.1", 80}, {"example.com", 443},
	}
	for _, tc := range cases {
		want := BaseURL("http", tc.host, tc.port)
		got := "http://" + DialAddressForURL(tc.host, tc.port)
		require.Equal(t, want, got)
	}
}

// TestBaseURLString_AddressFormEnumeration mirrors
// TestBaseURL_ComposesSchemeOverDialAddress for the string-port variant,
// including the zone-encoding case (the same one DialAddressString's own
// enumeration does NOT need, because DialAddressString is never fed to a
// URL parser).
// The non-http rows close the same scheme-parameter coverage gap documented on
// TestBaseURL_ComposesSchemeOverDialAddress: every case previously passed the
// literal "http", so a hardcoded scheme in BaseURLString was unasserted here.
func TestBaseURLString_AddressFormEnumeration(t *testing.T) {
	cases := []struct {
		name   string
		scheme string
		host   string
		port   string
		want   string
	}{
		{"bracketed_ipv6", "http", "[::1]", "18434", "http://[::1]:18434"},
		{"unbracketed_ipv6", "http", "::1", "18434", "http://[::1]:18434"},
		{"hostname", "http", "helixllm.internal", "18434", "http://helixllm.internal:18434"},
		{"zone_qualified_ipv6", "http", "fe80::1%eth0", "18434", "http://[fe80::1%25eth0]:18434"},
		{"https_hostname", "https", "helixllm.internal", "18434", "https://helixllm.internal:18434"},
		{"https_unbracketed_ipv6", "https", "::1", "18434", "https://[::1]:18434"},
		{"amqp_hostname", "amqp", "rabbit.internal", "5672", "amqp://rabbit.internal:5672"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BaseURLString(tc.scheme, tc.host, tc.port)
			require.Equal(t, tc.want, got)
			require.True(t, strings.HasPrefix(got, tc.scheme+"://"),
				"case %q: the scheme argument must reach the output, not a hardcoded literal", tc.name)

			u, err := url.Parse(got)
			require.NoError(t, err, "case %q: %q must be parseable", tc.name, got)
			require.Equal(t, tc.port, u.Port())
		})
	}
}

// TestUrlEncodeZone_GenuineNumericZone25_NotMisreadAsEncoded is the
// permanent regression guard for MUST-FIX 1 (2026-08-12 review). An earlier
// revision decided "already RFC 6874-encoded" with a bare
// strings.HasPrefix(zone, "25") check, which misread a GENUINE numeric zone
// "25" ("fe80::1%25", meaning interface index 25 — not an encoded
// delimiter) as already-encoded and left it unescaped. The result,
// "http://[fe80::1%25]:8100", then failed url.Parse outright: the lone
// "%25" gets decoded back to a bare '%' with nothing after it —
// "invalid host: ParseAddr(\"fe80::1%\"): zone must be a non-empty string".
//
// The RED baseline is re-derivable on demand rather than cited to a commit
// (§11.4.6 — this work is uncommitted, so a commit citation would name an
// artefact that does not exist): delete the `&& strings.TrimPrefix(zone,
// "25") != ""` clause from urlEncodeZone's alreadyEncoded test and re-run —
// this test FAILS with exactly the url.Parse error quoted above. That
// deletion IS the paired §1.1 mutation for this guard.
func TestUrlEncodeZone_GenuineNumericZone25_NotMisreadAsEncoded(t *testing.T) {
	got := BaseURL("http", "fe80::1%25", 8100)

	u, err := url.Parse(got)
	require.NoError(t, err, "a genuine numeric zone \"25\" must round-trip through a URL; got %q", got)
	require.Equal(t, "fe80::1%25", u.Hostname(),
		"the zone must decode back to the exact original raw zone value \"25\"")
	require.Equal(t, "8100", u.Port())
}

// TestDialAddressString_MatchesDialAddress_WhitespacePadded is the
// permanent regression guard for MUST-FIX 2 (2026-08-12 review).
// DialAddressString omitted the TrimSpace step DialAddress carries (via
// helixendpoint.unbracket), so the two entry points of this package
// disagreed about the SAME address for whitespace-padded input —
// DialAddress("  2001:db8::1  ", 6379) = "[2001:db8::1]:6379" while
// DialAddressString("  2001:db8::1  ", "6379") =
// "[  2001:db8::1  ]:6379" (a malformed, undialable authority). Reachable
// in production: internal/cache's RedisConfig.Host is never trimmed before
// reaching DialAddressString.
func TestDialAddressString_MatchesDialAddress_WhitespacePadded(t *testing.T) {
	const padded = "  2001:db8::1  "

	want := DialAddress(padded, 6379)
	got := DialAddressString(padded, "6379")
	require.Equal(t, want, got,
		"DialAddress and DialAddressString must agree on the SAME (padded) address — "+
			"two entry points of one package silently disagreeing is exactly the defect this package exists to prevent")
	require.Equal(t, "[2001:db8::1]:6379", got)
}

// TestDialAddressString_TrimsPaddedBracketedHost covers the sibling
// whitespace-plus-brackets shape MUST-FIX 2 also measured to disagree on:
// " [2001:db8::1] " must still unbracket correctly, not leave the padding
// trapped inside the brackets ("[ [2001:db8::1] ]:6379").
func TestDialAddressString_TrimsPaddedBracketedHost(t *testing.T) {
	got := DialAddressString(" [2001:db8::1] ", "6379")
	require.Equal(t, "[2001:db8::1]:6379", got)
}

// TestBaseURL_GenDelimHostRefusesRatherThanRedirects is the permanent guard
// for BLOCKING 1 of the 2026-08-12 review: BaseURL applied NO host validation
// while its package doc promised to "pass a malformed host through un-repaired
// and let the dial/connect fail loudly … never invent a destination".
//
// For three of the four RFC 3986 gen-delims the promise merely bent (the port
// was silently dropped); for "@" it broke outright — the URL parsed cleanly
// and named a DIFFERENT HOST. Measured RED baseline, captured against the
// pre-fix revision in the session that fixed it:
//
//	"hx@yz"      -> "http://hx@yz:7061"      hostname "yz"      port "7061"  <- WRONG HOST, silent
//	"hx/yz"      -> "http://hx/yz:7061"      hostname "hx"      port ""      <- port dropped
//	"hx?yz"      -> "http://hx?yz:7061"      hostname "hx"      port ""      <- port dropped
//	"hx#yz"      -> "http://hx#yz:7061"      hostname "hx"      port ""      <- port dropped
//	"evil.com/x" -> "http://evil.com/x:7061" hostname "evil.com" port ""     <- port dropped
//
// All five are reachable from operator config (config.QdrantHost,
// config.ChromaHost, the Flink JobManagerHost).
//
// The fix percent-encodes the four bytes so net/url refuses the result rather
// than re-reading it. The assertions below are deliberately BEHAVIOURAL — the
// URL must not parse — rather than string-equality on the escape, so the guard
// survives a change of encoding strategy as long as the SAFETY property holds.
//
// Paired §1.1 mutation: delete the genDelimEscaper.Replace call from
// encodeGenDelims (return host unchanged) and re-run — every sub-test below
// FAILS, and the "@" sub-test fails by silently resolving to host "yz".
func TestBaseURL_GenDelimHostRefusesRatherThanRedirects(t *testing.T) {
	cases := []struct {
		name        string
		host        string
		redBaseline string
	}{
		{"userinfo_at_redirects_to_wrong_host", "hx@yz", `parsed as host "yz" — a DIFFERENT machine`},
		{"path_slash_drops_port", "hx/yz", `parsed as host "hx" with an EMPTY port`},
		{"query_question_drops_port", "hx?yz", `parsed as host "hx" with an EMPTY port`},
		{"fragment_hash_drops_port", "hx#yz", `parsed as host "hx" with an EMPTY port`},
		{"realistic_operator_typo_drops_port", "evil.com/x", `parsed as host "evil.com" with an EMPTY port`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BaseURL("http", tc.host, 7061)

			u, err := url.Parse(got)
			require.Error(t, err,
				"BaseURL(%q) returned %q, which url.Parse ACCEPTED (hostname %q) — "+
					"a host carrying a URL gen-delimiter must never yield a usable URL; RED baseline: %s",
				tc.host, got, hostnameOrEmpty(u), tc.redBaseline)

			// Nothing invented, nothing dropped: every byte the caller wrote is
			// still present (as its escape) and so is the port. This is what
			// distinguishes "refuse" from "substitute a placeholder".
			require.Contains(t, got, ":7061",
				"the caller's port must survive so the failure is visibly about the HOST")
			require.NotContains(t, got, "localhost",
				"a malformed host must NEVER be silently swapped for a placeholder host")
		})
	}
}

// hostnameOrEmpty renders a possibly-nil *url.URL for a failure message.
func hostnameOrEmpty(u *url.URL) string {
	if u == nil {
		return "<unparsed>"
	}
	return u.Hostname()
}

// TestGenDelimEncoding_LeavesEveryLegitimateHostByteIdentical is the other
// half of the BLOCKING-1 guard: the neutralisation must be invisible to every
// host an operator can legitimately configure. A fix that made malformed input
// loud at the cost of mangling valid input would be a worse defect than the
// one it closed.
func TestGenDelimEncoding_LeavesEveryLegitimateHostByteIdentical(t *testing.T) {
	cases := []struct {
		name string
		host string
		want string
	}{
		{"hostname", "qdrant.internal.example", "http://qdrant.internal.example:6333"},
		{"ipv4", "127.0.0.1", "http://127.0.0.1:6333"},
		{"bare_ipv6", "::1", "http://[::1]:6333"},
		{"bracketed_ipv6", "[::1]", "http://[::1]:6333"},
		{"full_ipv6", "2001:db8::1", "http://[2001:db8::1]:6333"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "http://[fe80::1%25eth0]:6333"},
		{"already_encoded_zone", "fe80::1%25eth0", "http://[fe80::1%25eth0]:6333"},
		{"empty_host", "", "http://:6333"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, BaseURL("http", tc.host, 6333),
				"gen-delimiter neutralisation must not alter a legitimate host")
		})
	}
}

// TestBaseURLString_GenDelimHostRefusesRatherThanRedirects proves the
// string-port sibling carries the SAME guard. Both URL-composing entry points
// share urlSafeHost precisely so one cannot be hardened while the other stays
// exploitable (§11.4.74).
func TestBaseURLString_GenDelimHostRefusesRatherThanRedirects(t *testing.T) {
	for _, host := range []string{"hx@yz", "hx/yz", "hx?yz", "hx#yz"} {
		t.Run(host, func(t *testing.T) {
			got := BaseURLString("http", host, "18434")
			_, err := url.Parse(got)
			require.Error(t, err,
				"BaseURLString(%q) returned %q, which url.Parse ACCEPTED", host, got)
		})
	}
}

// TestDialAddressForURL_GenDelimHostRefusesRatherThanRedirects covers the
// credentialed-URI path (AMQP: scheme + userinfo + THIS authority + vhost).
// A host carrying "@" is the worst case there — an unencoded one would let the
// host itself terminate the userinfo the caller spliced in front of it.
func TestDialAddressForURL_GenDelimHostRefusesRatherThanRedirects(t *testing.T) {
	authority := DialAddressForURL("hx@yz", 5672)
	got := "amqp://user:pass@" + authority + "/vhost"

	_, err := url.Parse(got)
	require.Error(t, err,
		"a gen-delimiter in the host must not survive into a credentialed URI; got %q", got)
}

// TestDialAddress_GenDelimHostIsPassedThroughRaw pins the DELIBERATE asymmetry:
// the dial path must NOT be encoded. net.Dial treats the host as an opaque
// label, so an unrepaired malformed host genuinely does fail loudly there,
// whereas an injected "%40" would corrupt a hostname that a resolver might
// otherwise handle. Encoding here would be a defect, not a hardening.
func TestDialAddress_GenDelimHostIsPassedThroughRaw(t *testing.T) {
	require.Equal(t, "hx@yz:6379", DialAddress("hx@yz", 6379),
		"DialAddress feeds net.Dial and must pass the host through un-encoded")
	require.Equal(t, "hx@yz:6379", DialAddressString("hx@yz", "6379"),
		"DialAddressString feeds net.Dial and must pass the host through un-encoded")
}

// TestUrlEncodeZone_LastPercentConvention_RefusesRatherThanInvents kills the
// mutant that MUST-FIX 1 of the 2026-08-12 review found unasserted: flipping
// urlEncodeZone's strings.LastIndex to strings.Index survived the whole suite,
// the only survivor of a 9-mutation sweep.
//
// The earlier justification for that survival — that the two conventions
// disagree only on inputs "unparseable under EITHER convention" — is
// measurably FALSE. On "fe80::1%eth0%25" they differ, and they differ in the
// direction that matters:
//
//	LAST-'%'  -> "http://[fe80::1%eth0%2525]:8100"  url.Parse FAILS ("%et")
//	FIRST-'%' -> "http://[fe80::1%25eth0%25]:8100"  url.Parse OK, hostname "fe80::1%eth0%"
//
// FIRST-'%' INVENTS a host the caller never wrote, by re-reading the caller's
// own literal "%25" as this function's delimiter. LAST-'%' refuses. Refusing
// is this package's contract for an input it cannot faithfully express.
//
// Paired §1.1 mutation: change strings.LastIndex to strings.Index in
// urlEncodeZone and re-run — this test FAILS.
func TestUrlEncodeZone_LastPercentConvention_RefusesRatherThanInvents(t *testing.T) {
	const host = "fe80::1%eth0%25"

	got := BaseURL("http", host, 8100)

	u, err := url.Parse(got)
	require.Error(t, err,
		"BaseURL(%q) returned %q which url.Parse ACCEPTED as hostname %q — "+
			"splitting the zone at the FIRST '%%' fabricates a host the caller never wrote; "+
			"the LAST-'%%' convention must refuse instead",
		host, got, hostnameOrEmpty(u))

	require.NotEqual(t, "http://[fe80::1%25eth0%25]:8100", got,
		"this is the FIRST-'%' output — the mutation is live")
}

// TestUrlEncodeZone_NumericZoneBeginning25_TruncatesAndAgreesWithHelixEndpoint
// pins MUST-FIX 2 of the 2026-08-12 review, which found the package doc
// understating a real defect: a raw numeric zone whose digits begin with "25"
// does not merely fail to be expressible, it silently addresses a DIFFERENT
// INTERFACE.
//
// This test asserts BOTH halves of the decision to leave it as-is:
//
//  1. the truncation is REAL and is recorded honestly (zone "250" -> "0"), and
//  2. helixendpoint produces BYTE-IDENTICAL output, which is what makes
//     leaving it safe.
//
// Half 2 is the load-bearing one. Fixing this unilaterally here would CREATE a
// divergence between the two address builders in this family — the absorption
// class this package exists to prevent. A future contributor who "fixes" the
// truncation in netaddr alone breaks this test, which is the intended alarm:
// the change belongs upstream in helixendpoint.normalizeHost, landing in both.
func TestUrlEncodeZone_NumericZoneBeginning25_TruncatesAndAgreesWithHelixEndpoint(t *testing.T) {
	cases := []struct {
		host         string
		wantURL      string
		wantHostname string
		note         string
	}{
		{"fe80::1%250", "http://[fe80::1%250]:7061", "fe80::1%0", `zone "250" truncates to "0"`},
		{"fe80::1%251", "http://[fe80::1%251]:7061", "fe80::1%1", `zone "251" truncates to "1"`},
		{"fe80::1%255", "http://[fe80::1%255]:7061", "fe80::1%5", `zone "255" truncates to "5"`},
	}

	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := BaseURL("http", tc.host, 7061)
			require.Equal(t, tc.wantURL, got,
				"BaseURL must pass a zone beginning \"25\" through UNCHANGED "+
					"(it is read as an already-encoded delimiter); a difference here "+
					"means the encode rule moved and the truncation this test pins "+
					"has changed shape — re-read the urlEncodeZone contract before "+
					"updating the expectation")

			u, err := url.Parse(got)
			require.NoError(t, err,
				"the composed address must still be a PARSEABLE URL — the "+
					"truncation is a wrong-interface hazard, not a malformed-output one")
			require.Equal(t, tc.wantHostname, u.Hostname(),
				"HONEST BOUNDARY, not a passing feature: %s — the composed address "+
					"names a different interface than the caller configured", tc.note)

			// The no-divergence guard: netaddr and helixendpoint MUST agree.
			require.Equal(t, helixendpoint.BaseURL(tc.host, 7061), got,
				"netaddr and helixendpoint must not disagree about %q — a unilateral "+
					"fix here creates exactly the two-builders-disagree defect this "+
					"package exists to prevent; fix it upstream in normalizeHost instead", tc.host)
		})
	}
}

// TestDialAddressString_EmptyPort covers NIT 3 of the 2026-08-12 review.
// internal/config's RedisConfig.Port is a string with no default, so an
// unset value reaches DialAddressString as "" and composes "[::1]:".
//
// Pinned as the CORRECT behaviour, not fixed: the empty port is the caller's
// own unset config, and inventing a default port here is precisely the
// substitution this package refuses. "[::1]:" is a splittable authority whose
// port is visibly empty, so net.Dial fails against the actual bad input.
func TestDialAddressString_EmptyPort(t *testing.T) {
	require.Equal(t, "[::1]:", DialAddressString("::1", ""),
		"an empty port must pass through, never be silently defaulted")
	require.Equal(t, "redis.internal:", DialAddressString("redis.internal", ""))

	// Splittable, with the emptiness visible rather than swallowed.
	host, port, err := net.SplitHostPort(DialAddressString("::1", ""))
	require.NoError(t, err)
	require.Equal(t, "::1", host)
	require.Empty(t, port)
}

// TestStripBracket_EmptyBracketPair covers NIT 4 of the 2026-08-12 review:
// stripBracket("[]") yields "", so a host of literally "[]" is reduced to the
// empty host. Pinned as agreed-with-upstream behaviour rather than fixed —
// helixendpoint.unbracket does the identical thing, and "[]" is not a host any
// operator can mean, so the empty result fails loudly at dial time exactly as
// an empty host should.
func TestStripBracket_EmptyBracketPair(t *testing.T) {
	require.Equal(t, "", stripBracket("[]"))
	require.Equal(t, "", stripBracket("  []  "))

	// The re-export contract on this input — NOT a stripBracket-vs-unbracket
	// drift guard, which this assertion cannot be: DialAddress's whole body is
	// `return helixendpoint.DialAddress(host, port)` and never calls
	// stripBracket, so both sides are the same call and this is f(x) == f(x)
	// while that delegation holds. The drift guard for this input is the
	// stripBracket("  []  ") assertion above — measured 2026-08-13, that is the
	// assertion that dies when stripBracket's TrimSpace step is dropped; this
	// one never fires.
	require.Equal(t, helixendpoint.DialAddress("[]", 6333), DialAddress("[]", 6333),
		"netaddr.DialAddress must remain the verbatim re-export of helixendpoint.DialAddress on the empty-bracket pair")
}

// TestDialAddress_MatchesHelixEndpointDialAddress asserts the re-export
// contract stated in the package doc: this function IS
// helixendpoint.DialAddress under a local name, not a parallel
// reimplementation that could silently drift out of agreement with it
// (§11.4.74 — one function, two names, never two definitions).
func TestDialAddress_MatchesHelixEndpointDialAddress(t *testing.T) {
	inputs := []struct {
		host string
		port int
	}{
		{"[::1]", 6333},
		{"::1", 6333},
		{"fe80::1%eth0", 22},
		{"127.0.0.1", 80},
		{"example.com", 443},
		{"", 1},
	}
	for _, in := range inputs {
		got := DialAddress(in.host, in.port)
		want := helixEndpointDialAddressForTest(in.host, in.port)
		require.Equal(t, want, got,
			"netaddr.DialAddress(%q, %d) must match helixendpoint.DialAddress verbatim", in.host, in.port)
	}
}

// TestStripBracket_UnpairedBracketIsNotStripped pins the bracket-PAIRING
// condition in stripBracket — `len(host) > 1 && host[0] == '[' &&
// host[len(host)-1] == ']'` — which was COMPLETELY UNASSERTED until this test
// existed: deleting EITHER conjunct is caught by this test and by nothing else
// in the package (re-measured 2026-08-13; see the mutation table below).
//
// What each deletion does to an unterminated bracket — an ordinary config typo
// — MEASURED by probing DialAddressString + url.Parse under each mutation:
//
//	deleted conjunct    input          pristine             mutant
//	`== ']'`            "[::1"         "[[::1]:6379"        "[::]:6379"      PARSES — the WILDCARD address
//	`== ']'`            "[localhost"   "[localhost:6379"    "localhos:6379"  PARSES — host truncated
//	`host[0] == '['`    "host]"        "host]:6379"         "ost:6379"       PARSES — a DIFFERENT host
//	`host[0] == '['`    "a]"           "a]:6379"            ":6379"          PARSES — host lost entirely
//	`host[0] == '['`    "::1]"         "[::1]]:6379"        "[:1]:6379"      (both refuse; the STRING differs)
//
// Every mutant invents a destination: rows 1-2 turn a loud refusal into a
// silently dialable WRONG machine, rows 3-4 rewrite a host pristine preserves
// verbatim (§11.4.6). Row 5 is why the test asserts the EXACT composed string —
// both sides refuse to parse there, so a refusal-only assertion would miss it.
//
// ENTRY POINTS. Plain DialAddress is NOT one of them — it re-exports
// helixendpoint.DialAddress, which unbrackets with its OWN copy of this
// primitive, so mutating stripBracket is invisible there (measured: under all
// four stripBracket mutations below, TestDialAddress_MatchesHelixEndpointDialAddress
// PASSES). The paths that DO reach it: DialAddressString directly, and BaseURL /
// BaseURLString / DialAddressForURL via urlEncodeZone -> stripBracket.
//
// WHAT KILLER 3 OWNS — every cell MEASURED, never reasoned. Method: one
// mutation at a time in a mirror copy, substitution count asserted, mutant
// sha256 confirmed different from pristine, `go test ./internal/netaddr/
// -count=1 -v` run ONCE, then restored and sha-verified. FIRED is read by
// content (`grep -c 'must not drift from DialAddress' <log>`, a string nothing
// else here emits); RAN is read from the per-row subtest verdicts, since a log
// names only what FAILED and is silent about an assertion that ran and passed.
//
//	mutation                            ran     fired  whole package
//	----------------------------------  ------  -----  --------------------------------
//	stripBracket: drop `== ']'`         6 of 8  no     1 FAIL / 22 PASS (this test only)
//	stripBracket: drop `host[0]=='['`   5 of 8  no     1 FAIL / 22 PASS (this test only)
//	stripBracket: `if false && <cond>`  7 of 8  no     6 FAIL / 17 PASS
//	stripBracket: TrimSpace -> Clone    8 of 8  no     3 FAIL / 20 PASS
//	DialAddress: stop unbracketing      8 of 8  YES    5 FAIL / 18 PASS
//
// Killer 3 RUNS in most rows under every stripBracket mutation and catches none
// — not blindness: `require.*` aborts a row at its first failure, and the direct
// stripBracket assertion fails in exactly the rows where the mutant DIVERGES, so
// Killer 3 only ever sees rows where mutant and pristine agree. TrimSpace ->
// Clone reaches all 8 and diverges in none: no host here is whitespace-padded.
//
// The last row is what Killer 3 exists for — upstream re-export drift — where it
// fires on control_well_formed_pair_is_stripped as the SOLE dying assertion in
// that subtest (the direct stripBracket assertion, Killer 1 and BOTH of Killer
// 2's calls all PASS there). Killer 2 is no substitute: through urlSafeHost ->
// urlEncodeZone -> stripBracket it calls DialAddress("::1", ...) on a bracketed
// row, while Killer 3's left operand is DialAddress("[::1]", ...).
//
// DECISION (2026-08-13): keep all five, `require.*` included — `assert.*` was
// rejected because the other four are exact and deliberately abort-on-first, and
// the "ran" column measures that masking rather than removing it. WITHIN THIS
// TEST each is the sole guard of a branch the others cannot see: unpaired-bracket
// by the direct stripBracket assertion, TrimSpace by the padded rows of
// TestDialAddressString_MatchesDialAddress_WhitespacePadded,
// TestDialAddressString_TrimsPaddedBracketedHost and
// TestStripBracket_EmptyBracketPair, upstream-drift by Killer 3. PACKAGE-WIDE it
// is NOT Killer 3's alone — that mutation also kills the first and third of
// those tests, TestDialAddress_AddressFormEnumeration and
// TestDialAddress_MatchesHelixEndpointDialAddress (the 5 FAIL above).
//
// AUTHORING RULE: never claim an assertion inside a `require.*` block did or did
// not run from a failure log — that silence conflates "never reached" with
// "reached and passed". Measure that it EXECUTED, then whether it fired. Cited
// by symbol throughout, never by line — line numbers move with every edit above.
//
// CROSS-REPO (§11.4.74): the sibling llms_verifier clientip.stripBrackets
// carries the identical pairing conjunct — a known-fragile shape worth pinning
// on sight rather than auditing later.
func TestStripBracket_UnpairedBracketIsNotStripped(t *testing.T) {
	cases := []struct {
		name string
		host string
		// wantStrip is stripBracket's own output.
		wantStrip string
		// wantAddr is DialAddressString(host, "6379").
		wantAddr string
		// wantURL is BaseURL("http", host, 6379).
		wantURL string
		// urlHost is the hostname url.Parse recovers from wantURL, or "" when
		// wantURL refuses to parse (urlRefuses).
		urlRefuses bool
		urlHost    string
		explain    string
	}{
		{
			name:       "open_bracket_ipv6_no_close",
			host:       "[::1",
			wantStrip:  "[::1",
			wantAddr:   "[[::1]:6379",
			wantURL:    "http://[[::1]:6379",
			urlRefuses: true,
			explain:    "M15 (drop `== ']'`) turns this into the dialable WILDCARD [::]:6379",
		},
		{
			name:       "open_bracket_hostname_no_close",
			host:       "[localhost",
			wantStrip:  "[localhost",
			wantAddr:   "[localhost:6379",
			wantURL:    "http://[localhost:6379",
			urlRefuses: true,
			explain:    "M15 truncates the last byte and yields the dialable localhos:6379",
		},
		{
			name:       "close_bracket_ipv6_no_open",
			host:       "::1]",
			wantStrip:  "::1]",
			wantAddr:   "[::1]]:6379",
			wantURL:    "http://[::1]]:6379",
			urlRefuses: true,
			explain:    "M16 (drop `host[0]=='['`) yields [:1]:6379 — also unparseable, so only the exact string kills it",
		},
		{
			name:      "close_bracket_hostname_no_open",
			host:      "host]",
			wantStrip: "host]",
			wantAddr:  "host]:6379",
			wantURL:   "http://host]:6379",
			urlHost:   "host]",
			explain:   "M16 yields ost:6379 — parses cleanly as a DIFFERENT host, the dangerous row",
		},
		{
			name:      "close_bracket_single_char_host",
			host:      "a]",
			wantStrip: "a]",
			wantAddr:  "a]:6379",
			wantURL:   "http://a]:6379",
			urlHost:   "a]",
			explain:   "M16 yields :6379 — the host is lost entirely",
		},
		// Controls: a WELL-FORMED pair MUST still be stripped, so this test
		// cannot be satisfied by a mutation that simply never strips.
		{
			name:      "control_well_formed_pair_is_stripped",
			host:      "[::1]",
			wantStrip: "::1",
			wantAddr:  "[::1]:6379",
			wantURL:   "http://[::1]:6379",
			urlHost:   "::1",
			explain:   "the pairing condition holds, so exactly one layer comes off",
		},
		{
			name:      "control_unbracketed_ipv6_is_bracketed_downstream",
			host:      "::1",
			wantStrip: "::1",
			wantAddr:  "[::1]:6379",
			wantURL:   "http://[::1]:6379",
			urlHost:   "::1",
			explain:   "nothing to strip; JoinHostPort adds the brackets",
		},
		{
			name:      "control_plain_hostname_untouched",
			host:      "redis.internal",
			wantStrip: "redis.internal",
			wantAddr:  "redis.internal:6379",
			wantURL:   "http://redis.internal:6379",
			urlHost:   "redis.internal",
			explain:   "neither conjunct fires",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantStrip, stripBracket(tc.host),
				"stripBracket(%q): %s", tc.host, tc.explain)

			// Killer 1 — the direct dial-address path.
			require.Equal(t, tc.wantAddr, DialAddressString(tc.host, "6379"),
				"DialAddressString(%q, \"6379\"): %s", tc.host, tc.explain)

			// Killer 2 — the URL-composing path (urlEncodeZone -> stripBracket).
			require.Equal(t, tc.wantURL, BaseURL("http", tc.host, 6379),
				"BaseURL(\"http\", %q, 6379): %s", tc.host, tc.explain)
			require.Equal(t, tc.wantURL, BaseURLString("http", tc.host, "6379"),
				"BaseURLString must agree with BaseURL for %q", tc.host)

			// Killer 3 — cross-builder agreement. A mutated stripBracket makes
			// this package's string-port builder disagree with the verbatim
			// re-export of helixendpoint's int-port builder for one input.
			require.Equal(t, DialAddress(tc.host, 6379), DialAddressString(tc.host, "6379"),
				"DialAddressString(%q) must not drift from DialAddress(%q)", tc.host, tc.host)

			// The user-visible consequence: refuse, or preserve the host.
			u, err := url.Parse(tc.wantURL)
			if tc.urlRefuses {
				require.Error(t, err,
					"%q must REFUSE to parse — a mutant that makes it parse invents a destination", tc.wantURL)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.urlHost, u.Hostname(),
				"the operator's host must survive verbatim, never be truncated to a different machine")
		})
	}
}

// TestUrlEncodeZone_StripBracketPrecedesZoneScan pins the ORDER inside
// urlEncodeZone — stripBracket runs BEFORE the '%' scan — which round 5's
// mutation M18 found unasserted (deleting the stripBracket call changed 0 of
// its 21 corpus inputs through the full BaseURL pipeline).
//
// The corpus did not contain the BRACKETED spelling of the divergence input
// the package doc's zone section already discusses. Measured 2026-08-13:
//
//	host "[fe80::1%eth0%25]"
//	  with stripBracket  -> "http://[fe80::1%eth0%2525]:8100"
//	  without            -> "http://[fe80::1%eth0%25]:8100"
//
// The trailing ']' is otherwise swept into the zone, which flips the
// already-encoded branch: TrimPrefix("25]", "25") leaves "]" (non-empty, so
// "already encoded"), while TrimPrefix("25", "25") leaves "" (so "raw, encode
// it"). Two different branches for one logical host.
//
// THE FAMILY, MEASURED (§11.4.118 — enumerated, not "we looked and it was
// fine"). The pre-strip's ENTIRE behavioural reach was measured 2026-08-13
// over a 41-input corpus through this pipeline: exactly THREE inputs diverge,
// all one family — a BRACKETED host whose zone ends "25]". The row above is
// the weakest member of that family, because its two spellings both refuse.
// The other two are NOT string-only, and are pinned below:
//
//	host "[fe80::1%25]"
//	  with stripBracket  -> "http://[fe80::1%2525]:8100"  PARSES, host "fe80::1%25"
//	  without            -> "http://[fe80::1%25]:8100"    REFUSES: zone must be
//	                                                      a non-empty string
//
//	host "[%25]"
//	  with stripBracket  -> "http://%2525:8100"           PARSES, host "%25"
//	  without            -> "http://%25:8100"             PARSES, host "%"
//
// So dropping the pre-strip is not merely "a visible decision": on
// "[fe80::1%25]" it flips PARSE into REFUSE, and on "[%25]" it silently
// resolves a DIFFERENT DESTINATION ("%" instead of "%25") — the
// invent-a-destination failure this whole package exists to refuse, arrived at
// by a URL that still parses cleanly. That is a user-visible behavioural
// difference, and it is asserted below as parse-outcome + url.Hostname(), not
// as a composed string.
//
// HONEST BOUNDARY (§11.4.6) — what this test does NOT claim. The FIRST row
// ("[fe80::1%eth0%25]") remains string-only: both its spellings REFUSE, with
// the same `invalid URL escape "%et"`, and that is asserted explicitly below
// so the row cannot be read as claiming the strip averts a parse failure it
// does not avert. The behavioural claim rests on the other two rows. The
// three-input divergence set is a MEASURED result over the corpus named
// above, not a proof that no 42nd input diverges (§11.4.6).
func TestUrlEncodeZone_StripBracketPrecedesZoneScan(t *testing.T) {
	// The discriminator, at the function's own boundary.
	// Each divergent row is its own subtest so that a mutation report names
	// WHICH guarantee broke. require aborts at the first failure, so a single
	// flat body would let the string-only row mask the two behavioural ones and
	// leave "does the behavioural claim actually kill the mutant?" unproven.

	t.Run("string_only_row_bracketed_named_zone", func(t *testing.T) {
		require.Equal(t, "fe80::1%eth0%2525", urlEncodeZone("[fe80::1%eth0%25]"),
			"stripBracket must run BEFORE the '%%' scan, or the ']' joins the zone "+
				"and flips the already-encoded branch")

		// The bracketed and unbracketed spellings of one logical host must agree.
		require.Equal(t, urlEncodeZone("fe80::1%eth0%25"), urlEncodeZone("[fe80::1%eth0%25]"),
			"bracketing must not change which zone-encoding branch is taken")

		// End to end, through the pipeline the callers actually use.
		require.Equal(t, "http://[fe80::1%eth0%2525]:8100", BaseURL("http", "[fe80::1%eth0%25]", 8100))
		require.Equal(t, BaseURL("http", "fe80::1%eth0%25", 8100), BaseURL("http", "[fe80::1%eth0%25]", 8100))

		// Both spellings refuse — the honest outcome for an input this package
		// cannot faithfully express. Asserted so the row cannot be read as
		// claiming the strip AVERTS a parse-level failure; for THIS row it does not.
		for _, host := range []string{"fe80::1%eth0%25", "[fe80::1%eth0%25]"} {
			_, err := url.Parse(BaseURL("http", host, 8100))
			require.Error(t, err,
				"BaseURL(%q) must refuse to parse rather than invent a destination", host)
		}
	})

	// BEHAVIOURAL ROW 1 — "[fe80::1%25]". Dropping the pre-strip flips this
	// from PARSES to REFUSES: the ']' joins the zone, the already-encoded
	// branch returns the input untouched, and net/url is then handed a literal
	// "%25]" whose zone is empty. Asserted as the parse OUTCOME and the
	// resolved host, so the pre-strip's absence cannot pass as a string nit.
	t.Run("behavioural_parse_refuse_flip", func(t *testing.T) {
		u, err := url.Parse(BaseURL("http", "[fe80::1%25]", 8100))
		require.NoError(t, err,
			"BaseURL(\"[fe80::1%%25]\") must PARSE — without the pre-strip net/url "+
				"rejects it with \"zone must be a non-empty string\"")
		require.Equal(t, "fe80::1%25", u.Hostname(),
			"the parsed host must be the zone-qualified literal the caller named")

		require.Equal(t, "http://[fe80::1%2525]:8100", BaseURL("http", "[fe80::1%25]", 8100))
		require.Equal(t, BaseURL("http", "fe80::1%25", 8100), BaseURL("http", "[fe80::1%25]", 8100),
			"bracketing must not change the composed authority")
	})

	// BEHAVIOURAL ROW 2 — "[%25]". The severe one: BOTH spellings parse, so
	// there is no loud failure to notice, and dropping the pre-strip silently
	// resolves host "%" where the caller named "%25". A cleanly-parsing URL
	// pointing at a DIFFERENT destination is precisely the substitution this
	// package refuses (see the package doc's default-substitution section).
	t.Run("behavioural_different_destination", func(t *testing.T) {
		u, err := url.Parse(BaseURL("http", "[%25]", 8100))
		require.NoError(t, err, "BaseURL(\"[%%25]\") must parse")
		require.Equal(t, "%25", u.Hostname(),
			"the parsed host must be %%25 as named — without the pre-strip it "+
				"silently resolves %% instead, a DIFFERENT destination reached "+
				"through a cleanly-parsing URL")

		require.Equal(t, "http://%2525:8100", BaseURL("http", "[%25]", 8100))
		require.Equal(t, BaseURL("http", "%25", 8100), BaseURL("http", "[%25]", 8100),
			"bracketing must not change the composed authority")
	})

	// Control: the ordinary bracketed zone-qualified host, where the strip is
	// genuinely redundant — pinned so the test is not mistaken for a claim
	// that bracketing always matters here. This subtest must SURVIVE the
	// mutation; only the three divergent rows above may die.
	t.Run("control_strip_is_redundant_here", func(t *testing.T) {
		require.Equal(t, "http://[fe80::1%25eth0]:8100", BaseURL("http", "[fe80::1%eth0]", 8100))
		require.Equal(t, BaseURL("http", "fe80::1%eth0", 8100), BaseURL("http", "[fe80::1%eth0]", 8100))
	})
}
