package backup

import (
	"bytes"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/testutil"
	"github.com/mirasins/Certplum/internal/vault"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupWALRestoreAndTampering(t *testing.T) {
	root := t.TempDir()
	s, e := store.Open(filepath.Join(root, "source"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	phrase := []byte("frase de respaldo larga y diferente")
	sec := testutil.Credentials(t)
	defer sec.Clear()
	if e = auth.Setup(s, []byte("contraseña de prueba"), phrase, sec); e != nil {
		t.Fatal(e)
	}
	key, got, e := auth.Unlock(s, phrase)
	if e != nil {
		t.Fatal(e)
	}
	got.Clear()
	defer vault.Wipe(key)
	plain := []byte("MARCADOR_CLINICO_FICTICIO_NO_PERSISTIR")
	encrypted, _ := vault.Seal(key, plain, "document:test:pdf:v1")
	d := store.Document{ID: "test", Issued: "2026-10-16T12:00:00Z", Hash: "hash-publico-de-prueba", PDF: encrypted, Payload: encrypted}
	if e = s.Issue(d); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, "snapshot.backup")
	if e = Export(s, phrase, out); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(out)
	if bytes.Contains(raw, plain) || bytes.Contains(raw, []byte(d.Hash)) {
		t.Fatal("backup leaked plaintext or public metadata")
	}
	dest := filepath.Join(root, "restored")
	if e = Restore(out, dest, []byte("incorrecta")); e == nil {
		t.Fatal("wrong phrase accepted")
	}
	if e = Restore(out, dest, phrase); e != nil {
		t.Fatal(e)
	}
	rest, e := store.Open(dest)
	if e != nil {
		t.Fatal(e)
	}
	gotDoc, e := rest.Find("test")
	if e != nil || gotDoc.Hash != d.Hash {
		t.Fatal("WAL transaction missing from backup")
	}
	events, e := rest.Events()
	if e != nil || len(events) != 1 {
		t.Fatal("audit missing")
	}
	rest.Close()
	if e = Restore(out, dest, phrase); e == nil {
		t.Fatal("overwrite accepted")
	}
	if e = Export(s, phrase, out); e == nil {
		t.Fatal("backup overwrite accepted")
	}
	raw[len(raw)-1] ^= 1
	bad := filepath.Join(root, "bad.backup")
	os.WriteFile(bad, raw, 0600)
	if e = Restore(bad, filepath.Join(root, "bad"), phrase); e == nil {
		t.Fatal("tampered backup accepted")
	}
	for _, suffix := range []string{"", "-wal"} {
		data, _ := os.ReadFile(s.Path + suffix)
		if bytes.Contains(data, plain) {
			t.Fatal("plaintext leaked to SQLite")
		}
	}
}
