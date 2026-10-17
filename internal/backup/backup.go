package backup

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/vault"
	"io"
	"os"
	"path/filepath"
)

const magic = "CERTPLUM_BACKUP1\n"
const maxSize = 1024 * 1024 * 1024

// Export encrypts the complete consistent SQLite snapshot, including metadata.
func Export(s *store.Store, phrase []byte, dest string) error {
	key, sec, e := auth.Unlock(s, phrase)
	if e != nil {
		return e
	}
	defer vault.Wipe(key)
	defer sec.Clear()
	tmp := filepath.Join(filepath.Dir(s.Path), "backup-"+auth.Token()+".db")
	defer os.Remove(tmp)
	if e = s.Snapshot(tmp); e != nil {
		return e
	}
	if e = store.Check(tmp); e != nil {
		return e
	}
	f, e := os.Open(tmp)
	if e != nil {
		return e
	}
	plain, e := io.ReadAll(io.LimitReader(f, maxSize+1))
	f.Close()
	defer vault.Wipe(plain)
	if e != nil {
		return e
	}
	if len(plain) > maxSize {
		return errors.New("respaldo mayor a 1 GiB; requiere exportador ampliado")
	}
	p := vault.NewParams()
	bk, e := vault.Derive(phrase, p)
	if e != nil {
		return e
	}
	defer vault.Wipe(bk)
	header, _ := json.Marshal(p)
	sealed, e := vault.Seal(bk, plain, magic+string(header))
	if e != nil {
		return e
	}
	out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(dest)
		}
	}()
	var b bytes.Buffer
	b.WriteString(magic)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(header)))
	b.Write(header)
	b.Write(sealed)
	if _, e = out.Write(b.Bytes()); e != nil {
		return e
	}
	if e = out.Sync(); e != nil {
		return e
	}
	ok = true
	return nil
}

// Restore refuses any existing destination database or journal files.
func Restore(source, dir string, phrase []byte) error {
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(io.LimitReader(f, maxSize+65537))
	f.Close()
	if e != nil {
		return e
	}
	if len(raw) > maxSize+65536 || len(raw) < len(magic)+4 || string(raw[:len(magic)]) != magic {
		return errors.New("respaldo inválido")
	}
	n := int(binary.BigEndian.Uint32(raw[len(magic):]))
	offset := len(magic) + 4
	if n < 1 || n > 4096 || offset+n >= len(raw) {
		return errors.New("cabecera inválida")
	}
	header := raw[offset : offset+n]
	var p vault.Params
	if e = json.Unmarshal(header, &p); e != nil {
		return e
	}
	key, e := vault.Derive(phrase, p)
	if e != nil {
		return e
	}
	defer vault.Wipe(key)
	plain, e := vault.Open(key, raw[offset+n:], magic+string(header))
	if e != nil {
		return errors.New("frase incorrecta o respaldo alterado")
	}
	defer vault.Wipe(plain)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	if len(entries) != 0 {
		return errors.New("restaura en un directorio vacío")
	}
	path := filepath.Join(dir, "clinical.db")
	out, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = out.Write(plain)
	if e == nil {
		e = out.Sync()
	}
	out.Close()
	if e != nil {
		os.Remove(path)
		return e
	}
	if e = store.Check(path); e != nil {
		os.Remove(path)
		return e
	}
	return nil
}
