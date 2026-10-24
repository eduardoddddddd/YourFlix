# Entorno del ROG Ally y relevo (24-10-2026)

Documento operativo para el siguiente modelo (Opus) que continúe en el Ally. Las decisiones están en `PLAN.md` y `docs/DECISIONES.md`; el estado resumido, en `README.md`. Aquí va lo que **se hizo realmente** en esta máquina y por dónde seguir.

## Máquina

| Dato | Valor |
|---|---|
| Equipo | `RogAlly`, Windows 11, usuario `eduar` |
| IP | `192.168.1.13` por **Wi-Fi**, reservada en el DHCP del usuario. Es la que se usa en todas las pruebas |
| Futuro | Cuando el Ally pase a **cable** (dock con Ethernet) tendrá **otra IP fija**; el usuario la comunicará. Entonces: actualizar la dirección de fábrica de la app (`tv/`), la reserva y este documento |
| Herramientas | Go 1.27 windows/amd64, Node 24, ffmpeg/ffprobe 9.0.2 (gyan.dev, en el PATH). **No hay Python** instalado (el alias de la Store no vale) |
| Worktree | `C:\Users\eduar\developer\YourFlix\.claude\worktrees\rog-ally-project-384d24`, rama `claude/rog-ally-project-384d24` |
| TV | `192.168.1.4:26101`, sin tocar en esta sesión. App instalada: 0.2.0 |

## Qué hay en disco (fuera de Git)

| Ruta | Contenido |
|---|---|
| `C:\YourFlix\T7\pelis`, `…\pelis2` | Copia del SSD Samsung T7: 463 archivos, 882 GB (238,4 + 643,9 GB). Estructura idéntica al origen |
| `C:\YourFlix\Patriot` | Copia del USB Patriot (exFAT, 117 GB): 34 archivos, 104 GB, todos en la raíz salvo la carpeta `007` |
| `C:\YourFlix\copy_pelis.log`, `copy_pelis2.log`, `copy_patriot.log` | Logs de `robocopy` (0 errores) |
| `private/yourflix.exe`, `private/ffprobe.exe` | Binario del servidor (windows/amd64) y ffprobe |
| `private/data/` | SQLite, imágenes y registro del servidor |
| `private/cine-t7-catalogo/` | Clon superficial de `eduardoddddddd/cine-t7-catalogo` (la semilla, solo lectura) |
| `secrets/tmdb.token` | Token de lectura de TMDB (v4). Comprobado: 200 |
| `secrets/joplin.token` | Token del Web Clipper de Joplin local (`127.0.0.1:41184`). La nota «TMDB API» **no** está en el Joplin del Ally (sin sincronizar); el token de TMDB lo pasó el usuario por el chat |

Los dos SSD originales siguen intactos (la copia fue de solo lectura sobre el origen). Se excluyeron `._*`, `.DS_Store`, `.device_info_*`, `$RECYCLE.BIN`, `System Volume Information`, `.Trashes`, `.Spotlight-V100`, `.fseventsd`. Del T7 **no** se copiaron `Downloads` (14 GB) ni los HTML del catálogo.

Comando de copia usado (PowerShell, `robocopy` devuelve 1 = «copiado»; el aviso de «failed» de la herramienta era solo eso):

```powershell
robocopy "D:\pelis" "C:\YourFlix\T7\pelis" /E /MT:16 /R:2 /W:2 /XF "._*" ".DS_Store" /XD ".Trashes" ".Spotlight-V100" ".fseventsd" "System Volume Information" /NP /NDL /NJH /LOG:"C:\YourFlix\copy_pelis.log"
```

> Desde Git Bash la variable de la carpeta no se expandió dentro de `robocopy` (primer intento, exit 16). Usar PowerShell.

## Qué se hizo en el servidor (todo en el Ally)

```
go vet ./... ; go test ./...                 # todo en verde
node --test tv/tests/*.test.js               # 12 de 12
go build -o ../private/yourflix.exe ./cmd/yourflix
yourflix --data private/data seed import private/cine-t7-catalogo
yourflix --data private/data source add 'C:\YourFlix\T7' --label "Samsung T7"
yourflix --data private/data source add 'C:\YourFlix\Patriot' --label "USB Patriot"
yourflix --data private/data source scan --all
```

Resultado medido:

| Fuente | Vídeos | Por bytes (semilla) | Por título | Solo nombre | Errores ffprobe | Tiempo |
|---|---|---|---|---|---|---|
| Samsung T7 | 462 | 460 | 2 | 0 | 0 | 13 s |
| USB Patriot | 34 | 34 | 0 | 0 | 0 | 0,5 s |

Semilla: 255 fichas, 499 copias medidas, 259 portadas. Catálogo resultante: 251 títulos disponibles, 0 pendientes de TMDB. No se hizo ninguna llamada a TMDB (la semilla cubrió todo), así que `identify` sigue sin probarse con archivos nuevos en el Ally.

Rendimiento observado de la copia: ≈200 MB/s desde el T7 y ≈110 MB/s desde el Patriot (ambos por USB). Mucho menos de lo que el usuario esperaba (≈40 GB/min); la causa no se investigó (puerto USB del Ally, modo de energía, exFAT…). Si se quiere medir: lectura secuencial de un archivo grande del disco original en otro puerto.

## Qué NO se ha hecho

