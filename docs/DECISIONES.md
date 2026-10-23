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
