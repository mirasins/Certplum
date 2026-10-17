# Componentes de terceros

El código propio de Certplum se distribuye bajo MIT. Los módulos Go conservan
sus licencias originales; consultar sus repositorios y versiones en `go.mod`
y `go.sum`. No se relicencian bajo MIT.

Componentes principales: Go (BSD), pdfcpu (Apache-2.0), gopdf (MIT),
digitorus/pdfsign (BSD-2-Clause), modernc.org/sqlite y sus dependencias,
golang.org/x/crypto, pquerna/otp, go-qrcode, oksvg/rasterx y youmark/pkcs8.

Fuentes Noto Sans: SIL Open Font License 1.1, incluida en
`internal/pdf/assets/OFL.txt`. Perfil sRGB2014: International Color Consortium.
Sus términos, procedencia y hashes están en `internal/pdf/assets/NOTICE.md`.

Las herramientas de desarrollo OpenSSL, Poppler, veraPDF, Playwright y nFPM
no se incluyen como dependencias del servicio ni se instalan en el servidor.
El script de distribución usa nFPM solo para construir el paquete Debian.