- `serve` no se ha arrancado en el Ally; no hay regla de firewall; no hay servicio de Windows.
- No se ha tocado la TV: no se instaló nada nuevo ni se lanzó nada.
- No se ha medido el caudal Wi-Fi Ally → TV (S7/S10).
- La copia se verificó solo por número de archivos y bytes totales; **no se comparó el contenido** (ni hash) entre origen y destino. El emparejado «por bytes» con la semilla (494 de 496 vídeos) compara tamaño en bytes y duración, así que **no demuestra** la integridad del contenido. Pendiente si se quiere certeza: `Get-FileHash` (o `robocopy /L` con comparación) sobre origen y copia.
- No se ha cambiado código de `server/` ni de `tv/`: los cambios de esta sesión son solo documentación (`README.md`, `PLAN.md`, `docs/`) y están fusionados en `main` (24-10-2026).

## Por dónde seguir (S10 y después)

1. Arrancar el servidor:
   ```powershell
   .\private\yourflix.exe --data private\data --tmdb-token-file secrets\tmdb.token serve
   ```
   Comprobar `http://192.168.1.13:8787/api/v1/ping` desde otro equipo de la LAN.
2. Regla de firewall de entrada TCP 8787 **solo para red Privada** (pedir permiso al usuario; toca la configuración del sistema).
3. Con permiso del usuario y la TV libre: abrir la app 0.2.0 (busca primero la IP del Mac `192.168.1.3`, luego barre la /24: debería encontrar el Ally), probar H.264, HEVC 10 bits y el 4K, medir caudal Wi-Fi. Si el descubrimiento tarda, poner `192.168.1.13` como dirección de fábrica y reinstalar con el mismo ID.
4. Registrar resultados en `docs/PRUEBAS-TV.md` (S10) y en el «Estado» del `README.md`.
5. Fase 4 (compatibilidad y optimización: DTS → E-AC3; hoy hay 9 DTS) y fase 5 (admin web, servicio de Windows, energía del Ally, firewall, IP por cable).

El usuario ha avisado de que puede hacer una **refactorización** y añadir más cosas antes de continuar; confirmar con él el alcance antes de tocar `server/` o `tv/`.

## Servidor permanente en `C:\YourFlix\servidor` (24-10-2026)

Sustituye al `serve` lanzado a mano desde el worktree (que se para al cerrar la sesión de Claude o al reiniciar, y cuyos datos viven en una carpeta que se puede borrar al limpiar worktrees).

| Ruta | Contenido |
|---|---|
| `C:\YourFlix\servidor\yourflix.exe` | Binario (compilado desde `main`) |
| `…\ffmpeg.exe`, `…\ffprobe.exe` | Copias de ffmpeg 9.0.2 (gyan): la tarea corre como SYSTEM, que no ve el PATH del usuario |
| `…\secrets\tmdb.token` | Token de TMDB (fuera de Git) |
| `…\data\` | `yourflix.db`, `images\`, `subs\` y `yourflix.log` (se rota a `yourflix.old.log` al pasar de 10 MB) |
| `…\iniciar.cmd`, `parar.cmd`, `instalar.cmd`, `instalar-servicio.ps1` | Copias de `tools/ally/`. `instalar.cmd` (doble clic en el Ally) pide permiso de administrador y lanza el `.ps1`, que deja su resultado en `instalar.log` |

- **Arranque:** tarea programada «YourFlix» al encender, como SYSTEM y sin iniciar sesión. `iniciar.cmd` relanza el servidor a los 10 s si se cae. La crea `instalar-servicio.ps1`, que **ejecuta el usuario como administrador** porque cambia la configuración de Windows: tarea, regla de firewall «YourFlix (TCP 8787)» solo para ese ejecutable (perfiles Privado y Público, porque la Wi-Fi del Ally está como Pública) y energía con cargador sin suspensión ni hibernación.
- **Parar o arrancar a mano** (como administrador): `C:\YourFlix\servidor\parar.cmd` y `schtasks /run /tn YourFlix`.
- **Rutas de carátulas:** la importación de la semilla guarda rutas absolutas en `titles.poster` / `titles.backdrop`. `tools/ally/migrar-datos.mjs` copia la base de datos con `VACUUM INTO` y corrige esas rutas. Pendiente: guardarlas relativas a la carpeta de datos.
- **Probado** (sin cortar el servidor viejo): copia en `127.0.0.1:8788` con 251 títulos, carátulas y fondos 200, vídeo con Range 206, subtítulos cacheados y progreso idéntico; ffprobe encontrado junto al ejecutable.

**Cambio del viejo al nuevo** (corte de unos 20 s; solo con permiso del usuario y sin nadie viendo nada):
1. Parar el `serve` del worktree (proceso de la sesión de Claude).
2. `node tools/ally/migrar-datos.mjs <worktree>\private\data C:\YourFlix\servidor\data`, más `robocopy` de `images` y `subs` para lo extraído desde la prueba.
3. `schtasks /run /tn YourFlix` y comprobar `http://192.168.1.13:8787/api/v1/ping` y `data\yourflix.log`.
4. La carpeta `private\` del worktree se queda como copia de seguridad; las reglas «yourflix.exe» del worktree que aceptó el usuario dejan de usarse.

## Reglas que siguen vigentes

`AGENTS.md`: no instalar ni lanzar nada en la TV sin permiso ni mientras se ve algo; el token de TMDB y los certificados nunca al repo, a la TV, a logs ni a la API; ID de la app `Z8YOURFLX1.YourFlix` (conservarlo); firma con el perfil `CineT7`; distinguir «probado en navegador» de «probado en la TV»; antes de entregar, `go test ./...`, `go vet ./...`, `node --test tv/tests/*.test.js` y `node --check` de cada JS de `tv/`.
