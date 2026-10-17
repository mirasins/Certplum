# Contribuir a Certplum

1. Abrir una propuesta o un pull request pequeño con el comportamiento esperado.
2. Usar exclusivamente pacientes y credenciales ficticios. No subir PDFs, bases,
   claves, configuraciones reales ni rutas personales de desarrollo.
3. Ejecutar `go test ./...` y `go vet ./...`. Para PDF, revisar también render,
   firma independiente con OpenSSL y PDF/A-2b con veraPDF.
4. Conservar el funcionamiento manual cuando Medplum no responda. No introducir
   escrituras FHIR ni cambios en el despliegue de Medplum.
5. No introducir servicios de red, telemetría, claves persistidas sin cifrar o
   una instalación que configure proxy/firewall/inicie servicios por sorpresa.

Las contribuciones se distribuyen bajo la licencia MIT del proyecto. Noto Sans
y el perfil ICC conservan sus propias licencias y avisos.

El primer alcance es un profesional/usuario por instancia, con identificadores
RUT y formatos clínicos chilenos. Otros países/identificadores y varios usuarios
requieren propuestas y validación específicas; no se afirman soportados hoy.
