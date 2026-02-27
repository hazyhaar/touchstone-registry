// CLAUDE:SUMMARY Text normalization strategies (lowercase+strip-accents, lowercase-only, none) for dictionary term matching.
package dict

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Normalizer transforms a term before lookup.
type Normalizer func(string) string

var stripAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// NormalizeLowercaseASCII lowercases and strips accents (e.g. DUPONT, Élodie -> elodie).
func NormalizeLowercaseASCII(s string) string {
	result, _, _ := transform.String(stripAccents, strings.ToLower(s))
	return result
}

// NormalizeLowercaseUTF8 lowercases but preserves accents.
func NormalizeLowercaseUTF8(s string) string {
	return strings.ToLower(s)
}

// NormalizeNone returns the term unchanged.
func NormalizeNone(s string) string {
	return s
}

// GetNormalizer returns the normalizer for the given mode.
// Default is lowercase_ascii.
func GetNormalizer(mode string) Normalizer {
	switch mode {
	case "lowercase_ascii":
		return NormalizeLowercaseASCII
	case "lowercase_utf8":
		return NormalizeLowercaseUTF8
	case "none":
		return NormalizeNone
	default:
		return NormalizeLowercaseASCII
	}
}
