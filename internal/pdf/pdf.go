package pdf

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/documents"
	"github.com/signintech/gopdf"
	"github.com/skip2/go-qrcode"
	"strings"
	"time"
	_ "time/tzdata"
)

//go:embed assets/*
var assets embed.FS

const mm = 72.0 / 25.4
const Width = 216 * mm
const Height = 279 * mm
const Margin = 25 * mm
const bodyBottom = Height - Margin - 124
const continuationTop = Margin + 70

type Options struct {
	Format    config.DocumentFormat
	VerifyURL string
	IssuedAt  time.Time
	SignedAt  time.Time
}
type line struct {
	text         string
	size, height float64
	bold         bool
}
type sheet struct {
	lines []line
	start float64
}

func Santiago(t time.Time) time.Time { z, _ := time.LoadLocation("America/Santiago"); return t.In(z) }
func LongDate(city string, t time.Time) string {
	months := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	t = Santiago(t)
	return fmt.Sprintf("%s, %d de %s de %d", city, t.Day(), months[int(t.Month())-1], t.Year())
}
func ContentHash(in documents.Input) string {
	b, _ := json.Marshal(in)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Generate lays out all pages before adding their total count and archival
// metadata. Every transformation happens in memory and before PAdES signing.
func Generate(in documents.Input, o Options) ([]byte, error) {
	if e := in.Validate(); e != nil {
		return nil, e
	}
	if e := o.Format.Validate(); e != nil {
		return nil, e
	}
	if in.Folio == "" || o.VerifyURL == "" {
		return nil, errors.New("folio and verification URL required")
	}
	if o.IssuedAt.IsZero() {
		o.IssuedAt = time.Now()
	}
	if o.SignedAt.IsZero() {
		o.SignedAt = o.IssuedAt
	}
	var p gopdf.GoPdf
	p.Start(gopdf.Config{PageSize: gopdf.Rect{W: Width, H: Height}})
	for _, f := range []struct{ name, file string }{{"NotoSans", "NotoSans-Regular.ttf"}, {"NotoSans-Bold", "NotoSans-Bold.ttf"}} {
		b, e := assets.ReadFile("assets/" + f.file)
		if e != nil {
			return nil, e
		}
		if e = p.AddTTFFontData(f.name, b); e != nil {
			return nil, e
		}
	}
	width := Width - 2*Margin
	font := func(size float64, bold bool) error {
		name := "NotoSans"
		if bold {
			name = "NotoSans-Bold"
		}
		return p.SetFont(name, "", size)
	}
	wrap := func(text string, size float64, bold bool, w float64) ([]string, error) {
		if e := font(size, bold); e != nil {
			return nil, e
		}
		text = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\t", "    ")
		var out []string
		for _, para := range strings.Split(text, "\n") {
			if para == "" {
				out = append(out, "")
				continue
			}
			for _, r := range para {
				ok, e := p.IsCurrFontContainGlyph(r)
				if e != nil || !ok {
					return nil, fmt.Errorf("carácter no disponible en Noto Sans: %U", r)
				}
			}
			rows, e := p.SplitTextWithWordWrap(para, w)
			if e != nil {
				return nil, e
			}
			out = append(out, rows...)
		}
		return out, nil
	}
	text := func(x, y, size float64, s string, bold bool) error {
		if s == "" {
			return nil
		}
		if e := font(size, bold); e != nil {
			return e
		}
		p.SetXY(x, y)
		return p.Cell(nil, s)
	}
	var content []line
	add := func(s string, size float64, bold bool) error {
		rows, e := wrap(s, size, bold, width)
		if e != nil {
			return e
		}
		for _, r := range rows {
			content = append(content, line{r, size, size * 1.4, bold})
		}
		return nil
	}
	if e := add(strings.TrimSpace(in.Content), 11, false); e != nil {
		return nil, e
	}
	if strings.TrimSpace(in.Diagnosis) != "" {
		content = append(content, line{height: 10})
		if e := add("Diagnóstico", 11, true); e != nil {
			return nil, e
		}
		if e := add(in.Diagnosis, 11, false); e != nil {
			return nil, e
		}
	}
	if in.Kind == "certificado" && strings.TrimSpace(o.Format.CertificateClosing) != "" {
		content = append(content, line{height: 12})
		if e := add(o.Format.CertificateClosing, 11, false); e != nil {
			return nil, e
		}
	}
	logo, e := loadImage(o.Format.LogoPath)
	if e != nil {
		return nil, fmt.Errorf("logo: %w", e)
	}
	hand, e := loadImage(o.Format.SignatureImagePath)
	if e != nil {
		return nil, fmt.Errorf("firma manuscrita: %w", e)
	}
	patient, e := wrap("Paciente: "+in.Patient, 11, true, width)
	if e != nil {
		return nil, e
	}
	name, e := wrap(o.Format.Professional.Name, 11, true, width)
	if e != nil {
		return nil, e
	}
	profession, e := wrap(o.Format.Professional.Profession, 10, false, width)
	if e != nil {
		return nil, e
	}
	registration, e := wrap("Registro Superintendencia de Salud N.º "+o.Format.Professional.Registration, 9, false, width)
	if e != nil {
		return nil, e
	}
	signatureHeight := float64(len(name))*15 + float64(len(profession))*14 + float64(len(registration))*13 + 16 + 12 + 55
	if hand != nil {
		signatureHeight += 48
	}
	firstTop := Margin + 146 + float64(len(patient))*16
	pages := []sheet{{start: firstTop}}
	cursor := firstTop
	for _, l := range content {
		if cursor+l.height > bodyBottom {
			pages = append(pages, sheet{start: continuationTop})
			cursor = continuationTop
		}
		last := len(pages) - 1
		pages[last].lines = append(pages[last].lines, l)
		cursor += l.height
	}
	if cursor+signatureHeight+12 > bodyBottom {
		last := len(pages) - 1
		rows := pages[last].lines
		n := len(rows)
		count := 0
		split := n
		// Carry at least two nonempty content lines with the complete signature.
		for split > 0 && count < 2 {
			split--
			if strings.TrimSpace(rows[split].text) != "" {
				count++
			}
		}
		carry := append([]line(nil), rows[split:]...)
		pages[last].lines = rows[:split]
		if split == 0 && last > 0 {
			pages = pages[:last]
		}
		pages = append(pages, sheet{start: continuationTop, lines: carry})
	}
	lastPage := len(pages) - 1
	lastY := pages[lastPage].start
	for _, l := range pages[lastPage].lines {
		lastY += l.height
	}
	if lastY+signatureHeight+12 > bodyBottom {
		return nil, errors.New("signature block cannot fit with content")
	}
	qr, e := qrcode.Encode(o.VerifyURL, qrcode.Medium, 320)
	if e != nil {
		return nil, e
	}
	qrImage, e := gopdf.ImageHolderByBytes(qr)
	if e != nil {
		return nil, e
	}
	title := map[string]string{"certificado": "CERTIFICADO", "evolucion": "EVOLUCIÓN CLÍNICA", "epicrisis": "EPICRISIS"}[in.Kind]
	contact, e := wrap(o.Format.Professional.Contact, 7.2, false, width)
	if e != nil {
		return nil, e
	}
	if len(contact) > 2 {
		return nil, errors.New("professional contact exceeds two footer lines")
	}
	for i, page := range pages {
		p.AddPage()
		p.SetTextColor(31, 50, 61)
		p.SetStrokeColor(184, 199, 202)
		p.SetLineWidth(.5)
		dx := Margin
		if logo != nil {
			if e = drawImage(&p, logo, Margin, Margin, 58, 40); e != nil {
				return nil, e
			}
			dx += 72
		}
		if e = text(dx, Margin+13, 9, LongDate(o.Format.City, o.IssuedAt), false); e != nil {
			return nil, e
		}
		p.Line(Margin, Margin+52, Width-Margin, Margin+52)
		if i == 0 {
			if e = font(16, true); e != nil {
				return nil, e
			}
			tw, _ := p.MeasureTextWidth(title)
			if e = text((Width-tw)/2, Margin+69, 16, title, true); e != nil {
				return nil, e
			}
			if e = text(Margin, Margin+99, 9, "Folio: "+in.Folio, false); e != nil {
				return nil, e
			}
			y := Margin + 118
			for _, s := range patient {
				if e = text(Margin, y, 11, s, true); e != nil {
					return nil, e
				}
				y += 16
			}
			if e = text(Margin, y, 10, "RUT: "+in.RUT, false); e != nil {
				return nil, e
			}
		}
		y := page.start
		for _, l := range page.lines {
			if e = text(Margin, y, l.size, l.text, l.bold); e != nil {
				return nil, e
			}
			y += l.height
		}
		if i == lastPage {
			y += 12
			if hand != nil {
				if e = drawImage(&p, hand, Margin, y, 130, 42); e != nil {
					return nil, e
				}
				y += 48
			}
			for _, s := range name {
				if e = text(Margin, y, 11, s, true); e != nil {
					return nil, e
				}
				y += 15
			}
			if e = text(Margin, y, 9, "RUT: "+o.Format.Professional.RUT, false); e != nil {
				return nil, e
			}
			y += 16
			for _, s := range profession {
				if e = text(Margin, y, 10, s, false); e != nil {
					return nil, e
				}
				y += 14
			}
			for _, s := range registration {
				if e = text(Margin, y, 9, s, false); e != nil {
					return nil, e
				}
				y += 13
			}
			y += 12
			p.RectFromUpperLeft(Margin, y, width, 55)
			if e = text(Margin+10, y+7, 9, "Documento firmado electrónicamente", true); e != nil {
				return nil, e
			}
			if e = text(Margin+10, y+24, 8, "Fecha y hora de firma: "+Santiago(o.SignedAt).Format("02/01/2006 15:04:05 -07:00"), false); e != nil {
				return nil, e
			}
			if e = text(Margin+10, y+39, 8, "Tipo de firma: simple · Certificado X.509 propio", false); e != nil {
				return nil, e
			}
		}
		bottom := Height - Margin
		fy := bottom - 108
		p.Line(Margin, fy-8, Width-Margin, fy-8)
		if e = p.ImageByHolder(qrImage, Margin, fy, &gopdf.Rect{W: 56, H: 56}); e != nil {
			return nil, e
		}
		if e = text(Margin+68, fy+1, 8.5, in.Folio, false); e != nil {
			return nil, e
		}
		qrows, e := wrap("Verifique la autenticidad de este documento escaneando el código QR.", 8, false, width-68)
		if e != nil {
			return nil, e
		}
		for j, s := range qrows {
			if e = text(Margin+68, fy+17+float64(j)*11, 8, s, false); e != nil {
				return nil, e
			}
		}
		if e = text(Margin+68, fy+44, 7.2, "Huella del contenido (SHA-256): "+ContentHash(in)[:16], false); e != nil {
			return nil, e
		}
		if e = text(Margin, bottom-48, 7.1, "Documento confidencial. Contiene información clínica protegida por la Ley 20.584.", false); e != nil {
			return nil, e
		}
		for j, s := range contact {
			if e = text(Margin, bottom-34+float64(j)*10, 7.2, s, false); e != nil {
				return nil, e
			}
		}
		pagination := fmt.Sprintf("Página %d de %d", i+1, len(pages))
		if e = font(7.5, false); e != nil {
			return nil, e
		}
		tw, _ := p.MeasureTextWidth(pagination)
		if e = text(Width-Margin-tw, bottom-10, 7.5, pagination, false); e != nil {
			return nil, e
		}
	}
	var out bytes.Buffer
	if _, e = p.WriteTo(&out); e != nil {
		return nil, e
	}
	return archival(out.Bytes(), o.IssuedAt)
}
