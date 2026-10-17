# Certplum

**Documentos clínicos PDF/A-2b con firma local PAdES y una integración opcional
de solo lectura con Medplum.** Aplicación independiente para un profesional
por instancia, escrita en Go con HTML/JS incorporados y SQLite propio.

Proyecto comunitario inicial, distribuido bajo MIT. No es un producto oficial
de Medplum ni está afiliado o certificado por Medplum. La primera versión
conserva formatos chilenos (RUT y aviso de confidencialidad); no declara soporte
para todos los identificadores o legislaciones.

## Qué hace

- Certificados, evoluciones y epicrisis en carta, con datos profesionales
  configurables, Noto Sans embebida, folios, QR y paginación.
- Firma electrónica simple con certificado X.509 propio, PAdES B-B y TSA
  RFC 3161 opcional para B-T. El prestador de firma avanzada es solo un stub.
- AES-256-GCM antes de escribir contenido clínico, PDF y secretos en SQLite/WAL.
  Clave maestra derivada con Argon2id de una frase que no se guarda en disco.
- Contraseña + TOTP, bloqueo por inactividad, límites de intentos, HTTPS,
  cookies protegidas y CSRF.
- Historial, descarga, revocación y auditoría sin texto clínico.
- Verificación pública con ID aleatorio, hash del PDF final, fecha y estado.
  Comparación del archivo en el navegador sin subirlo.
- Respaldo consistente con WAL y cifrado adicional de la base completa.

## Aislamiento de Medplum

Certplum usa binario, usuario, unidad systemd, puerto, directorios y base propios.
No modifica ni comparte contenedores, Docker Compose, Redis, variables, usuarios
o archivos de Medplum. Solo obtiene un token OAuth y busca Patient mediante GET.
El ingreso manual funciona sin Medplum. No hay sincronización ni escritura FHIR.

Los ejemplos de Nginx/Caddy son sitios separados que no se aplican solos.
Antes de modificar un servidor compartido, aprobar explícitamente instalaciones,
recargas, puertos, firewall o proxy. No reiniciar ni reconfigurar Medplum.

## Descargas e instalación

- **Windows amd64:** `.exe` portátil, sin asistente ni instalación de servicios.
- **Ubuntu/Debian amd64 o arm64:** `.deb` con archivos propios. No configura,
  inicia o habilita el servicio, crea usuarios, toca proxy/firewall ni instala
  dependencias. La configuración y el alta son pasos explícitos posteriores.
- `SHA256SUMS`: comprobar integridad antes de ejecutar/instalar. Los binarios
  no llevan firma de editor ni el DEB una firma de repositorio APT.

Ver [instalación Windows y Ubuntu/Debian](docs/INSTALL.md). Los paquetes se
construyen con `scripts/release.mjs`; el workflow **Build packages** genera
artefactos descargables cuando lo ejecuta un mantenedor. Tener un workflow no
significa que exista una versión publicada en Releases.

## Compilar

Go **1.27.2 o posterior con parches**. No necesita Node, Java, Chromium, Docker
o un motor PDF externo para ejecutar el servicio.

```sh
go mod download
go test ./...
go vet ./...
go build -trimpath -buildvcs=false -o build/certplum ./cmd/certplum
```

En Windows usar salida `build/certplum.exe`. Para generar los tres paquetes,
se necesitan Node y nFPM **solo en la máquina de construcción**:

```sh
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0
node scripts/release.mjs 0.1.0
```

Genera DEB amd64/arm64, EXE Windows amd64 y sumas SHA-256 en `dist/`. Las variables
`CERTPLUM_GO`/`CERTPLUM_NFPM` permiten indicar herramientas portátiles.

## Configuración y certificado

Copiar `configs/config.example.json` a un JSON privado fuera de Git. Sustituir
la identidad ficticia, ciudad, contacto, rutas, dominio HTTPS y puerto. Usar un
directorio de datos exclusivo. No incluir contraseñas ni claves en el JSON.

Para un certificado propio, fuera del repositorio y con permisos privados:

```sh
umask 077
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -aes-256-cbc -out signer.enc.key
openssl req -new -x509 -sha256 -days 365 -key signer.enc.key -out signer.crt -subj "/CN=NOMBRE DEL TITULAR" -addext "keyUsage=critical,digitalSignature,nonRepudiation" -addext "extendedKeyUsage=1.3.6.1.5.5.7.3.36"
certplum setup -config config.local.json -cert signer.crt -key signer.enc.key
```

PKCS#8 cifrado se descifra en memoria durante la importación. El nombre impreso
y de firma se leen del JSON y deben corresponder al titular del certificado.
Un certificado autofirmado no equivale a firma avanzada acreditada; quien
recibe el PDF debe confiar en él por un canal independiente.

El alta pide contraseña (mínimo 12 bytes), frase diferente (mínimo 20), TOTP y,
si se habilita Medplum, ID/secreto del cliente. Solo terminal interactiva; no
pasar secretos por argumentos, variables, logs o issues. El URI de enrolamiento
TOTP se muestra durante el alta: custodiarlo y no grabar esa terminal.

