package httpserver

import (
	"encoding/json"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/testutil"
	"github.com/pquerna/otp/totp"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in, local HTTPS harness for scripts/check-ui.mjs. Never built into the app.
func TestBrowserHarness(t *testing.T) {
	dir := os.Getenv("CLINICAL_UI_TEST_DIR")
	if dir == "" {
		t.Skip("opt-in browser harness")
	}
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	sec := testutil.Credentials(t)
	defer sec.Clear()
	if e = auth.Setup(s, []byte("contraseña local de prueba"), []byte("frase larga y diferente de prueba local"), sec); e != nil {
		t.Fatal(e)
	}
	app := New(config.Config{IdleMinutes: 5, Document: testutil.Format()}, s)
	defer app.Close()
	server := httptest.NewTLSServer(app)
	defer server.Close()
	app.Config.Origin = server.URL
	code, _ := totp.GenerateCode(sec.TOTP, time.Now())
	b, _ := json.Marshal(map[string]string{"url": server.URL, "totp": code})
	if e = os.WriteFile(filepath.Join(dir, "demo.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if _, e = os.Stat(filepath.Join(dir, "done")); e == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("browser test did not finish")
}
