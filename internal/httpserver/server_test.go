package httpserver

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/signing"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/testutil"
	"github.com/pquerna/otp/totp"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFullWorkflowAndSecurity(t *testing.T) {
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	sec := testutil.Credentials(t)
	defer sec.Clear()
	pw := "contraseña local de prueba"
	phrase := "frase larga y diferente de prueba local"
	if e = auth.Setup(s, []byte(pw), []byte(phrase), sec); e != nil {
		t.Fatal(e)
	}
	c := config.Config{Listen: "127.0.0.1:18765", Origin: "https://consulta.example", DataDir: t.TempDir(), IdleMinutes: 5, Document: testutil.Format()}
	app := New(c, s)
	defer app.Close()
	csrf := auth.Token()
	session := ""
	call := func(method, path string, data any, validCSRF bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if data != nil {
			json.NewEncoder(&b).Encode(data)
		}
		r := httptest.NewRequest(method, c.Origin+path, &b)
		r.TLS = &tls.ConnectionState{}
		r.AddCookie(&http.Cookie{Name: csrfCookie, Value: csrf})
		if session != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
		}
		if method == "POST" {
			r.Header.Set("Content-Type", "application/json")
			if validCSRF {
				r.Header.Set("Origin", c.Origin)
				r.Header.Set("X-CSRF-Token", csrf)
			}
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/documents", nil, true); w.Code != 401 {
		t.Fatal("locked history exposed")
	}
	if w := call("POST", "/api/login", map[string]string{}, false); w.Code != 403 {
		t.Fatal("CSRF bypass")
	}
	code, _ := totp.GenerateCode(sec.TOTP, time.Now())
	w := call("POST", "/api/login", map[string]string{"password": pw, "phrase": phrase, "totp": code}, true)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookie {
			session = cookie.Value
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatal("insecure cookie")
			}
		}
	}
	if session == "" {
		t.Fatal("session missing")
	}
	input := testutil.Document()
	input.Folio = "FOLIO-FORJADO"
	w = call("POST", "/api/documents", input, true)
	if w.Code != 201 {
		t.Fatalf("issue: %d %s", w.Code, w.Body.String())
	}
	var d store.Document
	if e = json.Unmarshal(w.Body.Bytes(), &d); e != nil {
		t.Fatal(e)
	}
	w = call("POST", "/api/documents/"+d.ID+"/download", map[string]string{}, true)
	if w.Code != 200 {
		t.Fatalf("download: %s", w.Body.String())
	}
	signer, _ := signing.Parse(sec.Certificate, sec.PrivateKey)
	if e = signing.Verify(w.Body.Bytes(), signer.Cert, false); e != nil {
		t.Fatal(e)
	}
	w = call("GET", "/api/documents", nil, true)
	if !strings.Contains(w.Body.String(), testutil.Document().Patient) {
		t.Fatal("private history did not decrypt")
	}
	if strings.Contains(w.Body.String(), "FOLIO-FORJADO") || !strings.Contains(w.Body.String(), "CERT-") {
		t.Fatal("server did not assign its own folio")
	}
	for i := 0; i < 2; i++ {
		if w = call("POST", "/api/documents/"+d.ID+"/revoke", map[string]string{}, true); w.Code != 200 {
			t.Fatal("revoke failed")
		}
	}
	events, _ := s.Events()
	if len(events) != 3 {
		t.Fatalf("expected issued/downloaded/revoked exactly once, got %d", len(events))
	}
	w = call("POST", "/api/patients", map[string]string{"query": "Persona"}, true)
	if w.Code != 503 {
		t.Fatal("manual fallback missing")
	}
	// Verify automatic expiry wipes the key even when no browser sends a lock request.
	app.last = time.Now().Add(-6 * time.Minute)
	w = call("GET", "/api/documents", nil, true)
	if w.Code != 401 || len(app.key) != 0 || len(app.secrets.PrivateKey) != 0 {
		t.Fatal("expiry failed to wipe vault")
	}
	w = call("GET", "/verify-api/"+d.ID, nil, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"revoked":true`) {
		t.Fatal("public verification unavailable while locked")
	}
	for _, secret := range []string{testutil.Document().Patient, "patient", "rut", "content", "diagnosis", "payload", "pdf"} {
		if strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(secret)) {
			t.Fatalf("public metadata leaked: %s", secret)
		}
	}
	for _, suffix := range []string{"", "-wal"} {
		b, _ := os.ReadFile(s.Path + suffix)
		for _, plain := range []string{testutil.Document().Patient, testutil.Document().Content, "BEGIN PRIVATE KEY", sec.TOTP} {
			if bytes.Contains(b, []byte(plain)) {
				t.Fatalf("plaintext leaked to database%s", suffix)
			}
		}
	}
	// Reject spoofed proxy HTTPS on non-loopback and unexpected Host values.
	r := httptest.NewRequest("GET", c.Origin+"/", nil)
	r.TLS = nil
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, r)
	if rr.Code != 400 {
		t.Fatal("untrusted proxy accepted")
	}
	r = httptest.NewRequest("GET", "https://evil.example/", nil)
	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, r)
	if rr.Code != 400 {
		t.Fatal("unexpected Host accepted")
	}
}
