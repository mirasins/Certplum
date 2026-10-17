package signing

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	pdflib "github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign"
	"github.com/digitorus/pdfsign/extract"
	"github.com/digitorus/pdfsign/sign"
	"time"
)

type Signer interface {
	Sign(context.Context, []byte) ([]byte, error)
}

var ErrProviderUnavailable = errors.New("el prestador de firma avanzada todavía no está implementado")

// ProviderSigner will implement the provider's OTP flow. Never falls back to LocalSigner.
type ProviderSigner struct{}

func (ProviderSigner) Sign(context.Context, []byte) ([]byte, error) {
	return nil, ErrProviderUnavailable
}

type LocalSigner struct {
	Key      crypto.Signer
	Cert     *x509.Certificate
	Chain    []*x509.Certificate
	TSAURL   string
	SignedAt time.Time
	Name     string
}

func Parse(certPEM, keyPEM []byte) (*LocalSigner, error) {
	var certs []*x509.Certificate
	for len(certPEM) > 0 {
		b, rest := pem.Decode(certPEM)
		if b == nil {
			break
		}
		certPEM = rest
		if b.Type != "CERTIFICATE" {
			continue
		}
		c, e := x509.ParseCertificate(b.Bytes)
		if e != nil {
			return nil, e
		}
		certs = append(certs, c)
	}
	if len(certs) == 0 {
		return nil, errors.New("certificado PEM requerido")
	}
	b, _ := pem.Decode(keyPEM)
	if b == nil || b.Type != "PRIVATE KEY" {
		return nil, errors.New("se requiere clave PKCS#8 PEM (PRIVATE KEY)")
	}
	key, e := x509.ParsePKCS8PrivateKey(b.Bytes)
	if e != nil {
		return nil, e
	}
	k, ok := key.(*rsa.PrivateKey)
	if !ok || k.N.BitLen() < 3072 {
		return nil, errors.New("se requiere clave RSA de al menos 3072 bits")
	}
	a, _ := x509.MarshalPKIXPublicKey(k.Public())
	pub, _ := x509.MarshalPKIXPublicKey(certs[0].PublicKey)
	if !bytes.Equal(a, pub) {
		return nil, errors.New("la clave no corresponde al certificado")
	}
	s := &LocalSigner{Key: k, Cert: certs[0], Chain: certs[1:]}
	return s, s.Check()
}
func (s *LocalSigner) Check() error {
	now := time.Now()
	if now.Before(s.Cert.NotBefore) || now.After(s.Cert.NotAfter) {
		return errors.New("certificado fuera de vigencia")
	}
	if s.Cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("certificado sin uso digitalSignature")
	}
	return nil
}
func (s *LocalSigner) Sign(ctx context.Context, in []byte) ([]byte, error) {
	if e := s.Check(); e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	reader := bytes.NewReader(in)
	doc, e := pdflib.NewReader(reader, int64(len(in)))
	if e != nil {
		return nil, e
	}
	at := s.SignedAt
	if at.IsZero() {
		at = time.Now()
	}
	name := s.Name
	if name == "" {
		name = s.Cert.Subject.CommonName
	}
	data := sign.SignData{Signer: s.Key, Certificate: s.Cert, CertificateChains: [][]*x509.Certificate{append([]*x509.Certificate{s.Cert}, s.Chain...)}, DigestAlgorithm: crypto.SHA256, SubFilter: sign.SubFilterETSICAdESDetached, Context: ctx, TSA: sign.TSA{URL: s.TSAURL}, Signature: sign.SignDataSignature{CertType: sign.CertificationSignature, DocMDPPerm: sign.DoNotAllowAnyChangesPerms, Info: sign.SignDataSignatureInfo{Name: name, Date: at, Reason: "Emisión de documento clínico"}}}
	var out bytes.Buffer
	// The lower-level library API allows a single supplied signing instant for
	// both the visible block and /M. No custom cryptographic code is involved.
	if e = sign.SignWithData(reader, &out, doc, int64(len(in)), data); e != nil {
		return nil, e
	}
	if e = Verify(out.Bytes(), s.Cert, s.TSAURL != ""); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}

// Verify pins the expected certificate; it never trusts arbitrary embedded roots.
func Verify(data []byte, expected *x509.Certificate, requireTimestamp bool) error {
	doc, e := pdfsign.Open(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return e
	}
	roots, e := x509.SystemCertPool()
	if e != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	roots.AddCert(expected)
	v := doc.Verify().TrustedRoots(roots).ExternalChecks(false)
	if !v.Valid() {
		return errors.New("firma o cadena inválida")
	}
	count := 0
	for signature, err := range extract.Iter(doc.Reader(), bytes.NewReader(data)) {
		if err != nil {
			return err
		}
		count++
		r := signature.ByteRange()
		if len(r) != 4 || r[0] != 0 || r[2]+r[3] != int64(len(data)) {
			return errors.New("la firma no cubre el archivo completo o existen cambios posteriores")
		}
	}
	if count != 1 {
		return errors.New("número de firmas inesperado")
	}
	sigs := v.Signatures()
	if len(sigs) != 1 {
		return errors.New("número de firmas inesperado")
	}
	// A library-level chain check is complemented by a pinned signer certificate below.
	if sigs[0].Certificate == nil || !bytes.Equal(sigs[0].Certificate.Raw, expected.Raw) {
		return errors.New("certificado de firma inesperado")
	}
	if requireTimestamp && (sigs[0].Timestamp == nil || !sigs[0].TimestampValid) {
		return errors.New("sello de tiempo no confiable")
	}
	return nil
}
