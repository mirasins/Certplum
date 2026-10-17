package config

import (
	"encoding/json"
	"errors"
	"github.com/mirasins/Certplum/internal/documents"
	"net"
	"net/url"
	"os"
	"strings"
)

type Professional struct {
	Name         string `json:"name"`
	RUT          string `json:"rut"`
	Profession   string `json:"profession"`
	Registration string `json:"registration"`
	Contact      string `json:"contact"`
}
type DocumentFormat struct {
	City               string       `json:"city"`
	Professional       Professional `json:"professional"`
	LogoPath           string       `json:"logo_path"`
	SignatureImagePath string       `json:"signature_image_path"`
	CertificateClosing string       `json:"certificate_closing"`
}

func (d DocumentFormat) Validate() error {
	for _, s := range []string{d.City, d.Professional.Name, d.Professional.Profession, d.Professional.Registration} {
		if strings.TrimSpace(s) == "" || len([]rune(s)) > 140 || strings.ContainsAny(s, "\r\n\t") {
			return errors.New("complete document city and professional identity (single lines, max 140 characters)")
		}
	}
	if !documents.ValidRUT(d.Professional.RUT) {
		return errors.New("professional RUT is invalid")
	}
	if len([]rune(d.Professional.Contact)) > 240 || strings.ContainsAny(d.Professional.Contact, "\r\n\t") {
		return errors.New("contact must be a single line of up to 240 characters")
	}
	if len([]rune(d.CertificateClosing)) > 1200 {
		return errors.New("certificate closing too long")
	}
	return nil
}

type Config struct {
	Listen      string         `json:"listen"`
	Origin      string         `json:"origin"`
	DataDir     string         `json:"data_dir"`
	IdleMinutes int            `json:"idle_minutes"`
	TSAURL      string         `json:"tsa_url"`
	MedplumURL  string         `json:"medplum_url"`
	Document    DocumentFormat `json:"document"`
}

func Load(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	h, _, e := net.SplitHostPort(c.Listen)
	if e != nil || net.ParseIP(h) == nil || !net.ParseIP(h).IsLoopback() {
		return errors.New("listen must use a literal loopback IP")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("origin must be an HTTPS origin without path")
	}
	if c.DataDir == "" || c.IdleMinutes < 1 || c.IdleMinutes > 60 {
		return errors.New("data_dir and idle_minutes (1..60) required")
	}
	for _, v := range []string{c.TSAURL, c.MedplumURL} {
		if v == "" {
			continue
		}
		u, e := url.Parse(v)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("external endpoints must be HTTPS without credentials/query")
		}
	}
	return c.Document.Validate()
}
