package validate

import (
	"errors"
	"regexp"
	"strings"
)

const dniLetters = "TRWAGMYFPDXBNJZSQVHLCKE"

var (
	dniRe     = regexp.MustCompile(`^[0-9]{8}[A-Z]$`)
	nieRe     = regexp.MustCompile(`^[XYZ][0-9]{7}[A-Z]$`)
	cifRe     = regexp.MustCompile(`^[ABCDEFGHJNPQRSUVW][0-9]{7}[0-9A-J]$`)
	specialRe = regexp.MustCompile(`^[KLM][0-9]{7}[0-9A-Z]$`)
	foreignRe = regexp.MustCompile(`^[A-Z0-9]{4,20}$`)
)

// NormalizeTaxId upper-cases a tax id and drops spaces, dots and dashes.
func NormalizeTaxId(s string) string {
	s = strings.ToUpper(s)
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '\t':
			return -1
		}
		return r
	}, s)
}

// TaxId checks a normalized tax id for a country (ISO 3166-1 alpha-2). For
// Spain the check digit is verified (DNI, NIE, CIF and the K/L/M forms). For
// other countries only the shape is checked: the formats differ too much, and
// a wrong one is caught by the invoicing service.
func TaxId(country, id string) error {
	if id == "" {
		return errors.New("is required")
	}
	if country != "ES" {
		if !foreignRe.MatchString(id) {
			return errors.New("must be 4-20 letters or digits")
		}
		return nil
	}
	if !validSpanishId(id) {
		return errors.New("is not a valid NIF, NIE or CIF")
	}
	return nil
}

func validSpanishId(id string) bool {
	switch {
	case dniRe.MatchString(id):
		return dniLetter(id[:8]) == id[8]
	case nieRe.MatchString(id):
		digits := strings.NewReplacer("X", "0", "Y", "1", "Z", "2").Replace(id[:1]) + id[1:8]
		return dniLetter(digits) == id[8]
	case cifRe.MatchString(id), specialRe.MatchString(id):
		return validCif(id)
	}
	return false
}

func dniLetter(digits string) byte {
	n := 0
	for _, c := range digits {
		n = n*10 + int(c-'0')
	}
	return dniLetters[n%23]
}

// validCif checks the control character of a legal-entity id. The control is
// a digit for A, B, E, H, a letter for K, L, M, N, P, Q, R, S, W, either for
// the others.
func validCif(id string) bool {
	sum := 0
	for i := 1; i <= 7; i++ {
		d := int(id[i] - '0')
		if i%2 == 1 {
			d *= 2
			d = d/10 + d%10
		}
		sum += d
	}
	digit := (10 - sum%10) % 10
	letter := "JABCDEFGHI"[digit]
	control := id[8]
	switch id[0] {
	case 'A', 'B', 'E', 'H':
		return control == byte('0'+digit)
	case 'K', 'L', 'M', 'N', 'P', 'Q', 'R', 'S', 'W':
		return control == letter
	}
	return control == byte('0'+digit) || control == letter
}
