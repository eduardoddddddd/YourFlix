# YourFlix

Servidor de películas y series para casa (Asus ROG Ally con Windows 11, o Mac) y app nativa para la Samsung The Serif (Tizen 6.5). El servidor reúne varias carpetas elegidas por el usuario, identifica cada vídeo con fichas y carátulas y lo sirve por la red local; la tele lo reproduce con Samsung AVPlay.

- [PLAN.md](PLAN.md): arquitectura, fases y decisiones.
- [AGENTS.md](AGENTS.md): reglas para cualquier modelo que trabaje en el repo.

## Estado

**23-10-2026 — fase 0 terminada, puerta G0 superada** ([ADR-001](docs/DECISIONES.md)).

- Hecho: plan v1; servidor de prueba `tools/spike-server` (Go), app de prueba `tools/spike-avplay` (`Z8YFSPIKE1.YourFlixSpike`, instalada en la TV), generador de vídeos `tools/spike-media`, monitor CDP `tools/tv-monitor`, perfil `profiles/samsung-ls01b-2022.json`.
- **Probado en la TV real** ([resultados](docs/PRUEBAS-TV.md)): reproducción directa por HTTP Range de MKV/MP4/AVI, H.264/HEVC 10 bits/XviD/4K, saltos en archivos de 2,4 y 7,6 GB, cambio de audio, SRT como texto; 0 cuelgues. DTS suena mudo (sin error); PGS no llega.
- No probado: HDR real, servidor en el ROG Ally, DTS con otra pista compatible.
- Pendiente de decidir: Go (lo usado en la fase 0) o Python para el servidor, perfiles de usuario, modelo de Ally y dock; completar el perfil de la TV con las memorias de Codex.
- Siguiente paso: **fase 1A** (núcleo del servidor) y, en paralelo, **1B** (semilla T7 + TMDB).
