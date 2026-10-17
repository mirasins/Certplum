// Package testutil generates fictitious credentials in memory. No private fixture is committed.
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/documents"
	"math/big"
	"sync"
	"testing"
	"time"
)

var once sync.Once
var cert, key []byte
var err error

func Credentials(t testing.TB) auth.Secrets {
	t.Helper()
	once.Do(func() {
		var k *rsa.PrivateKey
		k, err = rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			return
		}
		c := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "Profesional de Prueba"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(48 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment, UnknownExtKeyUsage: []asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 36}}, BasicConstraintsValid: true}
		var der []byte
		der, err = x509.CreateCertificate(rand.Reader, c, c, &k.PublicKey, k)
		if err != nil {
			return
		}
		cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		der, err = x509.MarshalPKCS8PrivateKey(k)
		key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	})
	if err != nil {
		t.Fatal(err)
	}
	return auth.Secrets{TOTP: "JBSWY3DPEHPK3PXP", Certificate: append([]byte(nil), cert...), PrivateKey: append([]byte(nil), key...)}
}
func Document() documents.Input {
	return documents.Input{Kind: "certificado", Patient: "Persona Ficticia Álvarez", RUT: "12.345.678-5", Date: "2026-10-16", Content: "Documento de prueba sin datos reales. Atención clínica, evolución y coordinación.", Diagnosis: "", Folio: "CERT-2026-0001"}
}
func Format() config.DocumentFormat {
	return config.DocumentFormat{City: "Santiago", Professional: config.Professional{Name: "Profesional de Prueba", RUT: "12.345.678-5", Profession: "Psicólogo Clínico", Registration: "REGISTRO DE PRUEBA", Contact: "contacto@example.invalid"}, CertificateClosing: "Se extiende el presente certificado a petición del interesado para los fines que estime convenientes."}
}
