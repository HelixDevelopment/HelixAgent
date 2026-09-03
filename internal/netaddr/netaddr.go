// Package netaddr composes "host:port" and "scheme://host:port" network
// addresses for services HelixAgent does NOT own — third-party databases,
// message brokers, stream processors, mail relays, and any other foreign
// endpoint a HelixAgent config field names.
//
// # HXC-286: the family this package closes
//
// llms_verifier's own `pkg/helixendpoint` package (HXC-268) fixed the exact
// same defect for the endpoint IT resolves: naive `fmt.Sprintf("%s:%d", host,
// port)` composition breaks the moment host is an UNBRACKETED IPv6 literal —
// "2001:db8::1" + 7061 becomes "2001:db8::1:7061", which no reader can split
// into (host, port). Re-measured verbatim against this repository's Go
// toolchain (2026-08-13); an earlier revision of this paragraph misquoted the
// first error by dropping its LEADING COLON, so the strings below are copied
// from the run rather than paraphrased:
//
//	net/url.Parse("http://2001:db8::1:7061")
//	  -> invalid port ":db8::1:7061" after host
//	net.SplitHostPort("2001:db8::1:7061")
//	  -> address 2001:db8::1:7061: too many colons in address
//
// The url.Parse error is quoted for the composed URL, because that is where
// this defect actually lands (every BaseURL-class caller). Handing url.Parse
// the bare authority instead produces a THIRD, unrelated message ("first path
// segment in URL cannot contain colon") — scheme-less input is parsed as a
// path, so it is not evidence about port-splitting and is not cited here. An ALREADY-bracketed host composes correctly under
// naive concatenation ("[::1]" + ":6333" = "[::1]:6333", syntactically
// valid) — bracketing is not itself the failure mode for this family of
// sites. The double-bracket shape ("[[::1]]:6333") some earlier framings of
// this defect cited is a DIFFERENT mechanism entirely: it is what
// net.JoinHostPort produces when handed an already-bracketed host without
// first stripping the brackets, which is exactly why helixendpoint.go's
// unbracket() helper exists — none of THIS package's own sites reach that
// path (see "Why this is a NEW package" below). A read-only survey
// (docs/research/address_composition_family_20260812/ANALYSIS.md in the
// meta-repo) found ~30 more sites of the identical unbracketed-concatenation
// pattern in helix_agent — the CONSUMER of llms_verifier's endpoint, not
// covered by helixendpoint's own census because that census was scoped to
// "both modules of this repository" (llms_verifier only).
//
// (F7 of the round-5 review: an earlier revision quoted that census as "31
// sites". The number is DROPPED rather than repeated, because the very
// document it cites contests it — ANALYSIS.md:103 reads "**15 / 24 / 30 / 31
// do not agree.** Reconcile with a fresh recount before HXC-268 closes."
// Quoting one of four disagreeing totals as settled would contradict the
// "NO TOTAL IS ASSERTED HERE" paragraph below in the same document. The
// load-bearing claim — that the census was SCOPED to llms_verifier and so
// never looked at this consumer — is unaffected by which total is right.)
//
// # Why this is a NEW package rather than an import of helixendpoint.BaseURL
//
// llms_verifier already exports the exact no-default dial-address builder
// this package needs — DialAddress(host, port) — and this package reuses it
// verbatim (§11.4.74: extend, don't duplicate) rather than reimplementing the
// bracket-strip-then-JoinHostPort algorithm a third time. What llms_verifier
// does NOT export is a URL-composing sibling without default-substitution:
// helixendpoint.BaseURL substitutes DefaultHost ("localhost") when its input
// is unusable, and that default is meant SPECIFICALLY for HelixAgent's own
// endpoint (the package's first, primary responsibility — see its doc
// comment). Using it for a third-party service would be worse than the bug
// it fixes: a misconfigured address for someone else's database would
// silently become a *different, wrong* machine on the *right* port, because
// DialAddress/BaseURL fall back on host and port independently and the
// fallback host has nothing to do with the service actually being addressed.
//
// # The default-substitution decision (deliberate, argued)
//
// Two builders exist in this family now, chosen by OWNERSHIP of the address:
//
//   - digital.vasic.llmsverifier/pkg/helixendpoint.BaseURL(host, port) — WITH
//     default-substitution to HelixAgent's OWN placeholder. Correct ONLY for
//     composing HelixAgent's own endpoint (the address a CLI-agent config
//     must point at to reach THIS server). Used at cmd/helixagent/main.go's
//     three sites, which sit alongside cliagents.DefaultHelixAgentExtensions
//     and cliagents.DefaultFormattersConfig — both of which already resolve
//     the SAME host/port pair through helixendpoint.BaseURL internally. Using
//     a different, non-substituting builder for the sibling fields on that
//     same line would create exactly the defect class this whole family is
//     about: two builders privately disagreeing about the SAME logical
//     endpoint (the "absorption class" — one guard covering several sites
//     stops discriminating between them, viewed from the other side).
//
//   - netaddr.DialAddress / netaddr.BaseURL (this package) — NO
//     default-substitution. Used for EVERYTHING ELSE: every third-party
//     service (Qdrant, Chroma, Redis, Flink, the AMQP broker), every
//     ownership-agnostic generic helper (internal/ports), and every service
//     that is part of the Helix family but is NOT HelixAgent itself
//     (HelixLLM is a distinct submodule/service — see
//     internal/llm/providers/helixllm/provider.go's package doc — so
//     substituting HelixAgent's placeholder there would silently redirect a
//     malformed HelixLLM host to a machine meant for a completely different
//     service, which is worse than the "wrong host, right port" case the
//     upstream research already flags).
//
// The safe DEFAULT for a package with no idea what it is addressing is to
// change NOTHING it was not asked to change: pass a malformed host through
// un-repaired and let the dial/connect fail loudly, which is the correct and
// honest outcome (§11.4.6) — never invent a destination. Default-substitution
// is used ONLY at the one call site where the substituted value is verified
// correct for that EXACT target, matching sibling code already resolving the
// same logical address the same way.
//
// # "Fail loudly" is not the same guarantee on both sides (BLOCKING 1)
//
// A 2026-08-12 review measured that the paragraph above held for DialAddress
// but NOT for BaseURL, and the gap is structural rather than an oversight:
// the two functions hand their output to readers with opposite failure modes.
//
// net.Dial treats the host as an opaque label, so an unrepaired malformed
// host really does fail loudly there — net.Dial("tcp", "hx@yz:7061") cannot
// resolve "hx@yz" and errors. A URL PARSER does not: each of the four RFC
// 3986 gen-delims ENDS the host component, so a host carrying one is not
// rejected, it is RE-READ. Measured against this repository's Go toolchain
// (2026-08-12), with the four bytes passed through raw:
//
//	"hx@yz"      -> "http://hx@yz:7061"      host "yz"  — the WRONG HOST, "hx" became userinfo
//	"hx/yz"      -> "http://hx/yz:7061"      host "hx", port SILENTLY DROPPED, path "/yz:7061"
//	"hx?yz"      -> "http://hx?yz:7061"      host "hx", port SILENTLY DROPPED, query "yz:7061"
//	"hx#yz"      -> "http://hx#yz:7061"      host "hx", port SILENTLY DROPPED, fragment "yz:7061"
//
// The "@" row is the one that breaks the promise outright: it does not fail
// at all. It parses cleanly, connects to a DIFFERENT HOST than the operator
// configured, and nothing downstream can notice — precisely the invented
// destination this package exists to refuse. (The other three are milder:
// they drop the port rather than redirect the host.) These are reachable
// from operator config at every BaseURL site — config.QdrantHost,
// config.ChromaHost, the Flink JobManagerHost.
//
// The sibling helixendpoint.normalizeHost already rejects exactly these four
// bytes, with the same four worked examples in its HXC-269 comment; that
// guard was not carried across when this package was written.
//
// It cannot be carried across VERBATIM, because "reject" means different
// things on the two sides. normalizeHost signals rejection by returning
// ok=false, and its caller then substitutes DefaultHost — the one thing this
// package must never do (a third-party service silently swapped for
// HelixAgent's placeholder is the "wrong host, right port" outcome the
// default-substitution decision above rules out), and BaseURL's signature
// returns a bare string with nowhere to put an error.
//
// So the rejection is expressed in the only vocabulary a URL has: the four
// bytes are PERCENT-ENCODED so they can no longer act as delimiters. This
// keeps every byte the operator actually wrote, invents nothing, and makes
// net/url's own parser refuse the result — measured, same session:
//
//	"hx@yz"      -> "http://hx%40yz:7061"      url.Parse: invalid URL escape "%40"
//	"hx/yz"      -> "http://hx%2Fyz:7061"      url.Parse: invalid URL escape "%2F"
//	"hx?yz"      -> "http://hx%3Fyz:7061"      url.Parse: invalid URL escape "%3F"
//	"hx#yz"      -> "http://hx%23yz:7061"      url.Parse: invalid URL escape "%23"
//	"evil.com/x" -> "http://evil.com%2Fx:7061" url.Parse: invalid URL escape "%2F"
//
// (net/url permits percent-escapes in a host only for non-ASCII bytes — the
// RFC 3986 rule, with the RFC 6874 "%25" zone delimiter as the sole
// exception — so an escape whose first hex digit is < 8 is an error by
// construction. That is the same mechanism this package already relies on
// for zone IDs, used here in the opposite direction: there to make a legal
// host parseable, here to make an illegal one refuse to parse.)
//
// The failure surfaces at the first http.NewRequest / url.Parse the caller
// performs, quoting the offending bytes back, and the port is preserved in
// the string rather than silently displaced. This treatment is applied ONLY on
// the URL-composing path (BaseURL, BaseURLString, DialAddressForURL).
// DialAddress keeps the raw pass-through, because there the pass-through
// genuinely does fail loudly and an encoded byte would corrupt the net.Dial
// call it exists to serve.
//
// # The audit above missed "[" — it measured the host, not the authority (HXC-286-B)
//
// The four-byte set was closed with "bytes that make url.Parse FAIL on their
// own are already loud", and "[" was placed in that already-loud list. Measured
// 2026-09-03 against go1.26, that is FALSE for "[", and the reason is a scope
// error worth stating plainly, because it is the kind that recurs: the audit
// was run on the RAW HOST, but what ships to a URL reader is the COMPOSED
// AUTHORITY, and composition CHANGES the verdict.
//
// A bare "[" really is loud by itself — "http://a[b:6379" is refused with
// `missing ']' in host`, because there is no "]" anywhere. But every builder
// in this package ends in net.JoinHostPort, which brackets any host containing
// a colon — and so MANUFACTURES the "]" whose absence was doing all the
// rejecting. url.Parse then finds the IP-literal with
// strings.LastIndex(host, "[") (go1.26 net/url/url.go:550), which silently
// DISCARDS everything before the last "[" instead of refusing it:
//
//	"[::1"                 -> "http://[[::1]:6379"                 PARSES, hostname "::1"
//	"[2001:db8::5"         -> "http://[[2001:db8::5]:6379"         PARSES, hostname "2001:db8::5"
//	"db.prod.internal[::1" -> "http://[db.prod.internal[::1]:6379" PARSES, hostname "::1"
//
// The third row is the one that breaks the promise outright, and it is the
// same shape as the "@" row above: the operator NAMED "db.prod.internal", and
// what came back points at loopback with the name thrown away — an invented
// destination, reached from an ordinary typo (one bracket where two were
// meant) in exactly the operator-supplied config fields listed above. Measured
// end-to-end through a real caller, flink Config.GetRESTURL with
// jobmanager_host: "[::1", before the fix: "http://[[::1]:8082", hostname
// "::1".
//
// The dial side was never affected — net.SplitHostPort("[[::1]:6379") errors
// with `unexpected '[' in address`, so net.Dial refuses it. The hole was
// URL-side only, which is why the fix is URL-side only.
//
// So "[" and "]" join the encoded set, by the SAME mechanism and for the same
// reason as the original four (see encodeGenDelims for the ordering argument
// that keeps a well-formed "[::1]" untouched):
//
//	"[::1"                 -> "http://[%5B::1]:6379"                 invalid URL escape "%5B"
//	"db.prod.internal[::1" -> "http://[db.prod.internal%5B::1]:6379" invalid URL escape "%5B"
//	"::1]"                 -> "http://[::1%5D]:6379"                 invalid URL escape "%5D"
//
// No legitimate host is affected: hostnames, IPv4 literals, IPv6 literals
// (bare, or bracketed and unwrapped by urlEncodeZone before encodeGenDelims
// runs) and zone IDs contain none of these six bytes where this guard sees
// them.
//
// # Zone-identifier splitting (research finding, carried forward honestly)
//
// The meta-repo research artifact (§1, "the load-bearing new finding")
// establishes that Go's own stdlib splits an IPv6 zone at the LAST '%'
// (src/net/ipsock.go splitHostZone), while llms_verifier's normalizeHost
// guard splits at the FIRST. DialAddress does not implement its own
// zone-splitting at all — unbracket-then-JoinHostPort passes a zone-qualified
// host straight through to net.JoinHostPort, which treats the whole bracketed
// or unbracketed string as an opaque host component and performs no
// interpretation of '%' — so DialAddress inherits neither convention and
// cannot disagree with the stdlib about where a zone starts, and net.Dial
// itself expects the zone RAW (unencoded), so leaving it alone is correct.
//
// BaseURL is different, and this is a finding this package's own STEP-3 test
// enumeration (§11.4.146) surfaced empirically, not something assumed from
// the research artifact alone: BaseURL's OUTPUT is a string a URL PARSER
// will later re-read (an http.Client re-parses the address it dials), and a
// raw, unescaped '%' is ALWAYS a percent-escape trigger to net/url.Parse —
// "fe80::1%eth0" makes it try to decode "%et" as a hex escape and fail
// outright ("invalid URL escape \"%et\""). So BaseURL percent-encodes a zone
// ID per RFC 6874 ("fe80::1%eth0" -> "fe80::1%25eth0") before composing,
// splitting at the LAST '%' rather than reproducing llms_verifier's
// first-'%' choice.
//
// The LAST-vs-FIRST choice IS load-bearing, and an earlier revision of this
// comment got its reason wrong (MUST-FIX 1 of a second 2026-08-12 review).
// That revision recorded the mutation as UNASSERTED — true, and confirmed:
// flipping LastIndex to Index survived every test here. (An earlier revision
// added "the only survivor of a 9-mutation sweep"; that RANKING is softened
// per F6 below — the sweep left no artefact in this tree, so a reader cannot
// check it. The survival ITSELF is reproducible in one command: flip the
// LastIndex and run this package's tests.) But it then justified the survival
// by claiming the two
// conventions disagree only on "pathological multi-'%' inputs, every one of
// which is unparseable under EITHER convention". That claim is measurably
// FALSE. Counter-example, measured against this repository's Go toolchain in
// the session that corrected it — host "fe80::1%eth0%25":
//
//	LAST-'%'  -> "http://[fe80::1%eth0%2525]:8100"  url.Parse FAILS: invalid URL escape "%et"
//	FIRST-'%' -> "http://[fe80::1%25eth0%25]:8100"  url.Parse OK, hostname "fe80::1%eth0%"
//
// One parses and the other does not, so they are not equivalent — and the
// direction of the difference is what makes the choice matter. FIRST-'%'
// does not merely produce a different string: it produces a host the caller
// never wrote ("fe80::1%eth0%", a trailing-'%' zone conjured by re-reading
// the caller's own "%25" as this function's delimiter), which is exactly the
// invented destination the default-substitution decision above forbids.
// LAST-'%' produces a string that refuses to parse — the honest outcome for
// an input this function cannot faithfully express (§11.4.6).
//
// So the mutant is no longer unasserted:
// TestUrlEncodeZone_LastPercentConvention_RefusesRatherThanInvents pins it,
// and flipping LastIndex to Index now FAILS that test. The alignment with
// the Go stdlib's own splitHostZone convention remains a real secondary
// benefit; it is no longer the whole justification.
//
// Honest boundary (§11.4.6) on what this does NOT claim: the earlier
// revision's narrower sub-claim — that no VALID address the two builders in
// this family compose can disagree because of this choice — still holds, and
// the same session measured that "fe80::1%eth0%25" is in fact the input on
// which netaddr and helixendpoint's normalizeHost ALREADY disagree today
// (helixendpoint splits at the FIRST '%' and yields the invented
// "fe80::1%eth0%"; this package refuses). The divergence is confined to that
// unexpressible shape, and on it this package is the SAFER of the two — so
// it is recorded here rather than "fixed" by matching the weaker behaviour.
//
// DialAddress is NEVER given zone-encoding at all — encoding would break the
// exact net.Dial calls it exists to serve.
//
// # Scope: what this round does NOT close (§11.4.6, §11.4.118)
//
// This round routes NINE sites through this package (internal/cache/redis,
// internal/llm/providers/helixllm, internal/messaging/broker,
// internal/messaging/rabbitmq/connection, internal/ports,
// internal/search/store/{chroma,qdrant}, internal/streaming/flink/config,
// internal/vectordb/qdrant/config) and leaves the rest OPEN. The family is
// therefore NOT closed, and no closure note for this work may say that it is.
//
// THIS LIST IS PARTIAL — it is a floor, not a census (§11.4.6). An earlier
// revision framed the nine below as "the remainder ... a named set rather
// than a vague '~30 more'", invoking §11.4.118 enumerated coverage. That
// framing overclaimed: it is an enumeration of what one sweep found, not of
// what exists, and it was also internally inconsistent (it said "~30 found /
// 9 routed / rest open", implying ~21 remaining, then named 9 as the
// remainder).
//
// NO TOTAL IS ASSERTED HERE, and that refusal is itself a measured finding.
// Two independent sweeps of this tree on 2026-08-13 — same defect class, same
// day, differing only in how widely they cast — returned THIRTEEN and then
// TWENTY-FIVE. The second found eleven the first missed, all genuine and all
// verified by reading the source line (`postgres://%s:%s@%s:%s/...`-style DSNs
// and credentialed broker URIs, whose host+port halves the first sweep's
// narrower operand-shape filter skipped). Every revision of this paragraph so
// far has named a number and been wrong; the fourth guess would be worth no
// more than the first three. What follows is therefore an enumeration of what
// the sweeps FOUND — a lower bound with an explicitly UNKNOWN remainder, not
// a census, and §11.4.118 enumerated coverage is NOT claimed for it.
//
// A THIRD independent sweep the same day, run without sight of this list,
// reproduced the SET below exactly — every site it lists, and none it missed.
// That is the first sweep-to-sweep agreement in this family, and it is worth
// recording for what it does and does not establish. It raises confidence
// that the enumerated entries are real and that no site of these shapes was
// dropped; it does NOT convert the list into a census, because the third
// sweep filtered for the SAME composition shapes as the second (§11.4.6 — two
// instruments that share a blind spot agree about everything except that blind
// spot). Its arithmetic differed again (it reported TWENTY-SEVEN against the
// second sweep's TWENTY-FIVE) purely as a counting-boundary artefact, not a
// disagreement about membership: it counts each generator DSN line as its own
// site where the entries below fold several onto one line, and it counts the
// internal/testutil sites the reachability split below records separately.
// A number that moves while the underlying set does not is further reason to
// enumerate rather than total. Its one substantive correction — the HTTP/3
// listen address is at main.go:2442, not :2440 — is applied above.
//
// PROVENANCE OF THE SWEEP HISTORY (F6 of the round-5 review, 2026-08-13).
// The sweep narrative above — "9-mutation sweep", "THIRTEEN and then
// TWENTY-FIVE", "a THIRD independent sweep reproduced the SET exactly",
// "TWENTY-SEVEN as a counting-boundary artefact" — left NO artefact in this
// tree. The only place those runs exist is these comment lines, so the HISTORY
// is unfalsifiable from the repository and is recorded as narrative, not as
// evidence a reader can check (§11.4.6).
//
// The distinction that matters: the CONTENT claims are independently
// re-derivable and were re-derived — every enumerated entry below resolves to
// a real line, and the `Method:` recipe re-runs. It is the SEQUENCE (how many
// sweeps, in what order, agreeing how far) that has no captured proof. Treat
// the enumerated set as the finding and the sweep count as an anecdote; if a
// future round wants the history to carry weight, it must commit the run
// artefacts rather than restate the numbers.
//
// Method: sweep of ./internal + ./cmd (excluding *_test.go) for `%s:%d` /
// `%s:%s` / `%s:%v` / `x + ":" + y` compositions whose operands are a network
// host and port, each hit hand-classified to drop cache/map keys, docker
// mounts, file:line references, log-only fields, credential pairs, and
// port-only bind addresses (`":" + port` has no host operand, so it cannot
// exhibit this defect).
//
// REACHABLE from at least one of the eight ./cmd/* closures:
//
//	internal/config/config.go:68                       ServiceEndpoint.ResolvedURL()'s `e.Host + ":" + port` — the
//	                                                   most consequential of the set: SIX production callers, of which
//	                                                   FOUR are connection-affecting (internal/services/health_checker.go:87
//	                                                   TCP dial + :121 HTTP base, internal/services/discovery/discoverer.go:75
//	                                                   TCP probe + :105 HTTP probe) and TWO are log-only
//	                                                   (internal/router/router.go:461,465 — still real callers of the
//	                                                   defective function, so counted)
//	internal/database/sqldb.go:47                      postgres:// DSN
//	internal/messaging/init.go:252                     amqp:// URI (the RabbitMQ path that IS wired, unlike broker.go's)
//	internal/analytics/clickhouse.go:82                clickhouse:// DSN
//	internal/services/concurrency_alert_manager.go:1582  fed straight to smtp.SendMail
//	internal/services/plugin_system.go:682             plugin health URL
//	internal/mcp/servers/redis_adapter.go:107          the SAME Redis Addr defect already fixed in internal/cache/redis.go
//	internal/mcp/config/generator.go:159,177,194       postgresql:// / redis:// / mongodb:// defaults
//	internal/mcp/config/generator_full.go:208,222,234,248  postgresql:// / redis:// / mongodb:// / mysql:// defaults
//	internal/mcp/config/generator_container.go:237     http://%s:%d/sse
//	internal/sanity/boot_check.go:152,252              health-check URLs
//	cmd/helixagent/main.go:305                         dependencyURL()'s "http://%s:%s%s"
//	cmd/helixagent/main.go:802                         postgres:// DSN
//	cmd/helixagent/main.go:2442                        the HTTP/3 listen Address "%s:%s"
//
// SAME DEFECT, but the containing package is in NONE of the eight closures
// (recorded so the two populations are never silently merged, per the
// reachability section below):
//
//	internal/vectordb/milvus/client.go:70
//	internal/messaging/kafka/broker.go:506,551
//	internal/streaming/state_store.go:202              the SAME Redis Addr defect as redis_adapter.go:107 above
//	internal/testutil/infra.go:305,576,585             test-support package (not a _test.go file, so it survives the
//	                                                   filter, but it exists to serve tests)
//
// Excluded with the reason stated, so the boundary is auditable rather than
// implicit: internal/streaming/state_store.go:216 (the SAME address recomposed
// for a zap LOG FIELD — misleading for IPv6 but not connection-affecting; the
// dial at :202 above is the defect) and internal/messaging/broker.go:411
// (`c.Username + ":" + c.Password` — a credential pair, not a host and port;
// its authority already comes from netaddr.DialAddressForURL).
//
// (F3 of the round-5 review: that citation read ":404" and matched NO
// baseline. Re-measured 2026-08-13 — :404 is the `func (c *BrokerConfig)
// ConnectionString() string {` line in the WORKTREE, and the concatenation is
// at :411; in HEAD the concatenation is at :382 and the file is 401 lines
// long, so ":404" is past EOF there. Corrected to the worktree line, which is
// the baseline this whole sweep section uses.)
//
// The sweep covers those composition shapes only. Sites spelled another way
// are neither found nor excluded by it — which, given a 13→25 revision inside
// one day, is the operative caveat and not a formality. Line numbers above are
// a 2026-08-13 WORKTREE snapshot (see "Line-number baselines" below) and will
// drift; the identifying facts are the file and the naive-concatenation idiom.
//
// # SECOND POPULATION: net.JoinHostPort (F2, 2026-08-13)
//
// RETRACTION. An earlier revision of the paragraph above closed with "(The
// net.JoinHostPort call sites in this tree were checked and are correct by
// construction, so they add nothing.)" That sentence is WITHDRAWN — it is
// FALSE, and it hid a live defect on a production dial path.
//
// JoinHostPort is correct by construction only for an UNBRACKETED host. Handed
// one that is ALREADY bracketed it brackets a SECOND time, and the result is
// not merely ugly — it is unsplittable and unparseable. Measured against this
// repository's Go toolchain (2026-08-13):
//
//	net.JoinHostPort("[::1]", "6379") = "[[::1]]:6379"
//	  net.SplitHostPort -> address [[::1]]:6379: missing port in address
//	  url.Parse         -> invalid IP-literal
//
// This document already KNEW that mechanism — the opening section describes
// the double-bracket shape and correctly notes "none of THIS package's own
// sites reach that path". The false step was generalising that local finding
// to every JoinHostPort site in the tree.
//
// The site it hid: `ports.RedisAddr()`, cited at its COMPOSE line — the join
// itself, where the defect lives (internal/ports/redis.go — HEAD :98,
// worktree :145; see the baseline note below). Its host comes from
// `RedisHost()`, cited at its FUNC line, since the whole body is the subject
// (HEAD :67, worktree :68), which reads REDIS_HOST with NO
// bracket handling — so `REDIS_HOST=[::1]`, the form an operator copies
// straight out of a URL, composed to `[[::1]]:6379`.
//
// PRECISION (round-8 review finding 6): an earlier revision called RedisHost
// "a BARE `strings.TrimSpace(os.Getenv("REDIS_HOST"))`". It is not bare — it
// trims, guards for non-empty, and falls back to `DefaultRedisHost`. The
// load-bearing sub-claim is unaffected and remains TRUE: none of those steps
// is bracket handling, so a bracketed REDIS_HOST is passed through verbatim
// and the `[[::1]]:6379` conclusion stands. Only the word "bare" was wrong.
//
// It is REACHABLE FROM THE PRODUCTION BINARY, unlike all five of the
// routed-but-dead sites recorded in the reachability section below:
// cmd/helixagent/main.go:842 passes `ports.RedisAddr()` as the go-redis client's
// dial `Addr`, and :630 as the "redis" entry of the dependency health-probe map.
//
// The inversion is the point. This round routed `ports.HostPort` — ZERO callers
// module-wide, one of the five dead sites named below — through netaddr, while
// leaving `ports.RedisAddr`, in the SAME PACKAGE and on the LIVE dial path,
// naive. The two then disagreed about what "the dialable host:port" means for
// identical input: exactly the "two builders privately disagreeing about the
// SAME logical endpoint" defect the default-substitution section above names as
// this family's core, occurring INSIDE one package. Fixed 2026-08-13 —
// `RedisAddr` now composes through `netaddr.DialAddressString`, pinned by
// `TestRedisAddr_AlreadyBracketedIPv6HostIsNotDoubleBracketed` and
// `TestRedisAddr_AgreesWithHostPort` in internal/ports/redis_test.go.
//
// ENUMERATION of the JoinHostPort population, so it is a stated set rather
// than an implicit "checked, fine" (§11.4.6 / §11.4.118). Method: grep for
// `net.JoinHostPort` across ./internal + ./cmd excluding *_test.go; each hit
// classified by whether its HOST operand can arrive already-bracketed from
// operator config or a caller parameter, and by whether its package is in one
// of the eight ./cmd/* dependency closures (measured with
// `go list -deps ./cmd/<x>`, 2026-08-13).
//
// FIXED this round:
//
//	internal/ports/redis.go:98        RedisAddr — REACHABLE (4/8 closures:
//	                                 api, generate-constitution, grpc-server,
//	                                 helixagent) AND live at main.go:842/:630
//
// CORRECT BY CONSTRUCTION (the only site the retracted sentence was right
// about — it strips brackets FIRST, which is the whole point of the helper):
//
//	internal/netaddr/netaddr.go   func DialAddressString:
//	                              JoinHostPort(stripBracket(host), port)
//
// NOTE on the citation above (round-8 review finding 2). An earlier revision
// wrote this row as "netaddr.go:433". That was WRONG by 139 lines — and it
// went wrong the same way the broker.go:404 staleness F3 was fixing did: this
// file is ~92% comment, so THIS ROUND'S OWN prose insertions (the retraction,
// this enumeration table, the F4 baseline note, the HXC-327 follow-up) pushed
// the cited code down and invalidated the citation while it sat unchanged.
// A file whose prose keeps growing cannot carry a stable SELF-citation by line
// number, and the package is untracked, so no HEAD baseline can rescue one.
// The row therefore cites the FUNCTION NAME only — the stable identifier — and
// every other self-citation in this file MUST do the same. Line numbers are
// used here only for OTHER files, and only with their baseline declared below.
//
// SUSPECTED, NOT INDIVIDUALLY TRACED — enumerated here rather than silently
// fixed, because each needs its own reachability + host-provenance analysis
// and a §11.4.115 RED baseline before it may be touched (§11.4.124: "the host
// looks config-sourced" is not proof). Each takes a config- or
// parameter-sourced host that this sweep did NOT prove can never arrive
// bracketed. Reachability is PACKAGE-level (necessary, not sufficient — per
// the reachability section below, symbol-level was NOT separately traced for
// these):
//
//	internal/sanity/boot_check.go:198,225        config.PostgresHost / config.RedisHost;
//	                                             package in 1/8 closures (sanity-check)
//	internal/services/plugin_system.go:716       InstanceInfo.Address; 4/8 closures
//	internal/services/protocol_federation.go:308 server.Address; 4/8 closures
//	internal/adapters/containers/adapter.go:554  HealthCheckTCP(host, …) parameter; 4/8 closures
//	internal/search/service.go:226               vectorStoreReachable(host, …) parameter;
//	                                             1/8 closures (helixagent)
//	cmd/helixagent/infrastructure.go:281         checkTCPPort(host, …) parameter; IS the
//	                                             helixagent main package
//	internal/testutil/infra.go:147,226,592       test-support package (not a _test.go file,
//	                                             so it survives the filter); package in
//	                                             NO cmd closure
//
// This enumeration is a lower bound on the JoinHostPort shape with an
// explicitly UNKNOWN remainder, exactly as the concatenation list above is —
// §11.4.118 enumerated coverage is NOT claimed for it.
//
// # Line-number baselines (F4, declared 2026-08-13; corrected round 8)
//
// This changeset carries TWO baselines for cmd/helixagent/main.go, and an
// earlier revision stated neither. Both are correct for their purpose; the
// hazard was only that a reader could not tell which was which:
//
//   - THIS FILE cites main.go by WORKTREE line (:305, :802, :2442) — it
//     describes the post-fix tree, and all three were re-verified 2026-08-13.
//   - The GROUP-A RED test (cmd/helixagent/address_composition_red_test.go)
//     cites main.go by PRE-FIX HEAD line (:1948, :4460, :4508) — the artifact
//     its §11.4.115 RED baseline reproduces the defect against, which is the
//     required baseline for a RED test. Its present-tense phrasing is about
//     that HEAD artifact, and the file now says so in its own header.
//
// A THIRD baseline was in use but UNDECLARED (round-8 review finding 3) —
// F4's own discipline, violated by F4. The retraction section above cites
// internal/ports/redis.go by PRE-FIX HEAD line, not by worktree line, because
// it describes the defect as it stood BEFORE this round fixed it. Adjudicated
// against git 2026-08-13:
//
//	                      HEAD   worktree
//	RedisAddr's compose    :98      :145
//	func RedisHost         :67       :68
//
// Both numbers were CORRECT — against a baseline the reader was never told
// about, while F4 declared main.go as the only file carrying two. Every
// redis.go citation in this file now names its baseline inline.
//
// In all three places the stable identifier is the FUNCTION NAME each citation
// already carries; line numbers are a convenience that will drift. For
// citations of THIS file, see finding 2 above: no line number is used at all,
// because this file's own prose growth invalidates them.
//
// # Reachability of the sites this round DID fix (§11.4.6)
//
// Not every routed site is equally user-reachable, and the closure note for
// this work must not imply otherwise. An earlier revision of this paragraph
// said "TWO of them have NO production caller at all" and asserted that the
// live AMQP path (rabbitmq/connection.go's buildURL) "IS wired". Both were
// wrong: the count is FIVE, and buildURL is one of the five.
//
// Re-measured against this tree (2026-08-13) by two instruments — package
// reachability via `go list -deps ./cmd/<x>` for each of the 8 commands (api,
// audit, cognee-mock, generate-constitution, grpc-server, helixagent,
// mcp-bridge, sanity-check), and symbol reachability via a module-wide search
// for non-test callers. FIVE of the nine routed sites are NOT reachable from
// any production entry point:
//
//	internal/ports (HostPort/HTTPURL/HTTPSURL)   package IS in 4 closures, but these three
//	                                            symbols have ZERO production callers —
//	                                            only this doc comment and their own tests.
//	                                            SCOPE (round-8 finding 5): this row is about
//	                                            THOSE THREE SYMBOLS, never the package —
//	                                            internal/ports also carries RedisAddr, routed
//	                                            this same round and LIVE (see below).
//	internal/messaging (BrokerConfig.Connection- package IS in 4 closures, but the method
//	  String)                                   has ZERO PRODUCTION callers — only its own
//	                                            tests (4 call sites in
//	                                            internal/messaging/address_composition_red_test.go).
//	                                            An earlier revision said "ZERO callers
//	                                            module-wide" and dropped the qualifier the
//	                                            adjacent ports row carries; measured
//	                                            2026-08-13, that literal was FALSE
//	                                            (round-8 finding 7).
//	internal/messaging/rabbitmq (buildURL)      package is in NO cmd closure
//	internal/streaming/flink (GetRESTURL)       package is in NO cmd closure
//	internal/vectordb/qdrant (config accessors) package is in NO cmd closure
//
// The remaining FOUR — internal/cache/redis, internal/llm/providers/helixllm,
// internal/search/store/chroma, internal/search/store/qdrant — sit in packages
// that ARE in a cmd closure (the first two in 4 of 8, the store pair in
// helixagent). Honest boundary (§11.4.6): package-in-closure is a NECESSARY
// condition for user-reachability, not a sufficient one — whether each specific
// accessor is invoked on a live request path was NOT separately traced, so
// "in a production closure" is the claim, and "user-reachable" is not.
//
// The two instruments are not interchangeable, which is exactly how the earlier
// undercount happened: ports and messaging LOOK reachable at package level and
// are dead at symbol level. Neither check alone is sufficient.
//
// RE-DERIVED AFTER F2 (round-8 review finding 5). F2 — landed in this SAME
// round — established that `ports.RedisAddr` IS live at main.go:842/:630, and
// RedisAddr lives in internal/ports, one of the five rows above. The earlier
// conclusion "the five tests therefore prove a synthetic failure, not an
// end-user one" was written before that and no longer parses cleanly, because
// it read the ports row at PACKAGE granularity while the row itself is
// SYMBOL-scoped. Restated at the granularity the evidence actually supports:
//
//   - FOUR rows are wholly synthetic at both granularities — internal/messaging
//     (ConnectionString), internal/messaging/rabbitmq (buildURL),
//     internal/streaming/flink (GetRESTURL), internal/vectordb/qdrant (config
//     accessors). Their tests prove a synthetic failure, not an end-user one.
//   - The FIFTH row is symbol-scoped only. internal/ports/{HostPort,HTTPURL,
//     HTTPSURL} have no production caller, so THOSE THREE tests are synthetic;
//     but the package also carries RedisAddr, routed this same round, whose
//     defect WAS user-reachable. "internal/ports is unreachable" is therefore
//     FALSE as stated about the package and TRUE only about those three
//     symbols.
//
// A counting caveat, so the arithmetic is not mistaken for a census (§11.4.6):
// the "nine" enumerated in the Scope section counts PACKAGES/FILES, and TEN
// non-test files import this package (internal/ports contributes both ports.go
// and redis.go). The list mixes package and symbol granularity by necessity —
// the reachability verdict is per-SYMBOL — so neither number should be read as
// a site count.
//
// The synthetic rows are still worth fixing (a future caller inherits the
// correct behaviour rather than the defect), but they are not evidence of user
// impact. F2 is — and it is the one row that is.
package netaddr

