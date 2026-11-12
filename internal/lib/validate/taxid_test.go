package validate

import "testing"

func TestTaxId(t *testing.T) {
	tests := []struct {
		name    string
		country string
		id      string
		ok      bool
	}{
		{"dni", "ES", "12345678Z", true},
		{"dni wrong letter", "ES", "12345678A", false},
		{"dni short", "ES", "1234567Z", false},
		{"nie x", "ES", "X1234567L", true},
		{"nie y", "ES", "Y1234567X", true},
		{"nie wrong letter", "ES", "X1234567A", false},
		{"cif digit control", "ES", "A58818501", true},
		{"cif digit control b", "ES", "B12345674", true},
		{"cif wrong digit", "ES", "A58818502", false},
		{"cif letter control", "ES", "Q2826000H", true},
		{"cif letter required, digit given", "ES", "Q28260008", false},
		{"cif digit required, letter given", "ES", "A5881850A", false},
		{"cif wrong letter", "ES", "Q2826000J", false},
		{"unknown first letter", "ES", "T58818501", false},
		{"empty", "ES", "", false},
		{"foreign vat", "FR", "FR12345678901", true},
		{"foreign portuguese", "PT", "123456789", true},
		{"foreign too short", "FR", "AB1", false},
		{"foreign with symbols", "DE", "DE12*45678", false},
		{"foreign empty", "DE", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TaxId(tt.country, tt.id)
			if (err == nil) != tt.ok {
				t.Errorf("TaxId(%q, %q) = %v, want ok=%v", tt.country, tt.id, err, tt.ok)
			}
		})
	}
}

func TestNormalizeTaxId(t *testing.T) {
	tests := map[string]string{
		"12345678z":     "12345678Z",
		" x-1234567.l ": "X1234567L",
		"A 58 818 501":  "A58818501",
		"":              "",
	}
	for in, want := range tests {
		if got := NormalizeTaxId(in); got != want {
			t.Errorf("NormalizeTaxId(%q) = %q, want %q", in, got, want)
		}
	}
}
