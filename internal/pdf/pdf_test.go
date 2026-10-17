package pdf_test

import (
	"bytes"
	pdflib "github.com/digitorus/pdf"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"os"
	"strings"
	"testing"
)

func TestMultipageAndGlyphs(t *testing.T) {
	in := testutil.Document()
	in.Kind = "evolucion"
	in.Folio = "EVO-2026-0001"
	in.Diagnosis = "Diagnóstico ficticio de prueba, sin datos reales."
	in.Content = strings.Repeat("Evolución de prueba: ánimo, atención y vínculo terapéutico. Se acuerda revisar objetivos en la próxima sesión.\n", 90)
	b, e := pdfgen.Generate(in, pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"})
	if e != nil {
		t.Fatal(e)
	}
	r, e := pdflib.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil || r.NumPage() < 3 {
		t.Fatal("long document was not paginated")
	}
	if path := os.Getenv("CLINICAL_TEST_LONG_PDF"); path != "" {
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	in.Content = "Texto con carácter no admitido: 🧠"
	if _, e = pdfgen.Generate(in, pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"}); e == nil {
		t.Fatal("missing glyph silently accepted")
	}
}
