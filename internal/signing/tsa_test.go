package signing

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"github.com/digitorus/timestamp"
	pdfgen "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/testutil"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A dedicated fixture CA is pinned as the PDF certificate. Its TSA leaf is
// trusted through that root, without editing the machine's certificate store.
func TestRFC3161SuccessAndNonceMismatch(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(100), Subject: pkix.Name{CommonName: "CA ficticia de pruebas"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection, x509.ExtKeyUsageTimeStamping}}
	der, e := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	root, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	tsaKey, e := rsa.GenerateKey(rand.Reader, 3072)
	if e != nil {
		t.Fatal(e)
	}
	eku, _ := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
	leaf := &x509.Certificate{SerialNumber: big.NewInt(101), Subject: pkix.Name{CommonName: "TSA ficticia"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true, Value: eku}}}
	der, e = x509.CreateCertificate(rand.Reader, leaf, root, &tsaKey.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	tsaCert, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := pdfgen.Generate(testutil.Document(), pdfgen.Options{Format: testutil.Format(), VerifyURL: "https://consulta.example/v/test"})
	if e != nil {
		t.Fatal(e)
	}
	for _, badNonce := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(io.LimitReader(r.Body, 10000))
			request, e := timestamp.ParseRequest(b)
			if e != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			nonce := request.Nonce
			if badNonce {
				nonce = big.NewInt(1)
			}
			stamp := timestamp.Timestamp{HashAlgorithm: request.HashAlgorithm, HashedMessage: request.HashedMessage, Time: time.Now().UTC(), SerialNumber: big.NewInt(200), Policy: asn1.ObjectIdentifier{1, 2, 3, 4}, Nonce: nonce, AddTSACertificate: true, Certificates: []*x509.Certificate{root}}
			response, e := stamp.CreateResponse(tsaCert, tsaKey)
			if e != nil {
				http.Error(w, "fixture failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/timestamp-reply")
			w.Write(response)
		}))
		s := &LocalSigner{Key: key, Cert: root, TSAURL: server.URL}
		signed, e := s.Sign(context.Background(), raw)
		server.Close()
		if badNonce {
			if e == nil {
				t.Fatal("TSA nonce mismatch accepted")
			}
		} else {
			if e != nil {
				t.Fatal(e)
			}
			if e = Verify(signed, root, true); e != nil {
				t.Fatal(e)
			}
		}
	}
}
