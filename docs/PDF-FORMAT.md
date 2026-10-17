# Formato PDF y configuración

Certificado, evolución y epicrisis: carta 216 × 279 mm, márgenes de 25 mm,
Noto Sans completa embebida, cuerpo 11 pt, título centrado, folio por tipo/año,
identificación del paciente y contenido editable. El RUT se valida módulo 11.

Ciudad, nombre/RUT/profesión/registro/contacto, logo, imagen manuscrita y cierre
de certificado se leen de la configuración privada. Los valores del ejemplo
son ficticios. Nadie debe emitir documentos usando una identidad de ejemplo.

Logo/firma: PNG local (máximo 4 MiB y 4000 px por lado) o SVG de formas/trazados
sin texto, scripts, recursos externos, filtros o CSS. SVG se rasteriza dentro
de Go; las imágenes se normalizan sobre blanco. Rutas absolutas recomendadas.

El bloque profesional y el recuadro con hora/tipo de firma se mantienen con
contenido en la misma página. Encabezado y pie se repiten. Contacto vacío se
omite. No se crea una firma manuscrita ficticia.

El pie incluye QR, folio, confidencialidad de la plantilla chilena y página X/Y.
Muestra una **huella del contenido**, no el hash del propio PDF firmado: incluir
ese hash en el archivo sería autorreferente. El QR consulta el SHA-256 final
y permite compararlo con un archivo sin subirlo al servidor.

La preparación PDF/A-2b (fuentes/ICC/XMP) precede a la firma PAdES. El SHA-256
final se calcula después de firmar. La descarga/revocación no cambia los bytes.

Los folios reservados no se reutilizan tras errores. Restaurar un respaldo
antiguo devuelve también contadores/revocaciones a su estado histórico: revisar
antes de publicar y no ejecutar dos copias de la misma base simultáneamente.

La primera versión conserva RUT y aviso legal chileno como alcance de las
plantillas. No se presenta como adaptada a todas las legislaciones.
