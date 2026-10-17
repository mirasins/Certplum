# Arquitectura

Un binario Go contiene la interfaz HTML/JS (`embed`), servidor, generación PDF,
firma PAdES y SQLite. No necesita Docker, Redis, Node, Java, Chromium o Medplum
para ejecutar el servicio. Medplum es una fuente opcional de pacientes.

| Componente | Responsabilidad |
|---|---|
| `cmd/certplum` | setup, serve, backup, restore, version |
| `internal/auth` | Contraseña, TOTP, limitación y sesiones |
| `internal/vault` | Argon2id y AES-256-GCM de bibliotecas establecidas |
| `internal/store` | SQLite exclusivo, folios y auditoría transaccional |
| `internal/pdf` | Carta, Noto Sans completa, QR, imágenes y PDF/A-2b |
| `internal/signing` | LocalSigner PAdES y ProviderSigner no implementado |
| `internal/medplum` | Token OAuth y búsqueda Patient mediante GET |
| `internal/backup` | Instantánea consistente y exportación cifrada |

Emisión: validar datos/RUT → reservar folio por tipo/año → QR → paginar en
memoria → fuentes completas/ICC/XMP PDF/A-2b → firma PAdES → verificar cobertura
e integridad → SHA-256 final → cifrar → guardar con auditoría. No modificar
bytes después de firmar. La revocación cambia el registro, no el PDF.

Los metadatos públicos mínimos (ID aleatorio, fecha, hash y estado) se mantienen
legibles para verificar con la bóveda bloqueada. El contenido clínico, PDF,
TOTP, credenciales Medplum y clave de firma se cifran antes de SQLite/WAL.
La exportación de respaldo añade cifrado del archivo completo.

SQLite tiene marcador de aplicación propio; se rechaza una base ajena antes
de abrirla con SQL. Sincronización y escritura FHIR no se implementan.

La sesión única se bloquea por inactividad y como máximo a las 12 horas.
Las claves solo se conservan en memoria desbloqueada; se limpian referencias
y buffers al bloquear/cerrar. Go/navegador/SO pueden conservar copias transitorias:
no se garantiza borrado físico de RAM. Revisar swap y volcados del despliegue.

PDF/A-2b conserva representación visual, no la validez criptográfica para siempre.
No hay LTV B-LT/B-LTA ni firma avanzada acreditada. El stub de prestador rechaza
su uso y nunca degrada silenciosamente a firma simple.
