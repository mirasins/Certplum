package pdf

import (
	"bytes"
	"encoding/xml"
	"errors"
	"github.com/signintech/gopdf"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type picture struct {
	data []byte
	w, h int
}

func loadImage(path string) (*picture, error) {
	if path == "" {
		return nil, nil
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 4*1024*1024+1))
	if e != nil || len(b) > 4*1024*1024 {
		return nil, errors.New("image exceeds 4 MiB")
	}
	var img image.Image
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		decoder := xml.NewDecoder(bytes.NewReader(b))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if d, ok := token.(xml.Directive); ok && strings.Contains(strings.ToUpper(string(d)), "DOCTYPE") {
				return nil, errors.New("SVG document types forbidden")
			}
			if el, ok := token.(xml.StartElement); ok {
				switch strings.ToLower(el.Name.Local) {
				case "script", "image", "text", "foreignobject", "use", "style", "filter":
					return nil, errors.New("SVG must use paths/shapes without scripts, text, linked content or filters; export as PNG or outlined SVG")
				}
				for _, a := range el.Attr {
					if a.Name.Local == "xmlns" || a.Name.Space == "xmlns" {
						continue
					}
					v := strings.ToLower(a.Value)
					if strings.Contains(v, "http:") || strings.Contains(v, "https:") || strings.Contains(v, "file:") || strings.HasPrefix(strings.ToLower(a.Name.Local), "on") {
						return nil, errors.New("external SVG references forbidden")
					}
				}
			}
		}
		icon, err := oksvg.ReadIconStream(bytes.NewReader(b), oksvg.StrictErrorMode)
		if err != nil {
			return nil, err
		}
		vb := icon.ViewBox
		if vb.W <= 0 || vb.H <= 0 {
			return nil, errors.New("invalid SVG viewBox")
		}
		w, h := 1000, 1000
		ratio := vb.W / vb.H
		if ratio >= 1 {
			h = int(1000 / ratio)
		} else {
			w = int(1000 * ratio)
		}
		if w < 1 || h < 1 {
			return nil, errors.New("invalid SVG aspect ratio")
		}
		canvas := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
		icon.SetTarget(0, 0, float64(w), float64(h))
		icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, canvas, canvas.Bounds())), 1)
		img = canvas
	case ".png":
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4000 || cfg.Height > 4000 {
			return nil, errors.New("PNG dimensions exceed 4000 px")
		}
		img, e = png.Decode(bytes.NewReader(b))
		if e != nil {
			return nil, e
		}
	default:
		return nil, errors.New("only PNG or SVG images accepted")
	}
	// Normalize to opaque sRGB-compatible RGB before embedding; no inherited image metadata.
	bounds := img.Bounds()
	opaque := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(opaque, opaque.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), img, bounds.Min, draw.Over)
	var out bytes.Buffer
	if e = png.Encode(&out, opaque); e != nil {
		return nil, e
	}
	return &picture{out.Bytes(), bounds.Dx(), bounds.Dy()}, nil
}
func drawImage(p *gopdf.GoPdf, img *picture, x, y, w, h float64) error {
	scale := w / float64(img.w)
	if float64(img.h)*scale > h {
		scale = h / float64(img.h)
	}
	iw, ih := float64(img.w)*scale, float64(img.h)*scale
	holder, e := gopdf.ImageHolderByBytes(img.data)
	if e != nil {
		return e
	}
	return p.ImageByHolder(holder, x, y+(h-ih)/2, &gopdf.Rect{W: iw, H: ih})
}
