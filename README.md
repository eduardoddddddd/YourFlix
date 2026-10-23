# YourFlix

Servidor de películas y series para casa (Asus ROG Ally con Windows 11, o Mac) y app nativa para la Samsung The Serif (Tizen 6.5). El servidor reúne varias carpetas elegidas por el usuario, identifica cada vídeo con fichas y carátulas y lo sirve por la red local; la tele lo reproduce con Samsung AVPlay.

- [PLAN.md](PLAN.md): arquitectura, fases y decisiones.
- [AGENTS.md](AGENTS.md): reglas para cualquier modelo que trabaje en el repo.

## Estado

**23-10-2026 — fase de planificación.** Solo existe el plan; no hay código.

- Hecho: plan v1 (Opus 5.5), nombre YourFlix, servidor en el ROG Ally, fuentes múltiples, carga inicial desde `cine-t7-catalogo` y TMDB para lo nuevo. Token TMDB comprobado; vive en `secrets/tmdb.token` (fuera de Git).
- Probado de verdad: nada todavía en la TV.
- Pendiente de decidir: Go o Python para el servidor, perfiles de usuario, modelo de Ally y dock, perfil de formatos de la TV (memorias de Codex del usuario).
- Siguiente paso: **fase 0** (pruebas de AVPlay en la TV real, ver PLAN.md §8).
