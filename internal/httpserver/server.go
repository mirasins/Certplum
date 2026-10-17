package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/documents"
	"github.com/mirasins/Certplum/internal/medplum"
	clinicalpdf "github.com/mirasins/Certplum/internal/pdf"
	"github.com/mirasins/Certplum/internal/signing"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/vault"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var assets embed.FS
var idPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

const sessionCookie = "__Host-clinical_session"
const csrfCookie = "__Host-clinical_csrf"

type App struct {
	Config        config.Config
	Store         *store.Store
	mu            sync.Mutex
	key           []byte
	secrets       auth.Secrets
	session       string
	csrf          string
	last, started time.Time
	publicWindow  time.Time
	publicCount   int
}

func New(c config.Config, s *store.Store) *App { return &App{Config: c, Store: s} }
func (a *App) lock()                           { vault.Wipe(a.key); a.key = nil; a.secrets.Clear(); a.session = ""; a.csrf = "" }
func (a *App) Close()                          { a.mu.Lock(); defer a.mu.Unlock(); a.lock() }
func (a *App) expired(now time.Time) bool {
	return len(a.key) > 0 && (now.Sub(a.last) >= time.Duration(a.Config.IdleMinutes)*time.Minute || now.Sub(a.started) >= 12*time.Hour)
}
func (a *App) Watch(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			a.Close()
			return
		case now := <-t.C:
			a.mu.Lock()
			if a.expired(now) {
				a.lock()
			}
			a.mu.Unlock()
		}
	}
}
func equal(a, b string) bool {
	return len(a) > 0 && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}
