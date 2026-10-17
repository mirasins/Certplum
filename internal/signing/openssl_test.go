package signing

import (
	"bytes"
	"context"
	"encoding/asn1"
	pdflib "github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign/extract"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIndependentOpenSSLVerification(t *testing.T) {
	tool := os.Getenv("CLINICAL_OPENSSL")
	if tool == "" {
		var e error
		tool, e = exec.LookPath("openssl")
		if e != nil {
			t.Skip("OpenSSL no disponible para comprobación independiente")
		}
	}
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
	signed, e := s.Sign(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	reader := bytes.NewReader(signed)
	r, e := pdflib.NewReader(reader, int64(len(signed)))
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for sig, e := range extract.Iter(r, reader) {
		if e != nil {
			t.Fatal(e)
		}
		found = true
		body, e := sig.SignedData()
		if e != nil {
			t.Fatal(e)
		}
		content, e := io.ReadAll(body)
		if e != nil {
			t.Fatal(e)
		}
		var envelope asn1.RawValue
		if _, e = asn1.Unmarshal(sig.Contents(), &envelope); e != nil {
			t.Fatal(e)
		}
		dir := t.TempDir()
		cms := filepath.Join(dir, "signature.der")
		data := filepath.Join(dir, "signed.bin")
		certPath := filepath.Join(dir, "signer.crt")
		os.WriteFile(cms, envelope.FullBytes, 0600)
		os.WriteFile(data, content, 0600)
		os.WriteFile(certPath, sec.Certificate, 0600)
		check := func() ([]byte, error) {
			return exec.Command(tool, "cms", "-verify", "-binary", "-inform", "DER", "-in", cms, "-content", data, "-CAfile", certPath, "-purpose", "any", "-out", filepath.Join(dir, "verified.bin")).CombinedOutput()
		}
		if out, e := check(); e != nil {
			t.Fatalf("OpenSSL rejected signature: %s", out)
		}
		content[20] ^= 1
		os.WriteFile(data, content, 0600)
		if _, e = check(); e == nil {
			t.Fatal("OpenSSL accepted tampered content")
		}
	}
	if !found {
		t.Fatal("no CMS signature extracted")
	}
}
