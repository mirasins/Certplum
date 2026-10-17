package vault

import (
	"bytes"
	"testing"
)

func TestEncryptionIntegrityAndNonce(t *testing.T) {
	p := NewParams()
	key, e := Derive([]byte("frase de prueba suficientemente extensa"), p)
	if e != nil {
		t.Fatal(e)
	}
	defer Wipe(key)
	plain := []byte("contenido clínico ficticio")
	a, e := Seal(key, plain, "doc:1")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := Seal(key, plain, "doc:1")
	if bytes.Equal(a[:12], b[:12]) {
		t.Fatal("nonce reused")
	}
	if bytes.Contains(a, plain) {
		t.Fatal("plaintext leaked")
	}
	got, e := Open(key, a, "doc:1")
	if e != nil || !bytes.Equal(got, plain) {
		t.Fatal("round trip failed")
	}
	if _, e = Open(key, a, "doc:2"); e == nil {
		t.Fatal("AAD substitution accepted")
	}
	wrong := make([]byte, 32)
	if _, e = Open(wrong, a, "doc:1"); e == nil {
		t.Fatal("wrong key accepted")
	}
	a[len(a)-1] ^= 1
	if _, e = Open(key, a, "doc:1"); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = Open(key, []byte{1}, "doc:1"); e == nil {
		t.Fatal("truncated data accepted")
	}
	Wipe(key)
	if !bytes.Equal(key, make([]byte, 32)) {
		t.Fatal("wipe failed")
	}
}
func TestKDFBounds(t *testing.T) {
	p := NewParams()
	p.Memory = 1
	if _, e := Derive([]byte("test"), p); e == nil {
		t.Fatal("unsafe parameters accepted")
	}
}
