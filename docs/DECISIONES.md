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

## ADR-006 · Remux en vivo solo para el cliente web; la TV sigue con reproducción directa — 24-10-2026

**Contexto.** El usuario quiere ver la colección desde cualquier navegador de casa (iPhone, Android, Mac, PC con Windows) con el mismo servidor del ROG Ally. Medido en la colección real (496 archivos): 480 MKV y 16 AVI; vídeo H.264 8 bits 267, HEVC 8 bits 28, HEVC Main10 175, MPEG-4 ASP/XviD 26; audio AC3 442 pistas, E-AC3 159, AAC 221, MP3 6, DTS 9. Solo 53 archivos se podrían reproducir tal cual en un navegador: Safari no lee MKV, ningún navegador decodifica AC3/E-AC3 en Chrome/Firefox, Firefox no decodifica HEVC y nadie decodifica XviD. ADR-001 decía «sin transcodificación en vivo»; eso sigue valiendo para la TV.

**Decisión.**
- La TV no cambia: archivo tal cual por `/stream` (ADR-001). Las rutas nuevas son añadidas y la TV no las usa.
- El navegador decide en el cliente (`canPlayType` / `MediaSource.isTypeSupported`): **directo** con `/stream` si admite contenedor y códecs de la pista por defecto (Chrome ya abre MKV con H.264 + AAC); si no, **HLS con segmentos fMP4** (Safari nativo; hls.js 1.5.20 copiado en `web/vendor/` en el resto).
- **Modo copia** (H.264 de 8 bits, y HEVC si el navegador lo decodifica): vídeo copiado (`hvc1` para Safari) y audio a **AAC estéreo 192 kb/s** (mezcla de 5.1 de ffmpeg, codificador AAC `fast`).
- **XviD, HEVC en navegadores sin HEVC (Firefox) y H.264 de 10 bits: se recodifican a H.264** (`h264_amf` si funciona en el equipo, comprobado al arrancar; si no, `libx264 veryfast`), como mucho 1080p y 8 bits; el HDR se pasa a SDR con `zscale`+`tonemap` si el ffmpeg los tiene. `--video-encoder off` lo desactiva y la web lo explica («se puede ver en la TV»). Motivo: los 26 XviD son SD (recodificar cuesta poco) y en Safari, Chrome y Edge con aceleración el HEVC va copiado; avisar dejaría sin ver en Firefox un tercio de la colección.
- **Saltos en cualquier punto:** la lista es VOD con toda la duración. En modo copia los cortes son fotogramas clave reales sacados del índice Cues del MKV (instantáneo; ffprobe solo para otros contenedores); al recodificar, cortes cada 6 s con fotograma clave forzado. Cada segmento se genera bajo demanda: ffmpeg escribe un MP4 fragmentado por su salida estándar desde el segmento pedido (`-ss` + `-copyts` + `frag_discont` + `negative_cts_offsets`, marcas de tiempo del archivo +10 s) y el servidor lo corta según el plan. Si el reproductor salta lejos se relanza ffmpeg en ese segmento; los segmentos de ejecuciones distintas encajan porque conservan la línea de tiempo del archivo.
- **Recursos:** como mucho 2 conversiones a la vez (`--remux-jobs`; una por reproductor); ffmpeg se frena cuando va 8 segmentos por delante (el servidor deja de leer la tubería, sin señales de proceso, así que vale en Windows) y se mata si el reproductor deja de pedir 60 s o se cierra. Caché temporal en `DATOS/remux` (se vacía al arrancar; tope `--remux-cache-gb`, 20 GB; se borra lo que lleva 6 h sin usarse).
- **Subtítulos:** los de texto, en WebVTT (`/files/{id}/vtt/{sid}`) en un `<track>`; así se ven también en la pantalla completa nativa del iPhone, donde no se puede dibujar HTML encima del vídeo.

**Consecuencias.**
- El Ally gasta CPU mientras alguien ve algo en el navegador: casi nada en modo copia (solo audio), bastante al recodificar. Un 4K HEVC 10 bits recodificado por software puede no llegar a tiempo real en el Z1: sin probar.
- Los saltos en HLS tardan algo más que en directo (ffmpeg arranca en el punto nuevo); con recodificación, unos segundos.
- Sin DRM, sin acceso desde fuera de casa (la web es solo LAN, como la API) y sin elegir calidad: no se promete nada de eso.
- Pendiente de probar en dispositivos reales contra el Ally: iPhone (Safari nativo, HEVC copiado, pantalla completa con subtítulos), Android (Chrome), Mac (Safari y Chrome), Windows (Edge/Chrome con HEVC y Firefox recodificando); que `h264_amf` respete los fotogramas clave forzados; carga de CPU y tiempos de salto con los archivos de decenas de GB.

## ADR-007 · Administración web en `/admin/`, aparte de `/api/v1` — 24-10-2026

> Numerada 007 porque la 006 queda para el remux en vivo del cliente web (petición 5 del plan), que se hace a la vez en otra rama. Si al fusionar no existe, se puede renumerar.

**Contexto.** PLAN.md §6 esbozaba los endpoints de administración dentro de `/api/v1/admin/…`. Pero `/api/v1` tiene CORS abierto y no tiene sesión (lo usan la TV y la web de la colección), y la administración sí necesita rutas de disco, contraseña y protección CSRF.

**Decisión.**
- Paquete propio `server/internal/admin`, montado como un `http.Handler` en `/admin/` delante del resto del servidor: la web estática en `/admin/` (carpeta `admin/` del repo, servida desde disco, sin CDN) y la API en `/admin/api/…` (contrato en `docs/API.md`). `/api/v1` no cambia.
- Acceso: solo LAN; desde `localhost` (IP de bucle **y** `Host` local) sin contraseña; desde otro equipo, contraseña con hash PBKDF2-SHA256 de la biblioteca estándar (sin dependencias nuevas, ADR-002) y cookie de sesión en memoria. La primera contraseña solo desde `localhost`, la CLI (`yourflix admin password`) o `YOURFLIX_ADMIN_PASSWORD`. CSRF: JSON obligatorio + `Origin` + token de sesión en `X-CSRF-Token`.
- Los escaneos e identificaciones de la admin van en una cola de un solo trabajo, con progreso por SSE, y comparten un candado con el re-escaneo periódico de `serve`.
- La tabla `scan_runs` (historial de escaneos de la admin) la crea `store.InitAdmin`, sin tocar el esquema común.

**Consecuencias.**
- La API de administración devuelve rutas (como permite `AGENTS.md` para el explorador), pero solo a la admin; la TV recibe 401.
- Reiniciar el servidor cierra las sesiones remotas (la de `localhost` se recrea sola).
- El binario busca la web en `YOURFLIX_ADMIN_WEB`, `admin/` junto al ejecutable, `../admin` (para `private/yourflix.exe` en el repo) y `./admin`. El instalador del servicio de Windows (fase 5) debe copiar `admin/` junto al `.exe`.
- El emparejamiento de la TV por código (§3.3, fase 6) mostrará su código en esta admin.
