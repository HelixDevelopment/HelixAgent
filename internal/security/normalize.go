package security

import (
	"encoding/base64"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizedInput wraps a prompt with its decoded / normalised variants.
// Every text-matching detector in StandardGuardrailPipeline receives both
// the original and the Variants slice; a match on ANY variant counts as
// a trigger. This closes filter-bypass / stego-mutation gaps where
// attackers use base64, leet-speak, homoglyphs, ROT13, zero-width
// characters, whitespace channels, or reversed strings to smuggle
// directives past literal regex / keyword lists.
//
// Variants always contains the Original as its first element so existing
// callers that iterate can treat the slice uniformly.
type NormalizedInput struct {
	Original string
	Variants []string
}

// Normalize produces the canonical variants used by text detectors.
// Each variant is a different decoding / folding of the same prompt;
// they are unique (duplicates collapsed) and the original is always
// included. Cost is O(len(s)) for each transform.
func Normalize(s string) NormalizedInput {
	out := NormalizedInput{Original: s}
	seen := make(map[string]struct{}, 10)
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out.Variants = append(out.Variants, v)
	}

	add(s)

	// 1. NFKC Unicode normalization (folds fullwidth/compat forms).
	add(norm.NFKC.String(s))

	// 2. Strip zero-width characters (ZWJ/ZWSP/ZWNJ/BOM/word-joiner).
	stripped := stripZeroWidth(s)
	add(stripped)

	// 2b. Strip + NFKC combined — covers fullwidth + zero-width in one pass.
	add(norm.NFKC.String(stripped))

	// 3. Leet-speak de-leet.
	add(deleetSpeak(s))

	// 4. Homoglyph fold (Cyrillic/Greek look-alikes → ASCII).
	add(foldHomoglyphs(s))

	// 5. ROT13 round-trip.
	add(rot13(s))

	// 6. Base64 decode (best-effort; returns "" on non-base64 input).
	add(tryDecodeBase64(s))

	// 7. Whitespace collapse + de-hyphenate character-split attacks.
	add(collapseWhitespace(s))
	add(collapseCharacterSplit(s))

	// 8. Reverse string (catches reversed-prompt attack).
	add(reverseString(s))

	// 9. Combined: homoglyph fold + NFKC + zero-width strip — the most
	//    aggressive canonicalisation for stego attacks that combine
	//    techniques. Also lowercased for case-insensitive detectors.
	combined := foldHomoglyphs(norm.NFKC.String(stripped))
	add(combined)
	add(strings.ToLower(combined))

	return out
}

// AnyMatch returns true iff fn returns true for any variant.
// Convenience helper so detectors can scan all variants in one call.
func (n NormalizedInput) AnyMatch(fn func(string) bool) bool {
	for _, v := range n.Variants {
		if fn(v) {
			return true
		}
	}
	return false
}