func parse(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}
func cookie(w http.ResponseWriter, name, value string, max int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: max})
}
func (a *App) authorized(r *http.Request) bool {
	c, e := r.Cookie(sessionCookie)
	return e == nil && len(a.key) == 32 && equal(c.Value, a.session)
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	origin, _ := url.Parse(a.Config.Origin)
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if r.Host != origin.Host || (r.TLS == nil && (ip == nil || !ip.IsLoopback() || r.Header.Get("X-Forwarded-Proto") != "https")) {
		fail(w, 400, "Se requiere HTTPS y el dominio configurado.")
		return
	}
	defer func() {
		if recover() != nil {
			fail(w, 500, "No fue posible procesar la solicitud.")
		}
	}()
	path := r.URL.Path
	if r.Method == http.MethodGet && (path == "/" || path == "/app.js" || path == "/style.css" || path == "/verify.js" || strings.HasPrefix(path, "/v/")) {
		name := strings.TrimPrefix(path, "/")
		if path == "/" {
			name = "index.html"
		}
		if strings.HasPrefix(path, "/v/") {
			if !idPattern.MatchString(strings.TrimPrefix(path, "/v/")) {
				http.NotFound(w, r)
				return
			}
			name = "verify.html"
		}
		b, e := fs.ReadFile(assets, "web/"+name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		ext := ".html"
		if strings.HasSuffix(name, ".js") {
			ext = ".js"
		}
		if strings.HasSuffix(name, ".css") {
			ext = ".css"
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
		_, _ = w.Write(b)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if a.expired(now) {
		a.lock()
	}
	if r.Method == http.MethodGet && strings.HasPrefix(path, "/verify-api/") {
		// Bounded global limiter: no IP or clinical query is retained.
		if now.Sub(a.publicWindow) > time.Minute {
			a.publicWindow = now
			a.publicCount = 0
		}
		a.publicCount++
		if a.publicCount > 120 {
			fail(w, 429, "Intenta más tarde.")
			return
		}
		id := strings.TrimPrefix(path, "/verify-api/")
		if !idPattern.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		d, e := a.Store.Public(id)
		if e != nil {
			fail(w, 404, "Documento no encontrado.")
			return
		}
		reply(w, 200, d)
		return
	}
	if path == "/api/session" && r.Method == http.MethodGet {
		c, e := r.Cookie(csrfCookie)
		token := ""
		if e == nil && idPattern.MatchString(c.Value) {
			token = c.Value
		} else {
			token = auth.Token()
			cookie(w, csrfCookie, token, 0)
		}
		ok := a.authorized(r)
		if ok {
			a.csrf = token
		}
		reply(w, 200, map[string]any{"unlocked": ok, "csrf": token, "idle_minutes": a.Config.IdleMinutes})
		return
	}
	if r.Method == http.MethodPost {
		c, e := r.Cookie(csrfCookie)
		ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || !equal(c.Value, r.Header.Get("X-CSRF-Token")) || r.Header.Get("Origin") != a.Config.Origin || ct != "application/json" {
			fail(w, 403, "Solicitud rechazada por protección CSRF.")
			return
		}
	}
	if path == "/api/login" && r.Method == http.MethodPost {
		a.login(w, r, now)
		return
	}
	if !strings.HasPrefix(path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if !a.authorized(r) {
		fail(w, 401, "La aplicación está bloqueada.")
		return
	}
	if r.Method == http.MethodPost && !equal(a.csrf, r.Header.Get("X-CSRF-Token")) {
		fail(w, 403, "Sesión CSRF inválida.")
		return
	}
	a.last = now
	switch {
	case path == "/api/activity" && r.Method == http.MethodPost:
		reply(w, 200, map[string]bool{"active": true})
	case path == "/api/lock" && r.Method == http.MethodPost:
		a.lock()
		cookie(w, sessionCookie, "", -1)
		reply(w, 200, map[string]bool{"locked": true})
	case path == "/api/documents" && r.Method == http.MethodGet:
		a.history(w)
	case path == "/api/audit" && r.Method == http.MethodGet:
		events, e := a.Store.Events()
		if e != nil {
			fail(w, 500, "No fue posible consultar la auditoría.")
			return
		}
		reply(w, 200, events)
	case path == "/api/patients" && r.Method == http.MethodPost:
		var in struct {
			Query string `json:"query"`
		}
		if parse(w, r, &in) != nil {
			fail(w, 400, "Búsqueda inválida.")
			return
		}
		c := medplum.New(a.Config.MedplumURL, a.secrets.MedplumID, a.secrets.MedplumSecret)
		rows, e := c.Search(r.Context(), in.Query)
		if e != nil {
			fail(w, 503, "Medplum no disponible o sin permiso. Puedes ingresar los datos manualmente.")
			return
		}
		reply(w, 200, rows)
	case path == "/api/documents" && r.Method == http.MethodPost:
		a.issue(w, r)
	case strings.HasPrefix(path, "/api/documents/") && r.Method == http.MethodPost:
		parts := strings.Split(strings.TrimPrefix(path, "/api/documents/"), "/")
		if len(parts) != 2 || !idPattern.MatchString(parts[0]) {
			http.NotFound(w, r)
			return
		}
		id := parts[0]
		switch parts[1] {
		case "download":
			a.download(w, id)
		case "revoke":
			if e := a.Store.Revoke(id); e != nil {
				fail(w, 404, "No fue posible revocar el documento.")
				return
			}
			reply(w, 200, map[string]bool{"revoked": true})
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}
func (a *App) login(w http.ResponseWriter, r *http.Request, now time.Time) {
	if !auth.Allowed(a.Store, now) {
		w.Header().Set("Retry-After", "900")
		fail(w, 429, "Demasiados intentos. Espera 15 minutos.")
		return
	}
	var in struct {
		Password string `json:"password"`
		Phrase   string `json:"phrase"`
		TOTP     string `json:"totp"`
	}
	if parse(w, r, &in) != nil || len(in.Password) > 1024 || len(in.Phrase) > 2048 || len(in.TOTP) != 6 {
		_ = auth.Failed(a.Store, now)
		fail(w, 401, "No fue posible desbloquear. Revisa tus credenciales.")
		return
	}
	password := []byte(in.Password)
	phrase := []byte(in.Phrase)
	defer vault.Wipe(password)
	defer vault.Wipe(phrase)
	if !auth.PasswordOK(a.Store, password) {
		_ = auth.Failed(a.Store, now)
		fail(w, 401, "No fue posible desbloquear. Revisa tus credenciales.")
		return
	}
	key, sec, e := auth.Unlock(a.Store, phrase)
	if e != nil {
		_ = auth.Failed(a.Store, now)
		fail(w, 401, "No fue posible desbloquear. Revisa tus credenciales.")
		return
	}
	if !auth.ConsumeTOTP(a.Store, sec.TOTP, in.TOTP, now) {
		vault.Wipe(key)
		sec.Clear()
		_ = auth.Failed(a.Store, now)
		fail(w, 401, "No fue posible desbloquear. Revisa tus credenciales.")
		return
	}
	if e = auth.Reset(a.Store); e != nil {
		vault.Wipe(key)
		sec.Clear()
		fail(w, 500, "No fue posible abrir la sesión.")
		return
	}
	a.lock()
	a.key = key
	a.secrets = sec
	a.session = auth.Token()
	a.csrf = r.Header.Get("X-CSRF-Token")
	a.last = now
	a.started = now
	cookie(w, sessionCookie, a.session, 0)
	reply(w, 200, map[string]bool{"unlocked": true})
}
func (a *App) issue(w http.ResponseWriter, r *http.Request) {
	var in documents.Input
	if parse(w, r, &in) != nil {
		fail(w, 400, "Documento inválido.")
		return
	}
	if e := in.Validate(); e != nil {
		fail(w, 400, e.Error())
		return
	}
	signer, e := signing.Parse(a.secrets.Certificate, a.secrets.PrivateKey)
	if e != nil {
		fail(w, 422, "Revisa la vigencia y configuración del certificado de firma.")
		return
	}
	signer.TSAURL = a.Config.TSAURL
	id := auth.Token()
	now := time.Now()
	in.Folio, e = a.Store.NextFolio(in.Kind, clinicalpdf.Santiago(now).Year())
	if e != nil {
		fail(w, 500, "No fue posible reservar el folio del documento.")
		return
	}
	signer.SignedAt = now
	signer.Name = a.Config.Document.Professional.Name
	raw, e := clinicalpdf.Generate(in, clinicalpdf.Options{Format: a.Config.Document, VerifyURL: a.Config.Origin + "/v/" + id, IssuedAt: now, SignedAt: now})
	if e != nil {
		fail(w, 422, "No fue posible generar el PDF/A. Revisa la configuración profesional, imágenes y caracteres ingresados.")
		return
	}
	defer vault.Wipe(raw)
	signed, e := signer.Sign(r.Context(), raw)
	if e != nil {
		fail(w, 502, "No se emitió el documento: falló la firma o el sello de tiempo configurado.")
		return
	}
	defer vault.Wipe(signed)
	hash := sha256.Sum256(signed)
	body, e := json.Marshal(in)
	if e != nil {
		fail(w, 500, "No fue posible guardar el documento.")
		return
	}
	defer vault.Wipe(body)
	payload, e := vault.Seal(a.key, body, "document:"+id+":payload:v1")
	if e != nil {
		fail(w, 500, "No fue posible cifrar el documento.")
		return
	}
	encrypted, e := vault.Seal(a.key, signed, "document:"+id+":pdf:v1")
	if e != nil {
		fail(w, 500, "No fue posible cifrar el documento.")
		return
	}
	d := store.Document{ID: id, Issued: now.UTC().Format(time.RFC3339Nano), Hash: hex.EncodeToString(hash[:]), Payload: payload, PDF: encrypted}
	if e = a.Store.Issue(d); e != nil {
		fail(w, 500, "No fue posible guardar el documento.")
		return
	}
	reply(w, 201, d)
}
func (a *App) history(w http.ResponseWriter) {
	docs, e := a.Store.List()
	if e != nil {
		fail(w, 500, "No fue posible consultar el historial.")
		return
	}
	type row struct {
		store.Document
		Input documents.Input `json:"input"`
	}
	out := []row{}
	for _, d := range docs {
		full, e := a.Store.Find(d.ID)
		if e != nil {
			fail(w, 500, "No fue posible consultar el historial.")
			return
		}
		b, e := vault.Open(a.key, full.Payload, "document:"+d.ID+":payload:v1")
		if e != nil {
			fail(w, 500, "Falló la integridad de un registro.")
			return
		}
		var in documents.Input
		e = json.Unmarshal(b, &in)
		vault.Wipe(b)
		if e != nil {
			fail(w, 500, "Registro inválido.")
			return
		}
		out = append(out, row{d, in})
	}
	reply(w, 200, out)
}
func (a *App) download(w http.ResponseWriter, id string) {
	d, e := a.Store.Find(id)
	if e != nil {
		fail(w, 404, "Documento no encontrado.")
		return
	}
	b, e := vault.Open(a.key, d.PDF, "document:"+id+":pdf:v1")
	if e != nil {
		fail(w, 500, "Falló la integridad del documento.")
		return
	}
	defer vault.Wipe(b)
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != d.Hash {
		fail(w, 500, "El hash del documento no coincide.")
		return
	}
	if e = a.Store.Downloaded(d); e != nil {
		fail(w, 500, "No fue posible registrar la descarga.")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="documento-`+id[:12]+`.pdf"`)
	_, _ = w.Write(b)
}
