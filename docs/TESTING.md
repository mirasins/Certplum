# Pruebas

Requiere Go 1.27.2 o una versión posterior con parches:

```sh
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Las pruebas generan claves/certificados ficticios en memoria y usan directorios
temporales. Nunca usar credenciales/pacientes reales.

Cobertura: AES-GCM/Argon2id, manipulación/AAD/nonces, autenticación y TOTP,
bloqueo/CSRF/cookies/HTTPS, emisión/descarga/revocación, firma PAdES, Medplum
simulado solo lectura, respaldos con WAL, folios concurrentes, SVG externo
rechazado y fuentes completas. El contador de pruebas incluye subcasos.

OpenSSL verifica CMS de forma independiente y rechaza modificaciones. Si no
está en PATH, indicar `CLINICAL_OPENSSL`; sin él esa comprobación se omite.
Poppler comprueba texto/márgenes/firma sin página aislada: indicar
`CLINICAL_PDFTOTEXT` si no está en PATH. Sin Poppler, esa prueba se omite.

Para revisar PDF/A-2b, exportar muestras ficticias con `CLINICAL_FORMAT_DIR`
y validar los archivos **ya firmados** mediante veraPDF (`--flavour 2b`).
Comprobar `isCompliant="true"`; no basta el código de salida. Renderizar y
revisar las páginas también: PDF/A no detecta todos los defectos visuales.

El formato fuente se validó con veraPDF 1.30.3 y Poppler 26.09.0. Las fuentes
completas corrigen la pérdida visual de tildes de subconjuntos anidados.

La prueba opcional `scripts/check-ui.mjs` usa Playwright y Chrome contra un
servidor HTTPS temporal con datos ficticios. No es una dependencia del binario.
Consultar sus variables `CLINICAL_GO` y `CLINICAL_CHROME`, e instalar Playwright
en `.tools/ui` antes de ejecutarla. No apuntarla a pacientes reales.

El análisis de vulnerabilidades identificó cero rutas usadas afectadas en la
versión de Go comprobada. Puede aparecer una advertencia del módulo x/crypto
por OpenPGP, que esta aplicación no importa. Los resultados pueden cambiar:
repetir el análisis antes de cada distribución. No equivale a auditoría externa.
