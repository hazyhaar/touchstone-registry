package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateMod97(t *testing.T) {
	valid := []string{
		"FR7630006000011234567890189",
		"DE89370400440532013000",
		"GB29NWBK60161331926819",
		"ES9121000418450200051332",
		"IT60X0542811101000000123456",
		"BE68539007547034",
		"NL91ABNA0417164300",
		"CH9300762011623852957",
		"LU280019400644750000",
		"PT50000201231234567890154",
	}
	for _, iban := range valid {
		if !validateMod97(iban) {
			t.Errorf("validateMod97(%q) = false, want true", iban)
		}
	}

	invalid := []string{
		"FR7630006000011234567890180",
		"DE89370400440532013001",
		"GB29NWBK60161331926810",
		"XX00INVALID",
		"FR76",
		"",
	}
	for _, iban := range invalid {
		if validateMod97(iban) {
			t.Errorf("validateMod97(%q) = true, want false", iban)
		}
	}
}

func TestValidateLuhn(t *testing.T) {
	valid := []string{
		"4111111111111111", // Visa
		"5500000000000004", // Mastercard
		"340000000000009",  // Amex
		"79927398713",      // Standard test number
	}
	for _, cc := range valid {
		if !validateLuhn(cc) {
			t.Errorf("validateLuhn(%q) = false, want true", cc)
		}
	}

	invalid := []string{
		"4111111111111112",
		"5500000000000005",
		"1234567890123456",
		"",
	}
	for _, cc := range invalid {
		if validateLuhn(cc) {
			t.Errorf("validateLuhn(%q) = true, want false", cc)
		}
	}
}

func TestValidateNIR(t *testing.T) {
	// Valid NIR: 1 85 05 78 006 084, body=1850578006084 mod 97=6, key=97-6=91
	valid := []string{
		"185057800608491",
	}
	for _, nir := range valid {
		if !validateNIR(nir) {
			t.Errorf("validateNIR(%q) = false, want true", nir)
		}
	}

	invalid := []string{
		"185057800608400",
		"12345678901234",  // too short
		"1234567890123456", // too long
		"",
	}
	for _, nir := range invalid {
		if validateNIR(nir) {
			t.Errorf("validateNIR(%q) = true, want false", nir)
		}
	}
}

func TestPatternMatcher_IBAN(t *testing.T) {
	specs := []PatternSpec{
		{Name: "iban_fr", Regex: `^FR\d{2}\d{10}[A-Z0-9]{11}\d{2}$`, Validator: "mod97"},
	}
	pm, err := compilePatterns(specs)
	if err != nil {
		t.Fatalf("compilePatterns: %v", err)
	}

	name, ok := pm.match("FR7630006000011234567890189")
	if !ok {
		t.Fatal("expected match for valid IBAN FR")
	}
	if name != "iban_fr" {
		t.Errorf("name = %q, want iban_fr", name)
	}

	// Invalid checksum
	_, ok = pm.match("FR7630006000011234567890180")
	if ok {
		t.Error("expected no match for invalid IBAN checksum")
	}

	// Wrong country
	_, ok = pm.match("DE89370400440532013000")
	if ok {
		t.Error("expected no match for DE IBAN against FR pattern")
	}
}

func TestPatternMatcher_CreditCard(t *testing.T) {
	specs := []PatternSpec{
		{Name: "visa", Regex: `^4\d{12}(\d{3})?$`, Validator: "luhn"},
		{Name: "mastercard", Regex: `^5[1-5]\d{14}$`, Validator: "luhn"},
		{Name: "amex", Regex: `^3[47]\d{13}$`, Validator: "luhn"},
	}
	pm, err := compilePatterns(specs)
	if err != nil {
		t.Fatalf("compilePatterns: %v", err)
	}

	tests := []struct {
		input string
		name  string
		found bool
	}{
		{"4111111111111111", "visa", true},
		{"5500000000000004", "mastercard", true},
		{"340000000000009", "amex", true},
		{"4111111111111112", "", false}, // bad luhn
		{"9999999999999999", "", false}, // no pattern match
	}
	for _, tt := range tests {
		name, ok := pm.match(tt.input)
		if ok != tt.found {
			t.Errorf("match(%q) found=%v, want %v", tt.input, ok, tt.found)
		}
		if ok && name != tt.name {
			t.Errorf("match(%q) name=%q, want %q", tt.input, name, tt.name)
		}
	}
}

