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
