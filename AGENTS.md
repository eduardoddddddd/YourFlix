# YourFlix — instrucciones para cualquier modelo

Lee `PLAN.md` completo y la sección «Estado» de `README.md` antes de tocar nada. `PLAN.md` contiene las decisiones; no las reabras sin un dato nuevo y, si cambian, registra un ADR en `docs/DECISIONES.md`.

- Proyecto **local** e independiente de `AstroIPTV`, de la app `CineT7` (USB) y del repo público `cine-t7-catalogo`. De este último solo se **lee** `data/catalog.json` como semilla; no lo modifiques.
- TV real: Samsung The Serif `QE43LS01BBUXXC`, Tizen 6.5, Chromium M85, `192.168.1.4:26101`. Verifica las IP antes de usarlas.
- App TV: ID `Z8YOURFLX1.YourFlix` (consérvalo siempre al actualizar). Firma con el perfil `CineT7` **sin copiar certificados ni contraseñas al repo**. `.wgt` sin espacios en el nombre físico.
- AVPlay: sigue las lecciones de `AstroIPTV/README.md` (serializar aperturas, token, nunca `setSelectTrack('TEXT')`, plano de vídeo detrás del HTML). Los subtítulos los dibuja la app.
- Servidor: la biblioteca es **solo lectura**. La API nunca recibe rutas, solo IDs (salvo el explorador de carpetas de la admin, solo para localhost/admin).
- Metadatos: la colección T7 sale de la semilla `cine-t7-catalogo`, sin llamar a TMDB; TMDB solo para lo nuevo. El token está en `secrets/tmdb.token` (origen: nota «TMDB API» de Joplin). Nunca lo copies al repo, a la TV, a logs ni a respuestas de la API.
- No instales ni lances nada en la TV sin permiso del usuario, y nunca mientras está viendo algo.
- Un escritor por zona (`server/`, `tv/`); rama por paquete; revisión por un modelo distinto.
- Antes de entregar: `go test ./...`, `go vet ./...`, `node --test tv/tests/*.test.js`, `node --check` de cada JS de `tv/`.
- Distingue siempre «probado en navegador» de «probado en la TV». No prometas transcodificación en vivo, DRM ni acceso remoto.
- Al terminar, actualiza «Estado» en `README.md`: hecho, probado de verdad, pendiente, siguiente paso.
