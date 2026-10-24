# Decisiones (ADR)

## ADR-001 · Reproducción directa por HTTP Range (puerta G0) — 23-10-2026

**Contexto.** La fase 0 debía confirmar que AVPlay de la The Serif reproduce y busca dentro de MKV servidos por HTTP antes de construir YourFlix.

**Datos.** `docs/PRUEBAS-TV.md`: 11 archivos (10 sintéticos + 1 MKV real), 3 rondas, 0 cuelgues. MKV/MP4/AVI con H.264, HEVC Main/Main10, XviD y 4K; saltos en archivos de 2,4 y 7,6 GB (posiciones > 4 GB); cambio de audio; SRT recibido como texto. DTS se reproduce **sin sonido** y sin error; PGS no llega.

**Decisión.** Se mantiene el plan: el servidor sirve el archivo tal cual con `http.ServeContent` y **no** transcodifica en vivo. Lo incompatible se optimiza una vez (DTS → E-AC3). Los subtítulos de texto los extrae el servidor y los dibuja la app. Jellyfin y HLS quedan descartados para v1.

**Consecuencias.**
- `compat` debe marcar DTS (y TrueHD, sin probar) como «optimizar»: la TV no avisa.
- La app encola los saltos: un salto durante el rellenado del búfer, o más allá del final, devuelve `PLAYER_ERROR_SEEK_FAILED`.
- Por encima de ~35 Mb/s por Wi-Fi hay riesgo de cortes: aviso en la ficha y servidor con cable cuando se pueda.
- Pendiente: HDR real, servidor en el ROG Ally, archivo con DTS + pista compatible.

## ADR-002 · Servidor en Go con SQLite sin cgo — 23-10-2026

**Decisión.** El servidor se escribe en Go (como recomendaba PLAN.md §3.1 y se usó en la fase 0): un ejecutable por plataforma compilado desde el Mac (`windows/amd64` para el ROG Ally), `modernc.org/sqlite` (sin cgo, permite la compilación cruzada) y `golang.org/x/image` para redimensionar. Sin más dependencias.

**Consecuencias.** La configuración va por opciones y variables de entorno (`--data`/`YOURFLIX_DATA`, `--ffprobe`/`YOURFLIX_FFPROBE`, `--profile`); `yourflix.toml` se añadirá si hace falta (servicio de Windows). Las fuentes viven en la base de datos, no en un fichero. El perfil de la TV va incrustado (`internal/compat`) y un test exige que coincida con `profiles/`.

## ADR-003 · Emparejado de archivos con fichas — 23-10-2026

En cascada: (0) misma huella ya identificada en otra fuente → mismo título (copias); (1) bytes exactos de una copia medida en la semilla, desempatando por duración (±2 s); (2) título normalizado (sin tildes ni signos) + año contra fichas de la semilla/TMDB, también por título original; (3) ficha provisional `filename`, pendiente de TMDB. Las fichas provisionales que se quedan sin archivos se borran; las de la semilla nunca (el disco puede estar desconectado). Las fichas `locked` no las pisa ninguna importación.

## ADR-004 · Identificación con TMDB: prudente antes que completa — 23-10-2026

**Decisión.** TMDB solo se consulta para lo que no identificó la semilla. Se busca en es-ES y en-US (los nombres de archivo vienen en cualquiera de los dos) y cada candidato se puntúa por parecido del título (Dice de bigramas sobre el título español, inglés y original), año (±1 aceptado) y popularidad. Se asigna solo si el mejor es casi idéntico (≥ 0,88), el año es compatible y no hay un rival casi igual. Sin año, dos títulos homónimos exigen que uno tenga 5 veces más votos (Alien 1979 frente a un corto); los nombres de menos de 4 letras sin año exigen ≥ 1000 votos. Lo demás queda **pendiente con sus 5 mejores candidatos** y se reintenta cada 7 días (o con `--retry`).

**Por qué.** Una ficha equivocada (con su portada) es peor que una pendiente: se ve en la tele y nadie la corrige. La primera versión eligió «Parasites» en lugar de «Parásitos» y aceptó «S4 (2008)» para un archivo `s4_dts.mkv`; ambos casos tienen test.

**Consecuencias.** La web de administración (fase 5) debe mostrar los pendientes con sus candidatos para elegir con un clic (hoy: `yourflix titles --pending` y `yourflix match`).

## ADR-005 · El servidor lee copias locales en `C:\YourFlix`, no los discos USB — 24-10-2026

**Contexto.** El servidor corre en el ROG Ally (Wi-Fi hoy, cable más adelante) y la biblioteca vivía en el T7 y el Patriot, ambos por USB, con letras de unidad cambiantes y riesgo de que Windows duerma el disco.

**Decisión.** A petición del usuario se copió el contenido de ambos discos al SSD interno: `C:\YourFlix\T7` (carpetas `pelis` y `pelis2`, 882 GB) y `C:\YourFlix\Patriot` (104 GB). Las fuentes de YourFlix apuntan a esas rutas con las etiquetas «Samsung T7» y «USB Patriot». Los discos originales no se modifican y siguen siendo la copia de seguridad.

**Consecuencias.**
- El escaneo y el emparejado por bytes funcionan igual (460/462 y 34/34 por bytes con la semilla). La huella es de contenido, no de ruta.
- Desaparece el riesgo de disco USB dormido o con otra letra para el uso diario; el soporte «desconectado» queda como función para discos que se añadan a futuro.
- Ocupa 986 GB de los 1,86 TB de C:. Avisar antes de añadir más fuentes grandes, y contar con la carpeta de optimizados (fase 4).
- Para sincronizar con los discos originales hay que repetir la copia con `robocopy` (ver `docs/ENTORNO-ALLY.md`) y re-escanear.
- Dato nuevo para §3.6 del plan: la lectura desde los SSD USB iba a ≈110–200 MB/s en el Ally; la copia local evita ese cuello en servicio.
