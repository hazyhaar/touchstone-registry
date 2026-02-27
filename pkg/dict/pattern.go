// CLAUDE:SUMMARY Regex pattern matcher with checksum validators (IBAN mod97, Luhn, French NIR) for pattern-based dictionaries.
package dict

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// compiledPattern is a single named regex with an optional checksum validator.
type compiledPattern struct {
	name      string
	re        *regexp.Regexp
	validator func(string) bool
}

// patternMatcher holds compiled patterns for a pattern-based dictionary.
type patternMatcher struct {
	patterns []compiledPattern
}

// compilePatterns builds a patternMatcher from manifest pattern specs.
func compilePatterns(specs []PatternSpec) (*patternMatcher, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no patterns defined")
	}

	pm := &patternMatcher{patterns: make([]compiledPattern, 0, len(specs))}
	for _, spec := range specs {
		re, err := regexp.Compile(spec.Regex)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", spec.Name, err)
		}
		cp := compiledPattern{name: spec.Name, re: re}
		switch spec.Validator {
		case "":
			// No validator.
		case "mod97":
			cp.validator = validateMod97
		case "luhn":
			cp.validator = validateLuhn
		case "nir":
			cp.validator = validateNIR
		default:
			return nil, fmt.Errorf("pattern %q: unknown validator %q", spec.Name, spec.Validator)
		}
		pm.patterns = append(pm.patterns, cp)
	}
	return pm, nil
}

// match tests a term against all patterns. Returns the first matching pattern name.
func (pm *patternMatcher) match(term string) (string, bool) {
	cleaned := strings.ReplaceAll(term, " ", "")
	for _, p := range pm.patterns {
		if !p.re.MatchString(cleaned) {
			continue
		}
		if p.validator != nil && !p.validator(cleaned) {
			continue
		}
		return p.name, true
	}
	return "", false
}

// validateMod97 implements ISO 7064 MOD 97-10 (used by IBAN).
// The input must be an alphanumeric IBAN string (spaces already stripped).
func validateMod97(s string) bool {
	if len(s) < 5 {
		return false
	}
	// Move first 4 characters to the end.
	rearranged := s[4:] + s[:4]

	// Convert letters to digits: A=10, B=11, ..., Z=35.
	var digits strings.Builder
	for _, c := range strings.ToUpper(rearranged) {
		if c >= '0' && c <= '9' {
			digits.WriteRune(c)
		} else if c >= 'A' && c <= 'Z' {
			fmt.Fprintf(&digits, "%d", c-'A'+10)
		} else {
			return false
		}
	}

	n := new(big.Int)
	if _, ok := n.SetString(digits.String(), 10); !ok {
		return false
	}
	mod := new(big.Int).Mod(n, big.NewInt(97))
	return mod.Int64() == 1
}

// validateLuhn implements the Luhn algorithm (used by credit card numbers).
func validateLuhn(s string) bool {
	if len(s) == 0 {
		return false
	}
	var sum int
	nDigits := len(s)
	parity := nDigits % 2

	for i := 0; i < nDigits; i++ {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// validateNIR validates a French NIR (numéro de sécurité sociale).
// Format: 15 digits where the last 2 are a MOD 97 check key.
// Handles Corsican departments: 2A → 19, 2B → 20.
func validateNIR(s string) bool {
	if len(s) != 15 {
		return false
	}

	// Extract the check key (last 2 digits).
	keyStr := s[13:15]
	var key int64
	for _, c := range keyStr {
		if c < '0' || c > '9' {
			return false
		}
		key = key*10 + int64(c-'0')
	}

	// The 13-digit body, with Corsican department substitution.
	body := s[:13]
	body = strings.Replace(body, "2A", "19", 1)
	body = strings.Replace(body, "2B", "20", 1)

	var bodyNum int64
	for _, c := range body {
		if c < '0' || c > '9' {
			return false
		}
		bodyNum = bodyNum*10 + int64(c-'0')
	}

	// Check: key = 97 - (body mod 97)
	return key == 97-bodyNum%97
}