import (
	"net"
	"strings"

	"digital.vasic.llmsverifier/pkg/helixendpoint"
)

// DialAddress builds a "host:port" address for net.Dial-style consumers
// (redis, kafka, tcp dialers, SMTP relays) addressing a service HelixAgent
// does not own. It brackets an IPv6 literal per RFC 3986 §3.2.2 so
// "fe80::1" + 6379 yields "[fe80::1]:6379" and not the unusable
// "fe80::1:6379" a naive fmt.Sprintf("%s:%d", host, port) produces. An
// already-bracketed host is left alone rather than double-bracketed.
//
// NO placeholder fallback: a malformed host is passed through unrepaired so
// the eventual dial fails loudly against the actual bad input, rather than
// silently connecting to a different, wrong machine holding the right port
// (see the package doc's default-substitution decision).
//
// This is a thin, verbatim re-export of llms_verifier's own
// helixendpoint.DialAddress (§11.4.74) — the two packages share the exact
// same contract by construction, not by coincidence.
func DialAddress(host string, port int) string {
	return helixendpoint.DialAddress(host, port)
}

// stripBracket trims surrounding whitespace, then strips one layer of IPv6
// literal brackets — mirroring llms_verifier's own unexported
// helixendpoint.unbracket() byte-for-byte (TrimSpace, then a single
// len>1-and-first-is-'['-and-last-is-']' strip).
//
// This is the ONE definition every bracket-detecting entry point in THIS
// package (DialAddressString, urlEncodeZone) calls, so the two can never
// again silently disagree about what "the host, unbracketed" means for the
// same input — the exact regression a 2026-08-12 review measured
// (MUST-FIX 2): DialAddressString omitted TrimSpace while DialAddress (via
// helixendpoint.unbracket) had it, so DialAddress("  2001:db8::1  ", 6379)
// and DialAddressString("  2001:db8::1  ", "6379") produced two DIFFERENT
// addresses for the identical input — reachable in production because
// internal/cache's RedisConfig.Host is never trimmed before reaching
// DialAddressString. helixendpoint does not export unbracket(), so this
// reimplements the SAME 3-line primitive rather than duplicating the
// higher-level DialAddress/BaseURL functions (§11.4.74) — one independently
// tested pure function, now shared by every caller in this package that
// needs it.
func stripBracket(host string) string {
	host = strings.TrimSpace(host)
	if len(host) > 1 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return host
}

