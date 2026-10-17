# Instalar Certplum

Certplum es un servicio independiente de Medplum. Cada instancia es de un solo
usuario/profesional. No compartir directorios, bases, usuarios o puertos con
Medplum. Consultarlo únicamente por API FHIR con credenciales de lectura.

## Windows: ejecutable portátil

`certplum_0.1.0_windows_amd64.exe` es la aplicación compilada. No es un asistente
de instalación y no modifica servicios, registro de Windows ni firewall.

1. Crear una carpeta privada de trabajo y copiar el EXE.
2. Copiar `configs/config.example.json` como `config.local.json`, reemplazar los
   datos de ejemplo, usar un directorio de datos exclusivo y un puerto libre.
3. Crear/custodiar un certificado propio y clave PKCS#8, según el README.
4. Abrir PowerShell en esa carpeta:

```powershell
.\certplum_0.1.0_windows_amd64.exe version
.\certplum_0.1.0_windows_amd64.exe setup -config config.local.json -cert signer.crt -key signer.enc.key
.\certplum_0.1.0_windows_amd64.exe serve -config config.local.json -tls-cert https.crt -tls-key https.key
```

La última línea utiliza un certificado HTTPS local distinto del certificado de
firma. Abrir en el navegador el `origin` configurado. No hay modo HTTP inseguro.
No instalar el EXE en un equipo compartido sin revisar permisos de sus carpetas.

## Ubuntu/Debian: paquete DEB

Elegir `amd64` para x86-64 o `arm64` para ARM64. Verificar primero SHA-256 con
`certplum_0.1.0_SHA256SUMS.txt`. Los paquetes no tienen firma de distribución.

**Los siguientes comandos son para el administrador que haya decidido instalar
Certplum. En un servidor compartido, aprobar previamente cualquier instalación
y cambio de servicio/proxy/firewall.** No se ejecutan por descargar el paquete.

```sh
sudo dpkg -i certplum_0.1.0_amd64.deb
certplum version
```

El paquete copia únicamente archivos propios en `/usr/bin/certplum`,
`/usr/share/certplum`, `/usr/share/doc/certplum` y una unidad nueva
`/usr/lib/systemd/system/certplum.service`. No contiene scripts de instalación
o desinstalación, no crea usuarios/base/configuración, no arranca ni habilita
servicios, y no escribe en proxy, firewall o Medplum. No instala dependencias.

Para terminar la configuración, revisar que los nombres/rutas no pertenezcan
a otra aplicación. Solo entonces, con autorización del administrador:

```sh
sudo useradd --system --user-group --home-dir /var/lib/certplum --shell /usr/sbin/nologin certplum
sudo install -d -o certplum -g certplum -m 0700 /etc/certplum /var/lib/certplum
sudo install -o certplum -g certplum -m 0600 /usr/share/certplum/config.example.json /etc/certplum/config.json
```

Editar **solo** `/etc/certplum/config.json`: identidad profesional propia,
ciudad/contacto, dominio HTTPS, puerto libre y datos exclusivos. Los valores de
ejemplo son ficticios y deben reemplazarse. El JSON no lleva claves ni tokens.

Los archivos cifrados de importación deben ser legibles por el usuario exclusivo
durante el alta, sin abrir sus permisos a todo el sistema:

```sh
sudo -u certplum certplum setup -config /etc/certplum/config.json -cert /RUTA/PRIVADA/signer.crt -key /RUTA/PRIVADA/signer.enc.key
sudo systemd-analyze verify /usr/lib/systemd/system/certplum.service
sudo systemctl daemon-reload
sudo systemctl enable --now certplum.service
```

No repetir estos comandos si el usuario/directorios ya existen sin investigar
su pertenencia. La unidad empieza con la bóveda bloqueada. No toca Medplum.

HTTPS externo requiere un sitio nuevo separado. Los ejemplos del paquete se
encuentran en `/usr/share/certplum/`; no se aplican solos. Si el proxy es
compartido, revisar/aprobar el archivo final y la recarga antes de ejecutarlos.
No editar el sitio existente de Medplum ni sus configuraciones globales.

TLS y TSA necesitan un almacén CA confiable. Usar el del sistema o gestionar
su instalación por separado, con autorización; no desactivar la validación.

## Actualizar o desinstalar

Hacer respaldo antes de actualizar. Detener **solo Certplum**, revisar y
autorizar la actualización, instalar el DEB nuevo y volver a iniciar Certplum.
El paquete no reinicia servicios automáticamente. Nunca reiniciar Medplum.

Para retirar Certplum, detener/deshabilitar solo su unidad antes de desinstalar
el paquete. `/var/lib/certplum` y `/etc/certplum` se crean manualmente y no son
propiedad del paquete: su contenido se conserva. No borrar datos, respaldos o
claves sin inventario y autorización explícita.

## Alcance de las pruebas

Se construyeron los binarios y paquetes desde Windows; se revisó la estructura
DEB, rutas, permisos y ausencia de scripts. No equivale a una instalación
verificada en Ubuntu. Probar primero en una máquina/VM propia y desechable.
