package config

import "testing"

func TestIsolationAndHTTPS(t *testing.T) {
	base := Config{Listen: "127.0.0.1:18765", Origin: "https://consulta.example", DataDir: "data", IdleMinutes: 5, Document: DocumentFormat{City: "Santiago", Professional: Professional{Name: "Profesional de Prueba", RUT: "12.345.678-5", Profession: "Psicólogo Clínico", Registration: "PRUEBA"}}}
	if e := base.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, listen := range []string{"0.0.0.0:18765", ":18765", "localhost:18765"} {
		c := base
		c.Listen = listen
		if c.Validate() == nil {
			t.Fatal("nonliteral or public listen accepted")
		}
	}
	for _, origin := range []string{"http://consulta.example", "https://consulta.example/path", "https://user:secret@consulta.example"} {
		c := base
		c.Origin = origin
		if c.Validate() == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
}