// DialAddressString is DialAddress for a caller whose port is already a
// string (a config field typed `Port string`, e.g. internal/config's
// RedisConfig) rather than an int.
//
// It exists rather than forcing every such caller through
// strconv.Atoi(port)-then-DialAddress because that round trip is LOSSY on a
// malformed value: DialAddress performs no port validation at all (matching
// helixendpoint.DialAddress's own documented contract — a bad port fails
// loudly at dial time, which is correct), and an Atoi failure would force
// this function to invent a fallback int or an error return neither
// DialAddress nor its callers expect. It shares stripBracket with every
// other entry point in this package (§11.4.74) rather than repeating the
// bracket-detection inline.
func DialAddressString(host, port string) string {
	return net.JoinHostPort(stripBracket(host), port)
}

// BaseURLString is BaseURL for a caller whose port is already a string (an
// env-var-sourced port, e.g. HELIX_LLM_PORT in
// internal/llm/providers/helixllm) — the URL-composing sibling of
// DialAddressString, carrying the same RFC 6874 zone-encoding BaseURL
// applies (see DialAddressForURL) rather than DialAddressString's raw
// pass-through, because this output is also read by a URL parser, not
// handed to net.Dial.
func BaseURLString(scheme, host, port string) string {
	return scheme + "://" + DialAddressString(urlSafeHost(host), port)
}

