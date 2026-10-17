package medplum

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOnlyReadFHIRAndToken(t *testing.T) {
	calls := 0
	c := New("https://medplum.example", "client-id", "client-secret")
	c.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := ""
		switch r.URL.Path {
		case "/oauth2/token":
			if r.Method != "POST" {
				t.Fatal("wrong OAuth method")
			}
			id, secret, ok := r.BasicAuth()
			if !ok || id != c.ID || secret != c.Secret {
				t.Fatal("invalid token auth")
			}
			body = `{"access_token":"test-token"}`
		case "/fhir/R4/Patient":
			if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-token" || r.URL.Query().Get("name") != "Persona" {
				t.Fatal("FHIR request not read only")
			}
			body = `{"entry":[{"resource":{"name":[{"given":["Persona"],"family":"Ficticia"}],"identifier":[{"system":"https://example.cl/rut","value":"12345678-5"}]}}]}`
		default:
			t.Fatal("unexpected endpoint")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	patients, e := c.Search(context.Background(), "Persona")
	if e != nil || calls != 2 || len(patients) != 1 || patients[0].RUT != "12345678-5" {
		t.Fatalf("search: %v %#v", e, patients)
	}
}
func TestDisabledAndForbidden(t *testing.T) {
	if _, e := New("", "", "").Search(context.Background(), "Prueba"); e == nil {
		t.Fatal("disabled integration accepted")
	}
	c := New("https://medplum.example", "id", "secret")
	c.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("sensitive details"))}, nil
	})
	if _, e := c.Search(context.Background(), "Prueba"); e == nil || strings.Contains(e.Error(), "sensitive") {
		t.Fatal("forbidden response not sanitized")
	}
}