// stripZeroWidth removes characters commonly used for steganographic
// insertion: ZWJ (U+200D), ZWSP (U+200B), ZWNJ (U+200C), BOM (U+FEFF),
// and the word joiner (U+2060).
func stripZeroWidth(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF:
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// leetMap is a conservative de-leet table. Only characters that change
// meaning in a substitution-attack sense (digits / punctuation that
// imitate letters) are mapped; plain letters are preserved.
var leetMap = map[rune]rune{
	'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't',
	'@': 'a', '!': 'i', '$': 's',
}

// deleetSpeak returns a lowercased de-leet of s. Runes not in the leet
// map are lowercased via unicode.ToLower; this means a detector's own
// case-insensitive matching still works on the result.
func deleetSpeak(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if v, ok := leetMap[r]; ok {
			b.WriteRune(v)
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// homoglyphMap folds common Cyrillic and Greek look-alikes to their
// ASCII cognates. This is NOT a complete homoglyph table — it covers
// only the characters actually used in documented filter-bypass
// attacks (Ignоre, Dіsregard, …). A full confusables mapping is far
// larger and would risk folding legitimate foreign-language text.
var homoglyphMap = map[rune]rune{
	// Cyrillic (lowercase)
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'х': 'x', 'у': 'y',
	'і': 'i', 'ј': 'j',
	// Cyrillic (uppercase)
	'А': 'A', 'Е': 'E', 'О': 'O', 'Р': 'P', 'С': 'C', 'Х': 'X', 'У': 'Y',
	'І': 'I', 'Ј': 'J',
	// Greek (capitals only; lowercase Greek rarely substitutes for Latin)
	'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Κ': 'K', 'Μ': 'M', 'Ν': 'N',
	'Ο': 'O', 'Ρ': 'P', 'Τ': 'T', 'Υ': 'Y', 'Χ': 'X', 'Ζ': 'Z',
}

// foldHomoglyphs returns s with every mapped rune replaced by its
// ASCII cognate. Unmapped runes are preserved verbatim.
func foldHomoglyphs(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if v, ok := homoglyphMap[r]; ok {
			b.WriteRune(v)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// rot13 applies a ROT13 substitution to ASCII letters, leaving all
// other runes unchanged. The function is self-inverse, so running
// it against ROT13-encoded input decodes it.
func rot13(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune('A' + (r-'A'+13)%26)
		case r >= 'a' && r <= 'z':
			b.WriteRune('a' + (r-'a'+13)%26)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// b64RE matches plausible base64 payloads: a run of at least 16 base64
// alphabet characters optionally followed by up to two '=' pad chars.
// Shorter runs are almost always false positives (identifiers, hex, …).
var b64RE = regexp.MustCompile(`[A-Za-z0-9+/]{16,}={0,2}`)

// tryDecodeBase64 attempts to locate base64-encoded substrings inside s
// and decode them to printable text. Returns "" when no plausible
// base64 payload is present OR every match decodes to something that
// doesn't look like natural language. Multiple hits are joined with
// spaces so downstream detectors scan the aggregate.
func tryDecodeBase64(s string) string {
	matches := b64RE.FindAllString(s, -1)
	if len(matches) == 0 {
		return ""
	}
	var decoded []string
	for _, m := range matches {
		clean := strings.TrimRight(m, "=")
		payload, err := base64.StdEncoding.DecodeString(padBase64(clean))
		if err != nil {
			continue
		}
		text := string(payload)
		if !isPlausibleText(text) {
			continue
		}
		decoded = append(decoded, text)
	}
	if len(decoded) == 0 {
		return ""
	}
	return strings.Join(decoded, " ")
}

func padBase64(s string) string {
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	return s
}

// isPlausibleText is a cheap heuristic: at least 4 bytes, at least
// 80% of runes are printable or whitespace. Keeps us from alarming
// on random binary that happened to match the base64 alphabet.
func isPlausibleText(s string) bool {
	if len(s) < 4 {
		return false
	}
	var printable, total int
	for _, r := range s {
		total++
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printable++
		}
	}
	if total == 0 {
		return false
	}
	return float64(printable)/float64(total) > 0.8
}

// collapseWhitespaceRE collapses any run of whitespace characters into
// a single ASCII space. Package-level so the compile happens once.
var collapseWhitespaceRE = regexp.MustCompile(`\s+`)

func collapseWhitespace(s string) string {
	return collapseWhitespaceRE.ReplaceAllString(s, " ")
}

// collapseCharacterSplit reverses "I-g-n-o-r-e" style splits and
// "I g n o r e" style whitespace-between-characters attacks. It
// detects such patterns by removing single non-alphanumeric runes
// (hyphens, spaces, dots, zero-width chars) that sit BETWEEN two
// alphanumeric runes. The output is then re-examined by detectors.
func collapseCharacterSplit(s string) string {
	runes := []rune(s)
	if len(runes) < 3 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		if i == 0 || i == len(runes)-1 {
			b.WriteRune(r)
			continue
		}
		prev := runes[i-1]
		next := runes[i+1]
		// Drop a single separator sandwiched between two letters/digits.
		if isAlphaNum(prev) && isAlphaNum(next) && isSeparator(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isAlphaNum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isSeparator is true for characters commonly inserted between letters
// in a split-attack: hyphen, space, tab, dot, underscore, zero-width.
func isSeparator(r rune) bool {
	switch r {
	case '-', ' ', '\t', '.', '_', 0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF:
		return true
	}
	return false
}

// reverseString reverses a string by runes (handles UTF-8 correctly).
func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
