package remux

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// SegmentTarget: duración mínima de un segmento HLS (s). En modo copia se corta en el primer
// fotograma clave que la supera; al recodificar, ffmpeg fuerza un fotograma clave cada 6 s.
const SegmentTarget = 6.0

// tsOffset: ffmpeg suma 10 s a todas las marcas de tiempo (-output_ts_offset) para que el audio
// que empieza un poco antes del vídeo (AAC, AC3 con start_time negativo) no quede en negativo.
// Es una constante para todas las conversiones de un archivo, así que los segmentos de
// ejecuciones distintas de ffmpeg encajan en la misma línea de tiempo.
const tsOffset = 10.0

// eps: tolerancia al comparar el inicio de un fragmento con el de un segmento.
const eps = 0.005

// Plan: inicio de cada segmento en la línea de tiempo del archivo (s). El segmento i va de
// Starts[i] a Starts[i+1] (o hasta Duration).
type Plan struct {
	Starts   []float64
	Duration float64
}

// copyPlan corta en fotogramas clave reales (los del índice del archivo).
func copyPlan(keys []float64, duration float64) *Plan {
	p := &Plan{Starts: []float64{0}, Duration: duration}
	for _, k := range keys {
		if k-p.Starts[len(p.Starts)-1] >= SegmentTarget-eps && k < duration-1 {
			p.Starts = append(p.Starts, k)
		}
	}
	return p
}

// gridPlan: segmentos de 6 s exactos (al recodificar se fuerza un fotograma clave en cada corte).
func gridPlan(duration float64) *Plan {
	p := &Plan{Duration: duration}
	for t := 0.0; t == 0 || t < duration-1; t += SegmentTarget {
		p.Starts = append(p.Starts, t)
	}
	return p
}

func (p *Plan) Len() int { return len(p.Starts) }

func (p *Plan) End(i int) float64 {
	if i+1 < len(p.Starts) {
		return p.Starts[i+1]
	}
	return p.Duration
}

// SegmentAt: índice del segmento que contiene el instante t.
func (p *Plan) SegmentAt(t float64) int {
	i := sort.Search(len(p.Starts), func(i int) bool { return p.Starts[i] > t+eps }) - 1
	if i < 0 {
		return 0
	}
	return i
}

// Media: lista de reproducción VOD con todos los segmentos. query va pegado a cada URI
// (sesión y punto de arranque), sin «?».
func (p *Plan) Media(query string) string {
	var b strings.Builder
	max := 1.0
	for i := range p.Starts {
		max = math.Max(max, p.End(i)-p.Starts[i])
	}
	q := ""
	if query != "" {
		q = "?" + query
	}
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n", int(math.Ceil(max)))
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"init.mp4%s\"\n", q)
	for i := range p.Starts {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n%d.m4s%s\n", p.End(i)-p.Starts[i], i, q)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// Master: lista maestra con una sola variante (Safari quiere CODECS para HEVC).
func Master(codecs string, bandwidth int64, width, height int, videoRange, mediaURI string) string {
	attrs := []string{"BANDWIDTH=" + strconv.FormatInt(bandwidth, 10), `CODECS="` + codecs + `"`}
	if width > 0 && height > 0 {
		attrs = append(attrs, fmt.Sprintf("RESOLUTION=%dx%d", width, height))
	}
	if videoRange != "" {
		attrs = append(attrs, "VIDEO-RANGE="+videoRange)
	}
	return "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-STREAM-INF:" + strings.Join(attrs, ",") + "\n" + mediaURI + "\n"
}

// Cadenas RFC 6381 a partir de lo que dice ffprobe.
var avcProfiles = map[string]string{
	"baseline": "4200", "constrained baseline": "42e0", "main": "4d40", "extended": "5800",
	"high": "6400", "high 10": "6e00", "high 4:2:2": "7a00", "high 4:4:4 predictive": "f400",
}

func videoCodecString(v VideoInfo) string {
	switch v.Codec {
	case "h264":
		pc := avcProfiles[strings.ToLower(v.Profile)]
		if pc == "" {
			pc = "6400"
		}
		lvl := v.Level
		if lvl <= 0 {
			lvl = 41
		}
		return fmt.Sprintf("avc1.%s%02x", pc, lvl)
	case "hevc":
		prof, compat := 1, 6
		if strings.Contains(strings.ToLower(v.Profile), "10") || v.BitDepth >= 10 {
			prof, compat = 2, 4
		}
		lvl := v.Level
		if lvl <= 0 {
			lvl = 120
			if v.Height > 1100 {
				lvl = 153
			}
		}
		return fmt.Sprintf("hvc1.%d.%d.L%d.B0", prof, compat, lvl)
	}
	return ""
}

// Códecs de la salida recodificada: H.264 High (nivel 4.2 declarado) y AAC-LC.
const (
	transcodeCodecs = "avc1.64002a,mp4a.40.2"
	aacCodec        = "mp4a.40.2"
)

func videoRange(transfer string) string {
	switch transfer {
	case "smpte2084":
		return "PQ"
	case "arib-std-b67":
		return "HLG"
	}
	return ""
}
