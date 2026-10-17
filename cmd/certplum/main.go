package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"github.com/mirasins/Certplum/internal/auth"
	"github.com/mirasins/Certplum/internal/backup"
	"github.com/mirasins/Certplum/internal/config"
	"github.com/mirasins/Certplum/internal/httpserver"
	"github.com/mirasins/Certplum/internal/signing"
	"github.com/mirasins/Certplum/internal/store"
	"github.com/mirasins/Certplum/internal/vault"
	"github.com/pquerna/otp/totp"
	"github.com/youmark/pkcs8"
	"golang.org/x/term"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "0.1.0"

func secret(label string) ([]byte, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("usa una terminal interactiva; no pases secretos por argumentos ni variables")
	}
	fmt.Fprint(os.Stderr, label+": ")
	b, e := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return b, e
}
func confirmed(label string) ([]byte, error) {
	a, e := secret(label)
	if e != nil {
		return nil, e
	}
	b, e := secret("Repite " + strings.ToLower(label))
	defer vault.Wipe(b)
	if e != nil || string(a) != string(b) {
		vault.Wipe(a)
		return nil, errors.New("las entradas no coinciden")
	}
	return a, nil
}
func run() error {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("Certplum " + version)
		return nil
	}
	if len(os.Args) < 2 {
		return errors.New("uso: certplum setup|serve|backup|restore -config ruta")
	}
	cmd := os.Args[1]
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	conf := f.String("config", "config.local.json", "archivo de configuración")
	cert := f.String("cert", "", "certificado PEM público (setup)")
	keyPath := f.String("key", "", "clave privada PKCS#8 PEM (setup)")
	out := f.String("out", "", "respaldo de destino (backup)")
	in := f.String("in", "", "respaldo de origen (restore)")
	tlsCert := f.String("tls-cert", "", "certificado HTTPS para prueba local directa")
	tlsKey := f.String("tls-key", "", "clave HTTPS para prueba local directa")
	if e := f.Parse(os.Args[2:]); e != nil {
		return e
	}
	c, e := config.Load(*conf)
	if e != nil {
		return e
	}
	if cmd == "restore" {
		if *in == "" {
			return errors.New("indica -in")
		}
		phrase, e := secret("Frase de desbloqueo del respaldo")
		if e != nil {
			return e
		}
		defer vault.Wipe(phrase)
		if e = backup.Restore(*in, c.DataDir, phrase); e != nil {
			return e
		}
		fmt.Println("Respaldo restaurado en el directorio vacío indicado. Servicio sin iniciar.")
		return nil
	}
	s, e := store.Open(c.DataDir)
	if e != nil {
		return e
	}
	defer s.Close()
	switch cmd {
	case "setup":
		if _, e = s.Get("secrets"); e == nil {
			return errors.New("la aplicación ya está inicializada")
		}
		certData, e := os.ReadFile(*cert)
		if e != nil {
			return errors.New("no se pudo leer el certificado indicado")
		}
		keyData, e := os.ReadFile(*keyPath)
		if e != nil {
			return errors.New("no se pudo leer la clave indicada")
		}
		defer vault.Wipe(keyData)
		if block, _ := pem.Decode(keyData); block != nil && block.Type == "ENCRYPTED PRIVATE KEY" {
			pass, err := secret("Contraseña del PEM de firma")
			if err != nil {
				return err
			}
			private, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, pass)
			vault.Wipe(pass)
			if err != nil {
				return errors.New("no fue posible descifrar el PEM de firma")
			}
			der, err := x509.MarshalPKCS8PrivateKey(private)
			if err != nil {
				return err
			}
			vault.Wipe(keyData)
			keyData = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
			vault.Wipe(der)
			defer vault.Wipe(keyData)
		}
		signer, e := signing.Parse(certData, keyData)
		if e != nil {
			return e
		}
		if strings.TrimSpace(signer.Cert.Subject.CommonName) == "" {
			return errors.New("el certificado debe identificar al firmante en Common Name")
		}
		password, e := confirmed("Contraseña de acceso (mínimo 12 caracteres)")
		if e != nil {
			return e
		}
		defer vault.Wipe(password)
		phrase, e := confirmed("Frase de desbloqueo (mínimo 20 caracteres)")
		if e != nil {
			return e
		}
		defer vault.Wipe(phrase)
		otpKey, e := totp.Generate(totp.GenerateOpts{Issuer: "Certplum", AccountName: "consulta-personal"})
		if e != nil {
			return e
		}
		fmt.Println("Enrola esta clave en tu autenticador. Se muestra solo durante el alta local; no la copies al repositorio.")
		fmt.Println(otpKey.URL())
		code, e := secret("Código TOTP para confirmar el enrolamiento")
		if e != nil {
			return e
		}
		valid := totp.Validate(string(code), otpKey.Secret())
		vault.Wipe(code)
		if !valid {
			return errors.New("código TOTP inválido; alta cancelada")
		}
		sec := auth.Secrets{TOTP: otpKey.Secret(), Certificate: certData, PrivateKey: keyData}
		defer sec.Clear()
		if c.MedplumURL != "" {
			id, e := secret("ClientApplication ID de Medplum")
			if e != nil {
				return e
			}
			sec.MedplumID = string(id)
			vault.Wipe(id)
			v, e := secret("Client secret de Medplum")
			if e != nil {
				return e
			}
			sec.MedplumSecret = string(v)
			vault.Wipe(v)
		}
		if e = auth.Setup(s, password, phrase, sec); e != nil {
			return e
		}
		fmt.Println("Alta completada. Certificado y clave de firma cifrados. El PEM original sigue bajo tu resguardo.")
		return nil
	case "backup":
		if *out == "" {
			return errors.New("indica -out (archivo nuevo)")
		}
		phrase, e := secret("Frase de desbloqueo")
		if e != nil {
			return e
		}
		defer vault.Wipe(phrase)
		if e = backup.Export(s, phrase, *out); e != nil {
			return e
		}
		fmt.Println("Respaldo cifrado creado y sincronizado.")
		return nil
	case "serve":
		if _, e = s.Get("secrets"); e != nil {
			return errors.New("ejecuta setup antes de iniciar el servicio")
		}
		if (*tlsCert == "") != (*tlsKey == "") {
			return errors.New("indica ambos archivos HTTPS")
		}
		app := httpserver.New(c, s)
		defer app.Close()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		go app.Watch(ctx)
		srv := &http.Server{Addr: c.Listen, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024, ErrorLog: log.New(discard{}, "", 0)}
		done := make(chan error, 1)
		go func() {
			if *tlsCert != "" {
				done <- srv.ListenAndServeTLS(*tlsCert, *tlsKey)
			} else {
				done <- srv.ListenAndServe()
			}
		}()
		fmt.Println("Servicio iniciado en loopback; bóveda bloqueada.")
		select {
		case <-ctx.Done():
			app.Close()
			shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return srv.Shutdown(shutdown)
		case e := <-done:
			if errors.Is(e, http.ErrServerClosed) {
				return nil
			}
			return e
		}
	default:
		return errors.New("comando desconocido")
	}
}

// Standard HTTP error output is discarded to avoid accidentally logging URLs or payloads.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Error:", e)
		os.Exit(1)
	}
}