// BaseURL builds "scheme://host:port" for a third-party HTTP(S) service
// (Qdrant, ChromaDB, Flink's REST API, and similar), with the SAME
// no-default-substitution contract as DialAddress: a malformed host is never
// silently swapped for a different placeholder host, because that
// placeholder would name the wrong service's default, not the one this
// address is meant to reach.
//
// Unlike helixendpoint.BaseURL — which substitutes DefaultHost ("localhost")
// on failure, correct only when the address being built IS HelixAgent's own
// endpoint — this function has no opinion about what a "usable" host looks
// like beyond what DialAddress already enforces (bracket-safety). A caller
// that wants HelixAgent's own default-substituting behaviour for HelixAgent's
// own endpoint should call helixendpoint.BaseURL directly, not this function.
//
// An IPv6 zone identifier is percent-encoded per RFC 6874 before
// composition (see the package doc's "zone-identifier splitting" section) —
// DialAddress alone does not do this, because DialAddress's callers hand the
// result to net.Dial, which wants the zone raw.
//
// A host carrying one of the six RFC 3986 gen-delims that END or RE-DELIMIT
// the host component ("/", "?", "#", "@", and an unpaired or interior "[",
// "]") is likewise percent-encoded, so a URL parser cannot re-read it as a
// path, query, fragment, userinfo or IP-literal boundary. That is what makes
// this function's no-substitution contract actually hold for a URL reader
// rather than only for a dialer: without it, "hx@yz" parses cleanly as host
// "yz", and "db.prod.internal[::1" parses cleanly as host "::1" — each
// connecting to a machine the caller never named. See the package doc's
// "'Fail loudly' is not the same guarantee on both sides" section and the
// "[" subsection that follows it for the measured evidence.
func BaseURL(scheme, host string, port int) string {
	return scheme + "://" + DialAddressForURL(host, port)
}