Perder la frase implica perder documentos, clave y respaldos. No hay bypass
de TOTP ni recuperación alternativa. Cinco fallos bloquean 15 minutos incluso
tras reinicio. Un paso TOTP consumido no se reutiliza. Sesión máxima 12 horas.

## Medplum opcional

Configurar `medplum_url` con el origen HTTPS de su API, sin `/fhir/R4`. Vacío
mantiene la app completamente manual. Crear en Medplum un ClientApplication
dedicado con permiso mínimo de lectura/búsqueda Patient (`name`, `identifier`),
sin rol administrativo ni escritura; el propietario realiza esa configuración.
No reutilizar credenciales administrativas.

El backend hace POST `/oauth2/token` (`client_credentials` y HTTP Basic) y GET
`/fhir/R4/Patient`; sin redirecciones y con plazos máximos. Hasta 20 coincidencias.
Identificadores con `system` que contenga `rut` se interpretan como RUT; otros
se dejan para revisión manual. El formato de búsqueda depende del servidor.

Referencias oficiales: [Client credentials](https://www.medplum.com/docs/auth/client-credentials),
[Access policies](https://www.medplum.com/docs/access/access-policies).

## HTTPS

En producción: `certplum serve -config /etc/certplum/config.json`, detrás de
un proxy con sitio HTTPS propio. El backend solo acepta loopback y Host/origen
exactos; confía en el encabezado HTTPS del proxy únicamente desde loopback.

Para pruebas directas, usar otro certificado exclusivamente de transporte:

```sh
mkdir -p .local
openssl req -x509 -newkey rsa:3072 -nodes -keyout .local/https.key -out .local/https.crt -days 7 -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
certplum serve -config config.local.json -tls-cert .local/https.crt -tls-key .local/https.key
```

El origen local debe ser, por ejemplo, `https://localhost:18765`, con puerto
libre. Este certificado HTTPS es distinto de la clave de firma clínica.

## Respaldar y restaurar

```sh
certplum backup -config config.local.json -out /RUTA/PRIVADA/copia.backup
certplum restore -config /RUTA/PRIVADA/config-destino-vacio.json -in /RUTA/PRIVADA/copia.backup
```

La instantánea consistente incluye el WAL confirmado; no copiar SQLite activo
con `cp`. El archivo completo se cifra con sal/nonce nuevos y autentica su
cabecera. No contiene una clave para recuperarlo sin la frase. Destino nuevo,
sin sobrescritura. Restaurar a un directorio vacío y comprobar antes de servir.
Una copia antigua devuelve contadores y revocaciones a su momento histórico;
revisarlos antes de publicar. No ejecutar una restauración junto a su original.

Máximo actual de exportación: 1 GiB, con varias copias en memoria. Custodiar
respaldos fuera del servidor y ensayar su restauración. El JSON de configuración
(sin secretos) se respalda por separado. La base propia de Certplum tiene un
marcador distinto del prototipo; no copiar bases ajenas dentro de su directorio.

## Límites de seguridad y conservación

Los metadatos técnicos de autenticación y el registro mínimo público/auditoría
quedan legibles, para permitir verificar con la bóveda bloqueada. El respaldo
exportado los cifra también. El pie muestra una huella **del contenido**, pues
el hash del propio PDF final sería autorreferente; el QR consulta el hash final.

Bloquear limpia referencias/buffers e invalida la sesión. Go/navegador/SO pueden
retener copias transitorias: no se promete borrado físico de RAM. Cerrar la página
intenta bloquear; si se pierde esa solicitud actúa la inactividad. Usar swap
cifrada o sin swap y desactivar volcados. No se modifican esos ajustes solos.

Los PDFs descargados son legibles y quedan bajo custodia del usuario. Revocar
cambia el registro público, no el PDF ni el certificado X.509. Auditoría con
triggers no es invulnerable al administrador de la base. Revisar también logs
y buffering del proxy, no solo de la aplicación.

PDF/A-2b conserva la representación visual. No hay garantía de LTV, B-LT/B-LTA,
OCSP/CRL online, cumplimiento normativo universal ni acreditación de firma
avanzada. `pdfsign` está fijado a una versión candidata; evaluar y probar antes
de uso clínico. El stub de prestador falla explícitamente.

## Comunidad

[Arquitectura](docs/ARCHITECTURE.md) · [Formato PDF](docs/PDF-FORMAT.md) ·
[Pruebas](docs/TESTING.md) · [Contribuir](CONTRIBUTING.md) ·
[Seguridad](SECURITY.md) · [Licencia MIT](LICENSE)

No subir datos de pacientes, certificados, claves, bases, respaldos o datos
profesionales reales. Las identidades del ejemplo y las pruebas son ficticias.
Los componentes y activos de terceros conservan sus licencias originales.

