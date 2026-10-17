package store

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFoliosAcrossProcessesAndYears(t *testing.T) {
	dir := t.TempDir()
	a, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	var wg sync.WaitGroup
	results := make(chan string, 20)
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := a
			if i%2 == 1 {
				s = b
			}
			folio, e := s.NextFolio("certificado", 2026)
			if e != nil {
				errs <- e
				return
			}
			results <- folio
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for v := range results {
		if seen[v] {
			t.Fatal("duplicate folio")
		}
		seen[v] = true
	}
	if len(seen) != 20 {
		t.Fatal("missing folios")
	}
	if v, e := a.NextFolio("evolucion", 2026); e != nil || v != "EVO-2026-0001" {
		t.Fatal(v, e)
	}
	if v, e := a.NextFolio("epicrisis", 2026); e != nil || v != "EPI-2026-0001" {
		t.Fatal(v, e)
	}
	if v, e := a.NextFolio("certificado", 2027); e != nil || v != "CERT-2027-0001" {
		t.Fatal(v, e)
	}
}

func TestForeignDatabaseIsUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clinical.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE unrelated(value TEXT); INSERT INTO unrelated VALUES ('foreign');"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	if s, e := Open(dir); e == nil {
		s.Close()
		t.Fatal("foreign SQLite accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("foreign SQLite modified")
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, e = os.Stat(path + suffix); !os.IsNotExist(e) {
			t.Fatal("foreign auxiliary file created")
		}
	}
}
func TestOwnReopenAndAuditProtection(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Issue(Document{ID: "test", Issued: "2026-10-16", Hash: "test-hash", Payload: []byte("cipher"), PDF: []byte("cipher")}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("DELETE FROM audit"); e == nil {
		t.Fatal("audit deletion accepted")
	}
	if _, e = s.DB.Exec("UPDATE audit SET action='revocado'"); e == nil {
		t.Fatal("audit update accepted")
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Find("test"); e != nil {
		t.Fatal(e)
	}
}
