package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/vault"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"strconv"
	"time"
)

type Secrets struct {
	TOTP          string `json:"totp"`
	Certificate   []byte `json:"certificate"`
	PrivateKey    []byte `json:"private_key"`
	MedplumID     string `json:"medplum_id"`
	MedplumSecret string `json:"medplum_secret"`
}

func (s *Secrets) Clear() { vault.Wipe(s.Certificate); vault.Wipe(s.PrivateKey); *s = Secrets{} }

type Password struct {
	Params vault.Params
	Hash   []byte
}

func Token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Setup(s *store.Store, password, phrase []byte, sec Secrets) error {
	if len(password) < 12 || len(phrase) < 20 {
		return errors.New("contraseña mínima: 12 bytes; frase mínima: 20 bytes")
	}
	if subtle.ConstantTimeCompare(password, phrase) == 1 {
		return errors.New("usa una frase diferente de la contraseña")
	}
	p := vault.NewParams()
	k, e := vault.Derive(phrase, p)
	if e != nil {
		return e
	}
	defer vault.Wipe(k)
	raw, e := json.Marshal(sec)
	if e != nil {
		return e
	}
	defer vault.Wipe(raw)
	sealed, e := vault.Seal(k, raw, "settings:secrets:v1")
	if e != nil {
		return e
	}
	pp := vault.NewParams()
	hash, e := vault.Derive(password, pp)
	if e != nil {
		return e
	}
	defer vault.Wipe(hash)
	kd, _ := json.Marshal(p)
	pw, _ := json.Marshal(Password{pp, hash})
	return s.Initialize(map[string][]byte{"kdf": kd, "password": pw, "secrets": sealed, "totp_step": []byte("-1"), "attempts": []byte(`{"count":0,"until":0}`)})
}
func Unlock(s *store.Store, phrase []byte) ([]byte, Secrets, error) {
	var sec Secrets
	raw, e := s.Get("kdf")
	if e != nil {
		return nil, sec, e
	}
	var p vault.Params
	if e = json.Unmarshal(raw, &p); e != nil {
		return nil, sec, e
	}
	key, e := vault.Derive(phrase, p)
	if e != nil {
		return nil, sec, e
	}
	sealed, e := s.Get("secrets")
	if e != nil {
		vault.Wipe(key)
		return nil, sec, e
	}
	plain, e := vault.Open(key, sealed, "settings:secrets:v1")
	if e != nil {
		vault.Wipe(key)
		return nil, sec, errors.New("desbloqueo inválido")
	}
	defer vault.Wipe(plain)
	if e = json.Unmarshal(plain, &sec); e != nil {
		vault.Wipe(key)
		sec.Clear()
		return nil, sec, e
	}
	return key, sec, nil
}
func PasswordOK(s *store.Store, password []byte) bool {
	raw, e := s.Get("password")
	if e != nil {
		return false
	}
	var p Password
	if json.Unmarshal(raw, &p) != nil {
		return false
	}
	h, e := vault.Derive(password, p.Params)
	if e != nil {
		return false
	}
	defer vault.Wipe(h)
	return subtle.ConstantTimeCompare(h, p.Hash) == 1
}

type Attempts struct {
	Count int   `json:"count"`
	Until int64 `json:"until"`
}

func Allowed(s *store.Store, now time.Time) bool {
	b, e := s.Get("attempts")
	if e != nil {
		return false
	}
	var a Attempts
	if json.Unmarshal(b, &a) != nil {
		return false
	}
	return now.Unix() >= a.Until
}
func Failed(s *store.Store, now time.Time) error {
	b, e := s.Get("attempts")
	if e != nil {
		return e
	}
	var a Attempts
	if e = json.Unmarshal(b, &a); e != nil {
		return e
	}
	a.Count++
	if a.Count >= 5 {
		a.Until = now.Add(15 * time.Minute).Unix()
		a.Count = 0
	}
	b, _ = json.Marshal(a)
	return s.Set("attempts", b)
}
func Reset(s *store.Store) error { return s.Set("attempts", []byte(`{"count":0,"until":0}`)) }

// ConsumeTOTP accepts a small time skew and persists the matched step before login.
func ConsumeTOTP(s *store.Store, secret, code string, now time.Time) bool {
	b, e := s.Get("totp_step")
	if e != nil {
		return false
	}
	last, e := strconv.ParseInt(string(b), 10, 64)
	if e != nil {
		return false
	}
	current := now.Unix() / 30
	for _, delta := range []int64{0, -1, 1} {
		step := current + delta
		if step <= last {
			continue
		}
		valid, e := totp.ValidateCustom(code, secret, time.Unix(step*30, 0), totp.ValidateOpts{Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if e == nil && valid {
			return s.Set("totp_step", []byte(strconv.FormatInt(step, 10))) == nil
		}
	}
	return false
}
