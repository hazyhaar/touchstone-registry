package dict

import "testing"

func TestNormalizeLowercaseASCII(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"DUPONT", "dupont"},
		{"Élodie", "elodie"},
		{"café", "cafe"},
		{"naïve", "naive"},
		{"FRANÇOIS", "francois"},
		{"Ñoño", "nono"},
		{"", ""},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		got := NormalizeLowercaseASCII(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLowercaseASCII(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeLowercaseUTF8(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"DUPONT", "dupont"},
		{"Élodie", "élodie"},
		{"CAFÉ", "café"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeLowercaseUTF8(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLowercaseUTF8(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeNone(t *testing.T) {
	for _, input := range []string{"DUPONT", "Élodie", "café", ""} {
		got := NormalizeNone(input)
		if got != input {
			t.Errorf("NormalizeNone(%q) = %q, want unchanged", input, got)
		}
	}
}

func TestGetNormalizer(t *testing.T) {
	tests := []struct {
		mode  string
		input string
		want  string
	}{
		{"lowercase_ascii", "Élodie", "elodie"},
		{"lowercase_utf8", "Élodie", "élodie"},
		{"none", "Élodie", "Élodie"},
		{"", "Élodie", "elodie"},             // default = lowercase_ascii
		{"unknown_mode", "Élodie", "elodie"}, // fallback = lowercase_ascii
	}
	for _, tt := range tests {
		fn := GetNormalizer(tt.mode)
		got := fn(tt.input)
		if got != tt.want {
			t.Errorf("GetNormalizer(%q)(%q) = %q, want %q", tt.mode, tt.input, got, tt.want)
		}
	}
}
