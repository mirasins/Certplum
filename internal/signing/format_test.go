package signing

import (
	"context"
	"github.com/mirasins/Certplum/internal/config"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfiguredFormatsAndImages(t *testing.T) {
	sec := testutil.Credentials(t)
	defer sec.Clear()
	s, e := Parse(sec.Certificate, sec.PrivateKey)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := config.Load("../../configs/config.example.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"certificado", "evolucion", "epicrisis"} {
		t.Run(kind, func(t *testing.T) {
			in := testutil.Document()
			in.Kind = kind
			in.Folio = map[string]string{"certificado": "CERT-2026-0001", "evolucion": "EVO-2026-0001", "epicrisis": "EPI-2026-0001"}[kind]
			in.Content = "MUESTRA CON DATOS FICTICIOS. Certificado de firma de prueba, sin validez para uso clínico.\n\n" + in.Content
			if kind == "evolucion" {
				in.Content += strings.Repeat("\nEvolución ficticia: ánimo, atención y coordinación. Se revisarán los objetivos en la siguiente sesión.", 45)
			}
			if kind == "epicrisis" {
				in.Content += "\n\nMotivo de atención: ejemplo ficticio.\nProceso de atención: ejemplo ficticio.\nEstado al cierre y recomendaciones: ejemplo ficticio."
			}
			now := time.Now()
			s.SignedAt = now
			s.Name = cfg.Document.Professional.Name
			raw, e := pdfgen.Generate(in, pdfgen.Options{Format: cfg.Document, VerifyURL: "https://consulta.example/v/" + strings.Repeat("a", 64), IssuedAt: now, SignedAt: now})
			if e != nil {
				t.Fatal(e)
			}
			signed, e := s.Sign(context.Background(), raw)
			if e != nil {
				t.Fatal(e)
			}
			if dir := os.Getenv("CLINICAL_FORMAT_DIR"); dir != "" {
				if e = os.WriteFile(filepath.Join(dir, kind+"-carta.pdf"), signed, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
	// Exercise optional SVG logo and transparent PNG signature with geometric,
	// clearly fictitious fixtures. These are not the professional's real assets.
	dir := t.TempDir()
	logo := filepath.Join(dir, "logo.svg")
	if e = os.WriteFile(logo, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect x="5" y="5" width="90" height="40" fill="#24665e"/><circle cx="50" cy="25" r="13" fill="white"/></svg>`), 0600); e != nil {
		t.Fatal(e)
	}
	hand := filepath.Join(dir, "geometric-fixture.png")
	canvas := image.NewNRGBA(image.Rect(0, 0, 200, 60))
	for x := 10; x < 190; x++ {
		canvas.Set(x, 30, color.NRGBA{R: 25, G: 60, B: 60, A: 200})
	}
	f, e := os.Create(hand)
	if e != nil {
		t.Fatal(e)
	}
	e = png.Encode(f, canvas)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	form := testutil.Format()
	form.LogoPath = logo
	form.SignatureImagePath = hand
	in := testutil.Document()
	raw, e := pdfgen.Generate(in, pdfgen.Options{Format: form, VerifyURL: "https://consulta.example/v/test"})
	if e != nil {
		t.Fatal(e)
	}
	signed, e := s.Sign(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	if out := os.Getenv("CLINICAL_FORMAT_DIR"); out != "" {
		if e = os.WriteFile(filepath.Join(out, "imagenes-prueba.pdf"), signed, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e = os.WriteFile(logo, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.invalid/private"/></svg>`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = pdfgen.Generate(in, pdfgen.Options{Format: form, VerifyURL: "https://consulta.example/v/test"}); e == nil {
		t.Fatal("external SVG content accepted")
	}
}
