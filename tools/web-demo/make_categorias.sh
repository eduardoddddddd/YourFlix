#!/usr/bin/env bash
# Biblioteca de demostración para las fichas navegables y el kiosco: ~50 títulos reales de la
# semilla `cine-t7-catalogo` (solo se lee) con vídeos pequeños generados con ffmpeg, en cuatro
# calidades (4K HDR, 1080p, 720p, SD) e idiomas distintos para que «Filtrar por» tenga de todo.
#   tools/web-demo/make_categorias.sh RUTA-CLON-cine-t7-catalogo CARPETA
# Después:  CARPETA/yourflix --data CARPETA/data serve --web web
set -euo pipefail
SEED=${1:?uso: make_categorias.sh RUTA-SEMILLA CARPETA}
OUT=${2:?uso: make_categorias.sh RUTA-SEMILLA CARPETA}
DUR=${DUR:-30}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
BIN=${YOURFLIX:-$OUT/yourflix}
mkdir -p "$OUT/lib" "$OUT/tpl" "$OUT/data"
[ -x "$BIN" ] || (cd "$ROOT/server" && go build -o "$BIN" ./cmd/yourflix)
f() { ffmpeg -hide_banner -loglevel error -y "$@"; }
src() { echo "testsrc2=size=$1:rate=$2,drawtext=text='%{pts\:hms}':fontsize=40:x=20:y=20:fontcolor=white:box=1:boxcolor=black@0.5"; }
T="$OUT/tpl"
printf '1\n00:00:02,000 --> 00:00:08,000\nSubtítulo de prueba\n' > "$T/s.srt"
[ -f "$T/4k.mkv" ] || f -f lavfi -i "$(src 3840x1600 12)" -f lavfi -i "sine=440:sample_rate=48000" -f lavfi -i "sine=660:sample_rate=48000" -t "$DUR" \
  -map 0:v -map 1:a -map 2:a -c:v libx265 -preset ultrafast -pix_fmt yuv420p10le -x265-params keyint=48:log-level=error:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc \
  -c:a ac3 -metadata:s:a:0 language=spa -metadata:s:a:1 language=eng "$T/4k.mkv"
[ -f "$T/1080.mkv" ] || f -f lavfi -i "$(src 1920x800 24)" -f lavfi -i "sine=330:sample_rate=48000" -f lavfi -i "sine=550:sample_rate=48000" -i "$T/s.srt" -t "$DUR" \
  -map 0:v -map 1:a -map 2:a -map 3 -map 3 -c:v libx264 -preset veryfast -g 48 -c:a:0 ac3 -c:a:1 aac -c:s srt \
  -metadata:s:a:0 language=spa -metadata:s:a:1 language=eng -metadata:s:s:0 language=spa -metadata:s:s:1 language=eng "$T/1080.mkv"
[ -f "$T/720.mp4" ] || f -f lavfi -i "$(src 1280x720 25)" -f lavfi -i "sine=880:sample_rate=48000" -t "$DUR" \
  -c:v libx264 -preset veryfast -g 50 -c:a aac -metadata:s:a:0 language=eng -movflags +faststart "$T/720.mp4"
[ -f "$T/sd.avi" ] || f -f lavfi -i "$(src 640x352 25)" -f lavfi -i "sine=220:sample_rate=44100" -t "$DUR" \
  -c:v mpeg4 -vtag XVID -q:v 5 -c:a libmp3lame -b:a 128k "$T/sd.avi"
# Selección: las películas de los directores con más títulos, más otras hasta 46, y 2 series.
python3 - "$SEED/data/catalog.json" "$OUT/lib" "$T" <<'PY'
import json, sys, os, subprocess, collections, re
cat, lib, tpl = sys.argv[1:4]
items = [i for i in json.load(open(cat))["items"] if (i.get("meta") or {}).get("matched")]
movies = [i for i in items if i["type"] != "series"]
series = [i for i in items if i["type"] == "series"]
dirs = collections.Counter(d for i in movies for d in i["meta"].get("director") or [])
top = [d for d, n in dirs.most_common(6)]
pick = [i for i in movies if set(i["meta"].get("director") or []) & set(top)]
pick += [i for i in movies if i not in pick][: max(0, 46 - len(pick))]
tpls = ["1080.mkv", "1080.mkv", "720.mp4", "sd.avi", "4k.mkv"]
safe = lambda s: re.sub(r'[\\/:*?"<>|]', "", s).strip()
for n, i in enumerate(pick):
    t = tpls[n % len(tpls)]
    out = os.path.join(lib, "%s (%s)%s" % (safe(i["meta"]["title"]), i["meta"]["year"], os.path.splitext(t)[1]))
    if not os.path.exists(out):
        subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", os.path.join(tpl, t), "-map", "0", "-c", "copy",
                        "-metadata", "title=" + i["meta"]["title"], out], check=True)
for i in series[:2]:
    name = safe(i["meta"]["title"])
    os.makedirs(os.path.join(lib, name), exist_ok=True)
    for ep in ("S01E01", "S01E02", "S02E01"):
        out = os.path.join(lib, name, "%s %s.mkv" % (name, ep))
        if not os.path.exists(out):
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", os.path.join(tpl, "1080.mkv"), "-map", "0", "-c", "copy",
                            "-metadata", "title=%s %s" % (name, ep), out], check=True)
print(len(pick), "películas,", min(2, len(series)), "series")
PY
Y="$BIN --data $OUT/data"
$Y seed import "$SEED"
$Y source list | grep -q "$OUT/lib" || $Y source add "$OUT/lib" --label Ally
$Y source scan --all
echo "Listo. Arranca: $Y serve --web $ROOT/web   y abre http://localhost:8787/"