// DialAddressForURL is DialAddress with the SAME RFC 6874 zone-encoding
// BaseURL applies, exposed separately for callers that need the bracketed
// "host:port" authority WITHOUT a scheme prefix — typically because they
// must splice userinfo (user:pass@) between the scheme and the authority,
// as an AMQP or similar credentialed URI does. BaseURL is defined in terms
// of this function, not the other way around, so the two can never drift
// out of agreement.
func DialAddressForURL(host string, port int) string {
	return DialAddress(urlSafeHost(host), port)
}

// urlSafeHost applies BOTH URL-safety transforms this package owns, in the
// ONE order that composes: RFC 6874 zone-encoding first, then gen-delimiter
// neutralisation.
//
// The order is load-bearing rather than incidental. encodeGenDelims INTRODUCES
// percent-escapes ("@" -> "%40"), and urlEncodeZone decides where a zone
// starts by scanning for the LAST '%'. Run the other way round, a host with no
// zone at all ("hx@yz") would acquire a "%40" that urlEncodeZone then reads as
// a zone delimiter and re-encodes into "%2540" — a mangling of the caller's
// input, and exactly the invent-a-destination failure both transforms exist to
// prevent. Zone-first is safe in the other direction because encodeGenDelims
// rewrites only "/?#@" and never touches a '%', so the "%25" urlEncodeZone
// emits survives it untouched.
//
// This is the single definition both URL-composing entry points (BaseURL via
// DialAddressForURL, BaseURLString via DialAddressString) call, so the two can
// never disagree about what "the host, made safe for a URL reader" means
// (§11.4.74 — one definition, never two that can drift).
func urlSafeHost(host string) string {
	return encodeGenDelims(urlEncodeZone(host))
}

