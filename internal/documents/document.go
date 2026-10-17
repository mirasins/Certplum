package documents

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Input struct {
	Kind      string `json:"kind"`
	Patient   string `json:"patient"`
	RUT       string `json:"rut"`
	Date      string `json:"date"`
	Content   string `json:"content"`
	Diagnosis string `json:"diagnosis,omitempty"`
	Folio     string `json:"folio,omitempty"` // Assigned by the server; never taken from the form.
}

func (v Input) Validate() error {
	if v.Kind != "certificado" && v.Kind != "evolucion" && v.Kind != "epicrisis" {
		return errors.New("Selecciona un tipo de documento válido.")
	}
	if strings.TrimSpace(v.Patient) == "" || utf8.RuneCountInString(v.Patient) > 180 {
		return errors.New("Ingresa un nombre de hasta 180 caracteres.")
	}
	if !ValidRUT(v.RUT) {
		return errors.New("El RUT no es válido.")
	}
	if _, e := time.Parse("2006-01-02", v.Date); e != nil {
		return errors.New("La fecha no es válida.")
	}
	if strings.TrimSpace(v.Content) == "" || utf8.RuneCountInString(v.Content) > 30000 || utf8.RuneCountInString(v.Diagnosis) > 1000 {
		return errors.New("Revisa el contenido y su extensión máxima.")
	}
	for _, s := range []string{v.Patient, v.RUT, v.Content, v.Diagnosis} {
		for _, r := range s {
			if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
				return errors.New("El texto contiene caracteres de control.")
			}
		}
	}
	return nil
}
func ValidRUT(s string) bool {
	s = strings.ToUpper(strings.NewReplacer(".", "", "-", "", " ", "").Replace(s))
	if len(s) < 8 || len(s) > 9 {
		return false
	}
	sum, m := 0, 2
	for i := len(s) - 2; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		sum += int(s[i]-'0') * m
		m++
		if m > 7 {
			m = 2
		}
	}
	n := 11 - sum%11
	dv := byte('0' + n)
	if n == 11 {
		dv = '0'
	}
	if n == 10 {
		dv = 'K'
	}
	return s[len(s)-1] == dv
}
