package signing

import (
	"bytes"
	"context"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"os"
	"testing"
	"time"
)

func TestPAdESAndTampering(t *testing.T) {
	sec := testutil.Credentials(t)
	defer sec.Clear()
	s, e := Parse(sec.Certificate, sec.PrivateKey)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := pdfgen.Generate(testutil.Document(), pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/0123456789"})
	if e != nil {
		t.Fatal(e)
	}
	signed, e := s.Sign(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	if e = Verify(signed, s.Cert, false); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(signed, []byte("/ETSI.CAdES.detached")) {
		t.Fatal("PAdES subfilter missing")
	}
	if e = Verify(raw, s.Cert, false); e == nil {
		t.Fatal("unsigned PDF accepted")
	}
	if e = Verify(signed, s.Cert, true); e == nil {
		t.Fatal("missing timestamp accepted")
	}
	bad := append([]byte(nil), signed...)
	bad[20] ^= 1
	if e = Verify(bad, s.Cert, false); e == nil {
		t.Fatal("tampered PDF accepted")
	}
	if e = Verify(append(append([]byte(nil), signed...), []byte("\n% modificación posterior\n")...), s.Cert, false); e == nil {
		t.Fatal("unsigned trailing bytes accepted")
	}
	// Optional review artifact, explicitly fictitious and gitignored.
	if path := os.Getenv("CLINICAL_TEST_PDF"); path != "" {
		if e = os.WriteFile(path, signed, 0600); e != nil {
			t.Fatal(e)
		}
	}
	s.Cert.NotAfter = time.Now().Add(-time.Hour)
	if _, e = s.Sign(context.Background(), raw); e == nil {
		t.Fatal("expired certificate accepted")
	}
}
func TestTSAFailureAndProviderStub(t *testing.T) {
	sec := testutil.Credentials(t)
	defer sec.Clear()
	s, e := Parse(sec.Certificate, sec.PrivateKey)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := pdfgen.Generate(testutil.Document(), pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"})
	if e != nil {
		t.Fatal(e)
	}
	s.TSAURL = "https://127.0.0.1:1"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e = s.Sign(ctx, raw); e == nil {
		t.Fatal("TSA failure silently downgraded")
	}
	if _, e = (ProviderSigner{}).Sign(ctx, raw); e != ErrProviderUnavailable {
		t.Fatal("provider stub did not fail closed")
	}
}