// encodeGenDelims percent-encodes the six RFC 3986 gen-delims that can
// terminate or re-delimit the host component of a URL, so a host carrying one
// cannot be re-read by a parser as a path, query, fragment, userinfo boundary,
// or IP-literal boundary.
//
// This is the URL-side expression of "reject" for a function that has nowhere
// to return an error and must never substitute a different host: the offending
// bytes are preserved verbatim (as their escapes) and net/url's own parser then
// refuses the result, quoting them back — see the package doc's BLOCKING-1
// section for the measured before/after.
//
// The rejection is ONE mechanism, not six special cases. net/url's own
// unescape() refuses ANY percent-escape of an ASCII byte inside a host, with
// "%25" (the RFC 6874 zone marker) as the single exception — go1.26
// net/url/url.go:127, `mode == encodeHost && unhex(s[i+1]) < 8 &&
// s[i:i+3] != "%25"` -> EscapeError. Every escape this function emits has a
// leading hex digit below 8, so each one lands on that same branch:
//
//	"%2F" "%3F" "%23" "%40" "%5B" "%5D"  ->  invalid URL escape "%XX"
//
// # "[" was WRONGLY classified as self-loud (measured 2026-09-03, go1.26)
//
// An earlier revision touched only four bytes and justified excluding "["
// like this: *Bytes that make url.Parse FAIL on their own (space, "[", "\\",
// "^", "`", "{", "|", "}") are already loud and are deliberately left alone.*
//
// That claim is FALSE for "[", and the error was one of scope: it was measured
// against the RAW host, never against the COMPOSED authority this package
// actually emits. A bare "[" IS loud on its own ("http://a[b:6379" ->
// `missing ']' in host`) — but net.JoinHostPort, which every builder here ends
// in, BRACKETS any host containing a colon and thereby SUPPLIES the very "]"
// whose absence was doing the rejecting. url.Parse then locates the literal
// with strings.LastIndex(host, "[") (url.go:550), so everything before the
// LAST "[" is silently DISCARDED rather than refused:
//
//	host "[::1"                 -> "http://[[::1]:6379"                 hostname "::1"
//	host "db.prod.internal[::1" -> "http://[db.prod.internal%5B::1"...  hostname "::1"
//
// The second row is the severe one and is the reason this is a correctness fix
// rather than tidying: the operator NAMED "db.prod.internal", and the address
// that came back points at loopback with the name thrown away. That is the
// invented destination this package exists to refuse, reached from an ordinary
// typo — a configured host with one bracket where two were meant.
//
// With "[" and "]" encoded, every one of those inputs now REFUSES, and the
// operator's bytes are still quoted back verbatim in the error.
//
// Interior/unpaired only — the well-formed literal is never touched. This
// function runs SECOND inside urlSafeHost, after urlEncodeZone, which returns
// stripBracket()'s output on every one of its branches. So a correctly
// bracketed "[::1]" has already had its one legitimate pair REMOVED by the
// time this function sees it, and arrives here as "::1" with no bracket to
// encode. Any bracket still present at this point is therefore unpaired or
// interior — malformed by construction. That ordering is load-bearing; see
// urlSafeHost, which owns it.
//
// No legitimate host is affected: hostnames, IPv4 literals, IPv6 literals
// (bare, or bracketed and already unwrapped upstream) and RFC 6874 zone IDs
// contain none of these six bytes at this point in the pipeline.
//
// The remaining self-loud bytes (space, "\\", "^", "`", "{", "|", "}") stay
// untouched, on the original and still-correct reasoning: they are rejected on
// their own merits and encoding them would swap one honest parse error for
// another. Unlike "[", none of them is manufactured a partner by JoinHostPort.
func encodeGenDelims(host string) string {
	// Fast path: the overwhelmingly common case is a host with none of them,
	// which must come back byte-identical.
	if !strings.ContainsAny(host, "/?#@[]") {
		return host
	}
	return genDelimEscaper.Replace(host)
}