func TestPatternMatcher_Email(t *testing.T) {
	specs := []PatternSpec{
		{Name: "email", Regex: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`},
	}
	pm, err := compilePatterns(specs)
	if err != nil {
		t.Fatalf("compilePatterns: %v", err)
	}

	valid := []string{
		"user@example.com",
		"test.name+tag@domain.org",
		"a@b.co",
	}
	for _, email := range valid {
		_, ok := pm.match(email)
		if !ok {
			t.Errorf("expected match for %q", email)
		}
	}

	invalid := []string{
		"not-an-email",
		"@domain.com",
		"user@",
		"user@.com",
	}
	for _, email := range invalid {
		_, ok := pm.match(email)
		if ok {
			t.Errorf("expected no match for %q", email)
		}
	}
}

func TestPatternMatcher_SpaceStripping(t *testing.T) {
	specs := []PatternSpec{
		{Name: "iban_fr", Regex: `^FR\d{2}\d{10}[A-Z0-9]{11}\d{2}$`, Validator: "mod97"},
	}
	pm, err := compilePatterns(specs)
	if err != nil {
		t.Fatalf("compilePatterns: %v", err)
	}

	// IBAN with spaces — should still match after stripping.
	name, ok := pm.match("FR76 3000 6000 0112 3456 7890 189")
	if !ok {
		t.Fatal("expected match for IBAN with spaces")
	}
	if name != "iban_fr" {
		t.Errorf("name = %q, want iban_fr", name)
	}
}

func TestPatternMatcher_NoPatterns(t *testing.T) {
	_, err := compilePatterns(nil)
	if err == nil {
		t.Error("expected error for empty patterns")
	}
}

func TestPatternMatcher_InvalidRegex(t *testing.T) {
	specs := []PatternSpec{
		{Name: "bad", Regex: `[invalid`},
	}
	_, err := compilePatterns(specs)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestPatternMatcher_UnknownValidator(t *testing.T) {
	specs := []PatternSpec{
		{Name: "test", Regex: `^test$`, Validator: "unknown"},
	}
	_, err := compilePatterns(specs)
	if err == nil {
		t.Error("expected error for unknown validator")
	}
}

func TestDictionary_Classify_Pattern(t *testing.T) {
	dir := t.TempDir()
	dictDir := filepath.Join(dir, "email-test")
	os.MkdirAll(dictDir, 0o755)

	manifest := `id: email-test
version: "1.0"
jurisdiction: intl
entity_type: email
source: regex
method: pattern
patterns:
  - name: email
    regex: "^[a-zA-Z0-9._%+\\-]+@[a-zA-Z0-9.\\-]+\\.[a-zA-Z]{2,}$"
`
	os.WriteFile(filepath.Join(dictDir, "manifest.yaml"), []byte(manifest), 0o644)

	d, err := LoadDictionary(dictDir)
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	entry, ok := d.Classify("user@example.com")
	if !ok {
		t.Fatal("expected Classify match for email")
	}
	if entry.Metadata["pattern"] != "email" {
		t.Errorf("pattern = %q, want email", entry.Metadata["pattern"])
	}

	_, ok = d.Classify("not-an-email")
	if ok {
		t.Error("expected no Classify match for non-email")
	}
}

func TestDictionary_Classify_Lookup(t *testing.T) {
	dir := writeTestDict(t, "classify-lookup", "lowercase_ascii",
		"term;frequency\nDUPONT;1200\n")

	d, err := LoadDictionary(filepath.Join(dir, "classify-lookup"))
	if err != nil {
		t.Fatalf("LoadDictionary: %v", err)
	}

	entry, ok := d.Classify("dupont")
	if !ok {
		t.Fatal("expected Classify match via lookup")
	}
	if entry.Metadata["freq"] != "1200" {
		t.Errorf("freq = %q, want 1200", entry.Metadata["freq"])
	}
}

func TestRegistry_Classify_Pattern(t *testing.T) {
	dir := t.TempDir()

	// Pattern dict
	pd := filepath.Join(dir, "email")
	os.MkdirAll(pd, 0o755)
	os.WriteFile(filepath.Join(pd, "manifest.yaml"), []byte(`id: email
version: "1.0"
jurisdiction: intl
entity_type: email
source: regex
method: pattern
patterns:
  - name: email
    regex: "^[a-zA-Z0-9._%+\\-]+@[a-zA-Z0-9.\\-]+\\.[a-zA-Z]{2,}$"
`), 0o644)

	// Lookup dict
	ld := filepath.Join(dir, "noms-fr")
	os.MkdirAll(ld, 0o755)
	os.WriteFile(filepath.Join(ld, "manifest.yaml"), []byte(`id: noms-fr
version: "1.0"
jurisdiction: fr
entity_type: surname
source: test
data_file: data.csv
format:
  delimiter: ";"
  has_header: true
  key_column: "term"
  normalize: lowercase_ascii
`), 0o644)
	os.WriteFile(filepath.Join(ld, "data.csv"), []byte("term\nDUPONT\n"), 0o644)

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Email should match pattern dict.
	result := reg.Classify("user@example.com", nil)
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].EntityType != "email" {
		t.Errorf("entity_type = %q, want email", result.Matches[0].EntityType)
	}

	// Surname should match lookup dict.
	result = reg.Classify("DUPONT", nil)
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].EntityType != "surname" {
		t.Errorf("entity_type = %q, want surname", result.Matches[0].EntityType)
	}
}
