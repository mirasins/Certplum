// Package vault encrypts payloads before they reach SQLite or its journal.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/argon2"
)

type Params struct {
	Salt    []byte
	Time    uint32
	Memory  uint32
	Threads uint8
}

func NewParams() Params {
	s := make([]byte, 32)
	if _, e := rand.Read(s); e != nil {
		panic(e)
	}
	return Params{s, 3, 64 * 1024, 2}
}
func Derive(phrase []byte, p Params) ([]byte, error) {
	if len(p.Salt) != 32 || p.Time < 1 || p.Time > 10 || p.Memory < 64*1024 || p.Memory > 256*1024 || p.Threads < 1 || p.Threads > 8 {
		return nil, errors.New("invalid KDF parameters")
	}
	return argon2.IDKey(phrase, p.Salt, p.Time, p.Memory, p.Threads, 32), nil
}
func aead(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 256 bits")
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func Seal(key, plain []byte, aad string) ([]byte, error) {
	a, e := aead(key)
	if e != nil {
		return nil, e
	}
	n := make([]byte, a.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return nil, e
	}
	return a.Seal(n, n, plain, []byte(aad)), nil
}
func Open(key, data []byte, aad string) ([]byte, error) {
	a, e := aead(key)
	if e != nil {
		return nil, e
	}
	if len(data) < a.NonceSize()+a.Overhead() {
		return nil, errors.New("invalid ciphertext")
	}
	return a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte(aad))
}
func Wipe(b []byte) { clear(b) }
