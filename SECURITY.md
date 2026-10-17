# Seguridad

Certplum maneja información clínica y claves de firma. No adjuntar datos reales,
PDFs clínicos, bases SQLite, respaldos, certificados privados, TOTP o contraseñas
a issues, pull requests o conversaciones públicas.

Para una vulnerabilidad, usar la función privada **Report a vulnerability** de
GitHub si está habilitada. Si no lo está, abrir un issue sin detalles técnicos
sensibles solicitando un canal privado; no publicar pruebas con datos reales.

Proyecto comunitario inicial. Las pruebas no equivalen a una auditoría externa
ni a una certificación legal/PAdES. Antes de uso clínico, evaluar el despliegue,
la normativa aplicable, los permisos y la custodia de credenciales.

Medplum solo se consulta por API con un cliente dedicado de mínimos permisos.
Nunca instalar este proyecto sobre sus directorios, puertos o bases de datos.
