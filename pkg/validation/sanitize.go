package validation

import (
	"strings"
	"unicode"
)

// sqlReplacer strips SQL injection metacharacters. Constructed once; safe for
// concurrent use.
var sqlReplacer = strings.NewReplacer(
	"--", "", // line comment
	"/*", "", // block comment open
	"*/", "", // block comment close
	";",  "", // statement terminator
	"\\", "", // escape char (MySQL / some drivers)
)

// isASCIIDigit reports whether r is an ASCII decimal digit (0–9).
func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// truncateRunes truncates s to at most maxLen runes. No-op when maxLen <= 0.
func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

// SanitizeString trims leading/trailing whitespace, removes control characters,
// and truncates the result to maxLen runes. If maxLen <= 0 no truncation is applied.
func SanitizeString(s string, maxLen int) string {
	s = strings.TrimSpace(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	return truncateRunes(b.String(), maxLen)
}

// SanitizeSQL is a defence-in-depth wrapper around SanitizeString that also
// neutralises the most common SQL-injection metacharacters.
//
// IMPORTANT: parameterised queries / prepared statements are the PRIMARY
// defence against SQL injection and must always be used where possible.
// Use this function only as an additional layer for values that flow into
// dynamic SQL that cannot use bind parameters (e.g. identifiers, LIKE
// patterns).
//
// What it does on top of SanitizeString:
//   - Escapes single quotes  '  →  ''  (SQL-standard string escaping)
//   - Removes line-comment markers  --
//   - Removes block-comment markers  /*  */
//   - Removes statement terminators  ;
//   - Removes backslash escape chars  \  (used by MySQL and some drivers)
func SanitizeSQL(s string, maxLen int) string {
	s = SanitizeString(s, 0) // truncation applied last, after all transformations

	// Escape single-quote first so the replacements below don't interact.
	s = strings.ReplaceAll(s, "'", "''")

	// Truncate after all transformations so quote-escaping cannot push the
	// result beyond maxLen.
	return truncateRunes(sqlReplacer.Replace(s), maxLen)
}
