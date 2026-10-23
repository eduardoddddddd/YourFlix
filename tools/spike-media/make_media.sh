#!/bin/sh
# Genera los archivos sintéticos de la matriz de la fase 0 (docs/PRUEBAS-TV.md).
# La imagen es testsrc (lleva un contador de tiempo), así se comprueba a ojo cada salto.
# Uso: tools/spike-media/make_media.sh [carpeta]   (por defecto private/testmedia)
set -eu
OUT=${1:-"$(dirname "$0")/../../private/testmedia"}
mkdir -p "$OUT"; cd "$OUT"
D=120
FF="ffmpeg -hide_banner -loglevel error -y"
V1080="testsrc=size=1920x1080:rate=24:duration=$D"
TONE_A="sine=frequency=440:duration=$D"
TONE_B="sine=frequency=880:duration=$D"

srt() { # $1 fichero, $2 prefijo
  : > "$1"; i=1; t=0
  while [ $t -lt $D ]; do
    printf '%d\n00:%02d:%02d,000 --> 00:%02d:%02d,500\n%s %02d:%02d — línea %d\n\n' \
      $i $((t/60)) $((t%60)) $((t/60)) $(((t+4)%60)) "$2" $((t/60)) $((t%60)) $i >> "$1"
    i=$((i+1)); t=$((t+5))
  done
}
srt es.srt "ES"; srt en.srt "EN"

$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -map 0 -map 1 -c:v libx264 -profile:v high -pix_fmt yuv420p -preset veryfast -b:v 6M -c:a ac3 -ac 6 -b:a 448k -metadata:s:a:0 language=spa s1_h264_ac3.mkv
$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -map 0 -map 1 -c:v libx265 -profile:v main10 -pix_fmt yuv420p10le -preset fast -b:v 5M -tag:v hvc1 -c:a eac3 -ac 6 -b:a 640k -metadata:s:a:0 language=spa s2_hevc10_eac3.mkv
$FF -f lavfi -i "testsrc=size=720x400:rate=24:duration=$D" -f lavfi -i "$TONE_A" -c:v mpeg4 -vtag XVID -q:v 4 -c:a libmp3lame -b:a 160k s3_xvid.avi
$FF -f lavfi -i "testsrc=size=720x400:rate=24:duration=$D" -f lavfi -i "$TONE_A" -c:v mpeg4 -vtag XVID -q:v 4 -c:a ac3 -b:a 192k s3_xvid.mkv
$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -c:v libx264 -preset veryfast -b:v 6M -pix_fmt yuv420p -c:a dca -strict -2 -ac 6 -b:a 1509k s4_dts.mkv
$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -f lavfi -i "$TONE_B" -map 0 -map 1 -map 2 -c:v libx264 -preset veryfast -b:v 6M -pix_fmt yuv420p -c:a:0 ac3 -ac:a:0 6 -c:a:1 aac -ac:a:1 2 -metadata:s:a:0 language=spa -metadata:s:a:0 title="Español 440 Hz" -metadata:s:a:1 language=eng -metadata:s:a:1 title="English 880 Hz" s5_2audio.mkv
$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -i es.srt -i en.srt -map 0 -map 1 -map 2 -map 3 -c:v libx264 -preset veryfast -b:v 6M -pix_fmt yuv420p -c:a aac -c:s srt -metadata:s:s:0 language=spa -metadata:s:s:1 language=eng -disposition:s:0 default s6_srt.mkv
$FF -f lavfi -i "$V1080" -f lavfi -i "$TONE_A" -c:v libx264 -preset veryfast -b:v 6M -pix_fmt yuv420p -c:a aac -b:a 192k -movflags +faststart s8_h264_aac.mp4
# 4K HEVC Main10 con etiquetas HDR10 y ruido para forzar ~40 Mb/s (caudal realista de un 4K)
$FF -f lavfi -i "testsrc=size=3840x2160:rate=24:duration=60,noise=alls=25:allf=t" -f lavfi -i "sine=frequency=440:duration=60" -c:v hevc_videotoolbox -profile:v main10 -pix_fmt p010le -b:v 40M -tag:v hvc1 -color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc -c:a eac3 -ac 6 s7_4k_hevc_hdr.mkv
# > 4 GB: 1080p H.264 con ruido (videotoolbox se queda en ~42 Mb/s, 3,8 GB en 12 min),
# así que se une dos veces sin recodificar: 24 min, ~7,6 GB.
$FF -f lavfi -i "testsrc=size=1920x1080:rate=24:duration=720,noise=alls=30:allf=t" -f lavfi -i "sine=frequency=440:duration=720" -c:v h264_videotoolbox -b:v 55M -maxrate 60M -c:a ac3 -b:a 448k s9_half.mkv
printf "file 's9_half.mkv'\nfile 's9_half.mkv'\n" > concat.txt
$FF -f concat -safe 0 -i concat.txt -c copy s9_big_7gb.mkv
rm concat.txt s9_half.mkv
ls -la
