package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	DB   *sql.DB
	Path string
}

const applicationID = 0x43504c31 // CPL1: refuse databases owned by other apps.
type Document struct {
	ID      string `json:"id"`
	Issued  string `json:"issued"`
	Hash    string `json:"hash"`
	Revoked bool   `json:"revoked"`
	Payload []byte `json:"-"`
	PDF     []byte `json:"-"`
}
type Event struct {
	ID       int64  `json:"id"`
	Document string `json:"document"`
	At       string `json:"at"`
	Hash     string `json:"hash"`
	Action   string `json:"action"`
}

func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "clinical.db")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	if info.Size() != 0 {
		var header [100]byte
		_, e = f.ReadAt(header[:], 0)
		if e != nil || string(header[:16]) != "SQLite format 3\x00" || binary.BigEndian.Uint32(header[68:72]) != applicationID {
			f.Close()
			return nil, errors.New("refusing a database not owned by certplum")
		}
		if binary.BigEndian.Uint32(header[60:64]) != 1 {
			f.Close()
			return nil, errors.New("unsupported database schema version")
		}
	}
	f.Close()
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db, path}
	_, e = db.Exec(`PRAGMA application_id=0x43504c31; PRAGMA user_version=1; PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA temp_store=MEMORY;
 CREATE TABLE IF NOT EXISTS settings (name TEXT PRIMARY KEY, value BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS documents (id TEXT PRIMARY KEY, issued TEXT NOT NULL, hash TEXT NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, payload BLOB NOT NULL, pdf BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY, document TEXT NOT NULL REFERENCES documents(id), at TEXT NOT NULL, hash TEXT NOT NULL, action TEXT NOT NULL CHECK(action IN ('emitido','descargado','revocado')));
 CREATE TRIGGER IF NOT EXISTS audit_no_update BEFORE UPDATE ON audit BEGIN SELECT RAISE(ABORT,'append only'); END;
 CREATE TRIGGER IF NOT EXISTS audit_no_delete BEFORE DELETE ON audit BEGIN SELECT RAISE(ABORT,'append only'); END;
	 PRAGMA application_id=0x43504c31; PRAGMA user_version=1;`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }

// NextFolio reserves an irreversible sequence number. Failed emissions may leave gaps.
// It is atomic across processes; restored backups must not run alongside the source.
func (s *Store) NextFolio(kind string, year int) (string, error) {
	prefix := map[string]string{"certificado": "CERT", "evolucion": "EVO", "epicrisis": "EPI"}[kind]
	if prefix == "" || year < 2000 || year > 9999 {
		return "", errors.New("invalid folio series")
	}
	var n int64
	err := s.DB.QueryRow(`INSERT INTO settings(name,value) VALUES (?,1) ON CONFLICT(name) DO UPDATE SET value=CAST(settings.value AS INTEGER)+1 RETURNING value`, fmt.Sprintf("folio:%s:%d", prefix, year)).Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%04d", prefix, year, n), nil
}
func (s *Store) Get(name string) ([]byte, error) {
	var b []byte
	e := s.DB.QueryRow("SELECT value FROM settings WHERE name=?", name).Scan(&b)
	return b, e
}
func (s *Store) Set(name string, b []byte) error {
	_, e := s.DB.Exec("INSERT INTO settings VALUES (?,?) ON CONFLICT(name) DO UPDATE SET value=excluded.value", name, b)
	return e
}
func (s *Store) Initialize(values map[string][]byte) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRow("SELECT COUNT(*) FROM settings").Scan(&n); e != nil {
		return e
	}
	if n != 0 {
		return errors.New("already initialized")
	}
	for k, v := range values {
		if _, e = tx.Exec("INSERT INTO settings VALUES (?,?)", k, v); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func addEvent(tx *sql.Tx, d Document, action string) error {
	_, e := tx.Exec("INSERT INTO audit(document,at,hash,action) VALUES (?,?,?,?)", d.ID, time.Now().UTC().Format(time.RFC3339Nano), d.Hash, action)
	return e
}
func (s *Store) Issue(d Document) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO documents(id,issued,hash,payload,pdf) VALUES (?,?,?,?,?)", d.ID, d.Issued, d.Hash, d.Payload, d.PDF); e != nil {
		return e
	}
	if e = addEvent(tx, d, "emitido"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Find(id string) (Document, error) {
	var d Document
	e := s.DB.QueryRow("SELECT id,issued,hash,revoked,payload,pdf FROM documents WHERE id=?", id).Scan(&d.ID, &d.Issued, &d.Hash, &d.Revoked, &d.Payload, &d.PDF)
	return d, e
}
func (s *Store) Public(id string) (Document, error) {
	var d Document
	e := s.DB.QueryRow("SELECT id,issued,hash,revoked FROM documents WHERE id=?", id).Scan(&d.ID, &d.Issued, &d.Hash, &d.Revoked)
	return d, e
}
func (s *Store) List() ([]Document, error) {
	rows, e := s.DB.Query("SELECT id,issued,hash,revoked FROM documents ORDER BY issued DESC LIMIT 500")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if e = rows.Scan(&d.ID, &d.Issued, &d.Hash, &d.Revoked); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Downloaded(d Document) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = addEvent(tx, d, "descargado"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Revoke(id string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var d Document
	e = tx.QueryRow("SELECT id,hash,revoked FROM documents WHERE id=?", id).Scan(&d.ID, &d.Hash, &d.Revoked)
	if e != nil {
		return e
	}
	if d.Revoked {
		return nil
	}
	if _, e = tx.Exec("UPDATE documents SET revoked=1 WHERE id=?", id); e != nil {
		return e
	}
	if e = addEvent(tx, d, "revocado"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Events() ([]Event, error) {
	rows, e := s.DB.Query("SELECT id,document,at,hash,action FROM audit ORDER BY id DESC LIMIT 1000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.Document, &v.At, &v.Hash, &v.Action); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Snapshot includes committed WAL transactions; never copy a live database file.
func (s *Store) Snapshot(path string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if _, e = os.Stat(abs); !os.IsNotExist(e) {
		return errors.New("backup destination must not exist")
	}
	_, e = s.DB.ExecContext(context.Background(), "VACUUM INTO '"+strings.ReplaceAll(abs, "'", "''")+"'")
	if e != nil {
		return e
	}
	return os.Chmod(abs, 0600)
}
func Check(path string) error {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return e
	}
	defer db.Close()
	var owner, version int
	if e = db.QueryRow("PRAGMA application_id").Scan(&owner); e != nil || owner != applicationID {
		return errors.New("backup is not owned by this application")
	}
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 1 {
		return errors.New("unsupported backup schema")
	}
	var v string
	if e = db.QueryRow("PRAGMA integrity_check").Scan(&v); e != nil {
		return e
	}
	if v != "ok" {
		return fmt.Errorf("backup integrity check failed")
	}
	return nil
}
