package pdf_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type bboxWord struct {
	XMin float64 `xml:"xMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMin float64 `xml:"yMin,attr"`
	YMax float64 `xml:"yMax,attr"`
	Text string  `xml:",chardata"`
}
type bboxPage struct {
	Width  float64    `xml:"width,attr"`
	Height float64    `xml:"height,attr"`
	Words  []bboxWord `xml:"word"`
}
type bboxDocument struct {
	Pages []bboxPage `xml:"body>doc>page"`
}

func TestSignatureSharesContentAndMargins(t *testing.T) {
	tool := os.Getenv("CLINICAL_PDFTOTEXT")
	if tool == "" {
		var e error
		tool, e = exec.LookPath("pdftotext")
		if e != nil {
			t.Skip("Poppler no disponible para comprobar texto y márgenes independientemente")
		}
	}
	for _, n := range []int{1, 10, 17, 20, 23, 25, 26, 28, 29, 30, 45, 57} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			in := testutil.Document()
			in.Kind = "epicrisis"
			in.Content = strings.Repeat("CONTENIDO de prueba que debe acompañar la firma.\n", n)
			b, e := pdfgen.Generate(in, pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"})
			if e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(tool, "-bbox", "-", "-")
			cmd.Stdin = bytes.NewReader(b)
			output, e := cmd.Output()
			if e != nil {
				t.Fatal(e)
			}
			var doc bboxDocument
			if e = xml.Unmarshal(output, &doc); e != nil {
				t.Fatal(e)
			}
			if len(doc.Pages) == 0 {
				t.Fatal("no pages extracted")
			}
			var last strings.Builder
			for i, page := range doc.Pages {
				if math.Abs(page.Width-pdfgen.Width) > .02 || math.Abs(page.Height-pdfgen.Height) > .02 {
					t.Fatal("incorrect letter dimensions")
				}
				for _, w := range page.Words {
					if w.XMin < pdfgen.Margin-.5 || w.XMax > pdfgen.Width-pdfgen.Margin+.5 || w.YMin < pdfgen.Margin-.5 || w.YMax > pdfgen.Height-pdfgen.Margin+.5 {
						t.Fatalf("text outside 25 mm margins: page %d %+v", i+1, w)
					}
					if i == len(doc.Pages)-1 {
						last.WriteString(w.Text)
						last.WriteByte(' ')
					}
				}
			}
			if !strings.Contains(last.String(), "CONTENIDO") || !strings.Contains(last.String(), "Documento firmado") {
				t.Fatal("signature does not share a page with document content")
			}
		})
	}
}
func TestSpanishIssueDate(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	if got := pdfgen.LongDate("Santiago", at); got != "Santiago, 16 de octubre de 2026" {
		t.Fatal(got)
	}
}
