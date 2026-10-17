package pdf_test

import (
	"bytes"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"os"
	"testing"
)

func TestCompleteFontsPreserveCompositeDiacritics(t *testing.T) {
	b, e := pdfgen.Generate(testutil.Document(), pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, e := api.ReadContext(bytes.NewReader(b), model.NewDefaultConfiguration())
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, entry := range ctx.Table {
		d, ok := entry.Object.(types.Dict)
		if !ok {
			continue
		}
		kind := d.NameEntry("Type")
		if kind == nil || *kind != "FontDescriptor" {
			continue
		}
		name := d.NameEntry("FontName")
		if name == nil {
			t.Fatal("font without name")
		}
		path := "assets/NotoSans-Regular.ttf"
		if *name == "NotoSans-Bold" {
			path = "assets/NotoSans-Bold.ttf"
		}
		expected, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		font, _, e := ctx.DereferenceStreamDict(d["FontFile2"])
		if e != nil {
			t.Fatal(e)
		}
		if e = font.Decode(); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(font.Content, expected) {
			t.Fatal("full source font not embedded; nested diacritics could be lost")
		}
		found++
	}
	if found != 2 {
		t.Fatal("both fonts must be embedded")
	}
}
