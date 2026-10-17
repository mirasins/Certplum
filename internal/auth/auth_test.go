package auth_test

import (
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/testutil"
	"github.com/mirasins/Certplum/internal/vault"
	"github.com/pquerna/otp/totp"
	"testing"
	"time"
)

func TestAuthenticationAndLockout(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	sec := testutil.Credentials(t)
	defer sec.Clear()
	pw := []byte("contraseña solo para prueba")
	phrase := []byte("frase diferente y extensa para pruebas")
	if e = auth.Setup(s, pw, phrase, sec); e != nil {
		t.Fatal(e)
	}
	if auth.PasswordOK(s, []byte("incorrecta")) || !auth.PasswordOK(s, pw) {
		t.Fatal("password check failed")
	}
	if k, _, e := auth.Unlock(s, []byte("incorrecta")); e == nil {
		vault.Wipe(k)
		t.Fatal("wrong phrase accepted")
	}
	key, got, e := auth.Unlock(s, phrase)
	if e != nil {
		t.Fatal(e)
	}
	vault.Wipe(key)
	defer got.Clear()
	now := time.Now()
	code, _ := totp.GenerateCode(got.TOTP, now)
	if !auth.ConsumeTOTP(s, got.TOTP, code, now) || auth.ConsumeTOTP(s, got.TOTP, code, now) {
		t.Fatal("TOTP replay control failed")
	}
	for i := 0; i < 5; i++ {
		if e = auth.Failed(s, now); e != nil {
			t.Fatal(e)
		}
	}
	if auth.Allowed(s, now) || !auth.Allowed(s, now.Add(16*time.Minute)) {
		t.Fatal("persistent rate limit failed")
	}
	if e = auth.Setup(s, pw, phrase, sec); e == nil {
		t.Fatal("reinitialization accepted")
	}
}
