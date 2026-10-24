#!/usr/bin/env bash
# Biblioteca de prueba para la web: vídeos generados con ffmpeg (los tres casos de la colección
# real y uno que el navegador lee tal cual), una serie con dos temporadas y subtítulos.
#   tools/web-demo/make_demo.sh CARPETA      → CARPETA/lib (vídeos) y CARPETA/data (base de datos)
# Después:  yourflix --data CARPETA/data serve --web web
set -euo pipefail
OUT=${1:?uso: make_demo.sh CARPETA}
DUR=${DUR:-180}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
BIN=${YOURFLIX:-$OUT/yourflix}
mkdir -p "$OUT/lib/Serie de prueba" "$OUT/data"
[ -x "$BIN" ] || (cd "$ROOT/server" && go build -o "$BIN" ./cmd/yourflix)
f() { ffmpeg -hide_banner -loglevel error -y "$@"; }
src() { echo "testsrc2=size=$1:rate=$2,drawtext=text='%{pts\:hms}':fontsize=28:x=12:y=12:fontcolor=white:box=1:boxcolor=black@0.5"; }
FIVE="pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0"
cat > "$OUT/subs.srt" <<'SRT'
1
00:00:02,000 --> 00:00:06,000
Subtítulo de prueba: ¿se ve bien?

2
00:01:00,000 --> 00:01:05,000
Un minuto.

3
00:02:00,000 --> 00:02:05,000
Dos minutos.
SRT
L="$OUT/lib"
[ -f "$L/Prueba uno (2001).mkv" ] || f -f lavfi -i "$(src 1280x720 24)" -f lavfi -i "sine=frequency=440:sample_rate=48000" -f lavfi -i "sine=frequency=660:sample_rate=44100" \
  -i "$OUT/subs.srt" -t "$DUR" -filter_complex "[1:a]$FIVE[a]" -map 0:v -map "[a]" -map 2:a -map 3 \
  -c:v libx264 -preset veryfast -g 240 -bf 2 -c:a:0 ac3 -c:a:1 aac -c:s srt \
  -metadata:s:a:0 language=spa -metadata:s:a:1 language=eng -metadata:s:s:0 language=spa "$L/Prueba uno (2001).mkv"
[ -f "$L/Prueba dos (2019).mkv" ] || f -f lavfi -i "$(src 1280x720 25)" -f lavfi -i "sine=frequency=330:sample_rate=48000" -t "$DUR" \
  -filter_complex "[1:a]$FIVE[a]" -map 0:v -map "[a]" -c:v libx265 -preset ultrafast -pix_fmt yuv420p10le -x265-params keyint=100:log-level=error \
  -c:a eac3 -metadata:s:a:0 language=eng "$L/Prueba dos (2019).mkv"
[ -f "$L/Prueba tres (1999).avi" ] || f -f lavfi -i "$(src 640x352 25)" -f lavfi -i "sine=frequency=550:sample_rate=44100" -t "$DUR" \
  -c:v mpeg4 -vtag XVID -q:v 5 -c:a libmp3lame -b:a 128k "$L/Prueba tres (1999).avi"
[ -f "$L/Prueba cuatro (2010).mp4" ] || f -f lavfi -i "$(src 1280x720 30)" -f lavfi -i "sine=frequency=880:sample_rate=48000" -t "$DUR" \
  -c:v libx264 -preset veryfast -c:a aac -movflags +faststart "$L/Prueba cuatro (2010).mp4"
cp "$OUT/subs.srt" "$L/Prueba cuatro (2010).es.srt"
for ep in S01E01 S01E02 S02E01; do
  [ -f "$L/Serie de prueba/Serie de prueba $ep.mkv" ] || f -f lavfi -i "$(src 960x540 24)" -f lavfi -i "sine=frequency=500:sample_rate=48000" -t 90 \
    -c:v libx264 -preset veryfast -c:a ac3 -metadata:s:a:0 language=spa "$L/Serie de prueba/Serie de prueba $ep.mkv"
done
Y="$BIN --data $OUT/data"
$Y source list | grep -q "$L" || $Y source add "$L" --label Ally
$Y source scan --all
echo "Listo. Arranca: $Y serve --web $ROOT/web   y abre http://localhost:8787/"