// genDelimEscaper is package-level so the replacer is built once rather than
// per call. strings.Replacer is documented safe for concurrent use.
var genDelimEscaper = strings.NewReplacer(
	"/", "%2F",
	"?", "%3F",
	"#", "%23",
	"@", "%40",
	// "[" and "]" reach here only when unpaired or interior — urlEncodeZone
	// has already removed the one legitimate pair. See encodeGenDelims's
	// `"[" was WRONGLY classified as self-loud` section.
	"[", "%5B",
	"]", "%5D",
)

// urlEncodeZone rewrites host so any IPv6 zone identifier it carries is
// percent-encoded per RFC 6874, splitting at the LAST '%' to agree with the
// Go stdlib's own zone-splitting convention (net/ipsock.go splitHostZone) —
// see the package doc. A host with no '%' is returned unchanged, apart from
// the stripBracket call below.
//
// F5 of the round-5 review (2026-08-13) — that stripBracket call is REDUNDANT,
// and an earlier revision of this sentence over-claimed it. It read "(via
// stripBracket, so whitespace/brackets are still normalised — see MUST-FIX 2)",
// which reads as though THIS function is where that normalisation happens for
// the URL path. It is not: every caller of urlEncodeZone reaches
// DialAddress/DialAddressString downstream, and BOTH trim and unbracket again,
// so the normalisation would happen with or without this call. Measured:
// deleting stripBracket from this function changed 0 of 21 corpus inputs
// through the full BaseURL pipeline and left every test green (mutation M18 of
// the 2026-08-13 sweep — see the provenance note in the package doc for what
// that sweep does and does not establish).
//
// It is KEPT rather than removed, and the reason is narrower than a first
// draft of this paragraph claimed. That draft asserted a bracketed
// zone-qualified host "[fe80::1%eth0]" would otherwise have its trailing ']'
// swept into the zone and re-emitted as "%25eth0]". MEASURED, that is FALSE:
// with and without the call, "[fe80::1%eth0]" composes to the IDENTICAL
// "http://[fe80::1%25eth0]:8100", because DialAddress unbrackets again
// downstream. The claim is retracted here rather than quietly dropped — it is
// the same false-self-description class this whole item exists to remove.
//
// What IS observable, found while checking that claim, is one input the M18
// corpus did not contain — the BRACKETED spelling of the divergence input the
// package doc's zone section already discusses:
//
//	host "[fe80::1%eth0%25]"
//	  WITH stripBracket    -> "http://[fe80::1%eth0%2525]:8100"
//	  WITHOUT (M18)        -> "http://[fe80::1%eth0%25]:8100"
//
// The trailing ']' is swept into the zone, which flips the already-encoded
// branch ("25]" keeps a non-empty remainder after TrimPrefix, "25" does not),
// so the two spellings take different branches. HONEST BOUNDARY (§11.4.6):
// BOTH results REFUSE to parse, with the same error (`invalid URL escape
// "%et"`), so this is a difference in the composed STRING and NOT a
// demonstrated difference in user-visible behaviour. It does not rescue the
// call as load-bearing; it only shows the call is not literally unobservable.
//
// So the honest summary: the call keeps urlEncodeZone's OWN output well-formed
// independently of what its callers happen to do afterwards, which is a
// contract property worth having even though today's callers make it
// redundant. It is pinned by TestUrlEncodeZone_StripBracketPrecedesZoneScan
// so a future edit that drops it is a visible decision rather than a silent
// one — that test asserts the string, and claims nothing beyond it.
//
// The result is always UNBRACKETED: callers (BaseURL via
// DialAddressForURL, BaseURLString via DialAddressString) re-bracket
// downstream through DialAddress/DialAddressString, which is the ONLY place
// bracket-wrapping happens in this package now — re-bracketing here too
// would require this function to ALSO know where the true host ends before
// stripBracket runs, which was exactly the bug MUST-FIX 1 exploited (see
// below).
//
// MUST-FIX 1 (2026-08-12 review): an earlier revision decided "already
// encoded" with a bare strings.HasPrefix(zone, "25") check. That misreads a
// GENUINE numeric zone "25" ("fe80::1%25", meaning interface index 25) as
// an already-RFC-6874-encoded delimiter and leaves it unescaped —
// url.Parse then decodes the lone "%25" back to a bare '%' with nothing
// after it and fails outright ("zone must be a non-empty string").
// Restores the exact guard helixendpoint.normalizeHost already uses for
// this ("rest != \"\"" after `strings.TrimPrefix(zone, "25")`), adapted to
// this function's encode direction: the zone counts as already-encoded
// ONLY when stripping its "25" prefix leaves something behind (a zone that
// IS exactly "25" has nothing left once stripped, so it is raw and MUST
// still be encoded, to "%2525").
//
// Residual ambiguity, documented rather than resolved (helixendpoint carries
// the identical ambiguity and resolves it the same way, per its own comment):
// a raw numeric zone whose digits BEGIN with "25" is read as the
// already-encoded delimiter followed by whatever remains.
//
// MUST-FIX 2 of a second 2026-08-12 review: an earlier revision of this
// paragraph called that "a caller that means the latter has no way to spell
// it", which UNDERSTATES the consequence and is the more dangerous half of
// the two. It does not fail to express the address — it SILENTLY EXPRESSES A
// DIFFERENT ONE. Measured against this repository's Go toolchain in the
// session that corrected it, round-tripping BaseURL output through url.Parse:
//
//	"fe80::1%250" -> "http://[fe80::1%250]:7061"  hostname "fe80::1%0"  (zone 250 -> 0)
//	"fe80::1%251" -> "http://[fe80::1%251]:7061"  hostname "fe80::1%1"  (zone 251 -> 1)
//	"fe80::1%255" -> "http://[fe80::1%255]:7061"  hostname "fe80::1%5"  (zone 255 -> 5)
//
// A zone identifier names an INTERFACE, so this addresses a different
// interface than the operator configured — quietly, with no parse error and
// nothing downstream able to notice. Only the exact zone "25" was previously
// covered by a test (the MUST-FIX 1 guard below); the truncating neighbours
// were not.
//
// Left as-is DELIBERATELY, not overlooked. The same session measured that
// helixendpoint.normalizeHost produces BYTE-IDENTICAL output for all three
// (its `strings.TrimPrefix(zone, "25")` guard takes the same branch), so a
// unilateral "fix" here would CREATE a divergence between the two builders in
// this family — the exact absorption-class defect the package doc's
// default-substitution section is about, and a worse outcome than a shared,
// documented, test-pinned limitation.
//
// An earlier revision closed that thought with "changing it is an upstream
// change to helixendpoint that both must land together", which implies a
// deferred FIX exists and is merely waiting on coordination. Measured, it does
// not — the input is INHERENTLY AMBIGUOUS, and no encoder can resolve it:
// "fe80::1%25eth0" is, from the string alone, both the RFC 6874 encoding of
// zone "eth0" and the raw spelling of a zone literally named "25eth0". Only
// THREE rules are available, and all three are lossy:
//
//   - Treat "25…" as already-encoded (what both builders do). Idempotent —
//     re-encoding its own output is a fixed point — at the cost of truncating
//     a raw zone that begins "25".
//   - Always encode. Never truncates, but is NOT idempotent and grows without
//     bound on re-encode; measured this session, feeding the output back in
//     gives "%eth0" -> "%25eth0" -> "%2525eth0" -> "%252525eth0", so the
//     already-encoded input a conformant caller supplies is mangled.
//   - REFUSE — the option this package reaches for everywhere else (the
//     gen-delim guard, and the "fe80::1%eth0%25" case where it deliberately
//     diverges from helixendpoint BECAUSE refusing is safer). Not taken here,
//     and this is the reason the earlier revision never stated: refusing every
//     zone beginning "25" would refuse "fe80::1%25eth0" — the ordinary,
//     correct, RFC 6874 form. That is the overwhelmingly common input, and
//     TestBaseURL_AlreadyEncodedZoneIsNotDoubleEncoded pins that it must round
//     -trip. Refusal would reject the common case to protect a rare one.
//
// So this is a CONTRACT-level limitation, not a pending patch: fixing it needs
// a spelling that distinguishes the two intents (a separate zone parameter, or
// a documented escape), agreed across BOTH builders — not a smarter heuristic
// in this function. Reachability of the bad case: the zone must literally
// BEGIN "25", i.e. a numeric interface index >= 250, or an interface NAMED
// with that prefix. Measured 2026-08-13 through the full BaseURL pipeline:
//
//	host "fe80::1%25gbe0" -> "http://[fe80::1%25gbe0]:8100"
//	                         url.Hostname() = "fe80::1%gbe0", i.e. ZONE "gbe0"
//
// An earlier revision wrote this as `"25gbe0" -> hostname "gbe0"` (round-8
// review finding 8). That conflated two things: "gbe0" is the resulting ZONE,
// the truncated form of the caller's "25gbe0"; the HOSTNAME is the whole
// "fe80::1%gbe0". The truncation being described is real either way — only
// the label was wrong. Adjacent indices are unaffected ("24" -> "%24",
// "26" -> "%26", "3" -> "%3" all survive; re-measured 2026-08-13).
//
// FOLLOW-UP: HXC-327 (allocated 2026-08-13; M2 of the round-5 review).
//
// An earlier revision said "NOT YET FILED", on the reasoning that minting an id
// in a comment would be a dangling reference (§11.4.6). Refusing to FABRICATE
// an id was right; it was not an argument against ALLOCATING a real one, and
// leaving a contract-level limitation with a named remediation scope and a
// pinning test discoverable only by reading this file is the §11.4.197
// un-wired-in-the-backlog failure. HXC-327 is now allocated for the IPv6 zone
// "%25"-prefix truncation described above.
//
// Scope for HXC-327: introduce an unambiguous zone-spelling across
// netaddr + helixendpoint together, and only then retire the truncation.
// TestUrlEncodeZone_NumericZoneBeginning25_TruncatesAndAgreesWithHelixEndpoint
// pins BOTH facts: the truncation, and the cross-builder agreement that makes
// it safe to leave. A future unilateral change here breaks that test.
func urlEncodeZone(host string) string {
	trimmed := stripBracket(host)

	i := strings.LastIndex(trimmed, "%")
	if i < 0 {
		return trimmed
	}

	bare, zone := trimmed[:i], trimmed[i+1:]
	alreadyEncoded := strings.HasPrefix(zone, "25") && strings.TrimPrefix(zone, "25") != ""
	if alreadyEncoded {
		return trimmed
	}
	return bare + "%25" + zone
}
