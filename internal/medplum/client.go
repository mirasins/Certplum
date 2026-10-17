// Package medplum contains only token acquisition and read-only Patient search.
package medplum

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	ID      string
	Secret  string
	HTTP    *http.Client
}
type Patient struct {
	Name string `json:"name"`
	RUT  string `json:"rut"`
}

func New(base, id, secret string) *Client {
	return &Client{base, id, secret, &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func decode(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("Medplum no disponible o sin permiso de lectura")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if e != nil || len(b) > 2*1024*1024 {
		return errors.New("respuesta Medplum demasiado grande")
	}
	return json.Unmarshal(b, dst)
}
func (c *Client) Search(ctx context.Context, query string) ([]Patient, error) {
	if c.BaseURL == "" || c.ID == "" || c.Secret == "" {
		return nil, errors.New("Medplum no configurado; usa el ingreso manual")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	base := strings.TrimRight(c.BaseURL, "/")
	form := url.Values{"grant_type": {"client_credentials"}}
	req, e := http.NewRequestWithContext(ctx, "POST", base+"/oauth2/token", strings.NewReader(form.Encode()))
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(c.ID, c.Secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, errors.New("Medplum no disponible; usa el ingreso manual")
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if e = decode(resp, &token); e != nil {
		return nil, e
	}
	if token.AccessToken == "" {
		return nil, errors.New("token Medplum inválido")
	}
	values := url.Values{"_count": {"20"}, "_elements": {"name,identifier"}}
	query = strings.TrimSpace(query)
	if len(query) < 2 || len(query) > 100 {
		return nil, errors.New("búsqueda inválida")
	}
	isID := true
	for _, r := range strings.ToUpper(query) {
		if !(r >= '0' && r <= '9' || r == 'K' || r == '.' || r == '-') {
			isID = false
		}
	}
	if isID {
		values.Set("identifier", query)
	} else {
		values.Set("name", query)
	}
	req, e = http.NewRequestWithContext(ctx, "GET", base+"/fhir/R4/Patient?"+values.Encode(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/fhir+json")
	resp, e = c.HTTP.Do(req)
	if e != nil {
		return nil, errors.New("Medplum no disponible; usa el ingreso manual")
	}
	var bundle struct {
		Entry []struct {
			Resource struct {
				Name []struct {
					Text   string   `json:"text"`
					Given  []string `json:"given"`
					Family string   `json:"family"`
				} `json:"name"`
				Identifier []struct {
					System string `json:"system"`
					Value  string `json:"value"`
				} `json:"identifier"`
			} `json:"resource"`
		} `json:"entry"`
	}
	if e = decode(resp, &bundle); e != nil {
		return nil, e
	}
	out := []Patient{}
	for _, entry := range bundle.Entry {
		p := Patient{}
		if len(entry.Resource.Name) > 0 {
			n := entry.Resource.Name[0]
			p.Name = n.Text
			if p.Name == "" {
				p.Name = strings.TrimSpace(strings.Join(n.Given, " ") + " " + n.Family)
			}
		}
		for _, id := range entry.Resource.Identifier {
			if strings.Contains(strings.ToLower(id.System), "rut") {
				p.RUT = id.Value
				break
			}
		}
		out = append(out, p)
	}
	return out, nil
}
