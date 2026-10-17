package pdf

import (
	"bytes"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"time"
)

func init() { api.DisableConfigDir() }

// archival adds required XMP and an embedded, unmodified ICC output intent.
// It is called only before signing; never rewrite a signed PDF.
func archival(raw []byte, at time.Time) ([]byte, error) {
	cfg := model.NewDefaultConfiguration()
	cfg.WriteObjectStream = false
	cfg.WriteXRefStream = false
	ctx, e := api.ReadContext(bytes.NewReader(raw), cfg)
	if e != nil {
		return nil, e
	}
	catalog, e := ctx.Catalog()
	if e != nil {
		return nil, e
	}
	// Complete fonts preserve nested composite glyphs (Spanish diacritics),
	// which gopdf's subset closure can omit. Original glyph IDs are unchanged.
	for _, entry := range ctx.Table {
		d, ok := entry.Object.(types.Dict)
		if !ok {
			continue
		}
		t := d.NameEntry("Type")
		if t == nil || *t != "FontDescriptor" {
			continue
		}
		name := d.NameEntry("FontName")
		if name == nil {
			continue
		}
		file := ""
		switch *name {
		case "NotoSans":
			file = "NotoSans-Regular.ttf"
		case "NotoSans-Bold":
			file = "NotoSans-Bold.ttf"
		default:
			continue
		}
		data, err := assets.ReadFile("assets/" + file)
		if err != nil {
			return nil, err
		}
		font, err := ctx.NewStreamDictForBuf(data)
		if err != nil {
			return nil, err
		}
		font.InsertInt("Length1", len(data))
		if err = font.Encode(); err != nil {
			return nil, err
		}
		ref, err := ctx.IndRefForNewObject(*font)
		if err != nil {
			return nil, err
		}
		d["FontFile2"] = *ref
	}
	icc, e := assets.ReadFile("assets/sRGB2014.icc")
	if e != nil {
		return nil, e
	}
	sd, e := ctx.NewStreamDictForBuf(icc)
	if e != nil {
		return nil, e
	}
	sd.InsertInt("N", 3)
	if e = sd.Encode(); e != nil {
		return nil, e
	}
	profile, e := ctx.IndRefForNewObject(*sd)
	if e != nil {
		return nil, e
	}
	intent := types.Dict{"Type": types.Name("OutputIntent"), "S": types.Name("GTS_PDFA1"), "OutputConditionIdentifier": types.StringLiteral("sRGB IEC61966-2.1"), "RegistryName": types.StringLiteral("http://www.color.org"), "Info": types.StringLiteral("sRGB2014"), "DestOutputProfile": *profile}
	ref, e := ctx.IndRefForNewObject(intent)
	if e != nil {
		return nil, e
	}
	catalog["OutputIntents"] = types.Array{*ref}
	timestamp := at.UTC().Format(time.RFC3339)
	xmp := fmt.Sprintf(`<?xpacket begin="%s" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/" pdfaid:part="2" pdfaid:conformance="B"/>
<rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:CreateDate="%s" xmp:ModifyDate="%s" xmp:MetadataDate="%s"/>
</rdf:RDF></x:xmpmeta>
<?xpacket end="w"?>`, "\ufeff", timestamp, timestamp, timestamp)
	metadata, e := ctx.NewStreamDictForBuf([]byte(xmp))
	if e != nil {
		return nil, e
	}
	metadata.InsertName("Type", "Metadata")
	metadata.InsertName("Subtype", "XML")
	if e = metadata.Encode(); e != nil {
		return nil, e
	}
	mref, e := ctx.IndRefForNewObject(*metadata)
	if e != nil {
		return nil, e
	}
	catalog["Metadata"] = *mref
	// No clinical content or professional identifiers in document metadata.
	ctx.Info = nil
	var out bytes.Buffer
	if e = api.WriteContext(ctx, &out); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
