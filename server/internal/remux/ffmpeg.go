package remux

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Info: lo que hace falta saber del archivo para planificar y elegir códecs.
type Info struct {
	Container string      `json:"container"`
	Duration  float64     `json:"duration"`
	Bitrate   int64       `json:"bitrate"`
	Video     VideoInfo   `json:"video"`
	Audio     []AudioInfo `json:"audio"`
	Keyframes []float64   `json:"keyframes,omitempty"` // solo se calculan para el modo copia
}

type VideoInfo struct {
	Codec    string  `json:"codec"`
	Profile  string  `json:"profile,omitempty"`
	Level    int     `json:"level,omitempty"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	PixFmt   string  `json:"pixFmt,omitempty"`
	BitDepth int     `json:"bitDepth,omitempty"`
	Transfer string  `json:"transfer,omitempty"`
	FPS      float64 `json:"fps,omitempty"`
}

type AudioInfo struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels"`
}

func probeInfo(ctx context.Context, ffprobe, path string) (*Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	var raw struct {
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index       int            `json:"index"`
			CodecType   string         `json:"codec_type"`
			CodecName   string         `json:"codec_name"`
			Profile     string         `json:"profile"`
			Level       int            `json:"level"`
			Width       int            `json:"width"`
			Height      int            `json:"height"`
			PixFmt      string         `json:"pix_fmt"`
			BitsRaw     string         `json:"bits_per_raw_sample"`
			ColorTrc    string         `json:"color_transfer"`
			AvgRate     string         `json:"avg_frame_rate"`
			Channels    int            `json:"channels"`
			Disposition map[string]int `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	in := &Info{Container: raw.Format.FormatName}
	in.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	in.Bitrate, _ = strconv.ParseInt(raw.Format.BitRate, 10, 64)
	haveVideo := false
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			if haveVideo || s.Disposition["attached_pic"] == 1 {
				continue
			}
			haveVideo = true
			v := VideoInfo{Codec: s.CodecName, Profile: s.Profile, Level: s.Level, Width: s.Width, Height: s.Height, PixFmt: s.PixFmt, Transfer: s.ColorTrc}
			v.BitDepth, _ = strconv.Atoi(s.BitsRaw)
			if v.BitDepth == 0 && (strings.Contains(s.PixFmt, "10le") || strings.Contains(s.PixFmt, "10be")) {
				v.BitDepth = 10
			}
			if a, b, ok := strings.Cut(s.AvgRate, "/"); ok {
				n, _ := strconv.ParseFloat(a, 64)
				d, _ := strconv.ParseFloat(b, 64)
				if d > 0 {
					v.FPS = n / d
				}
			}
			in.Video = v
		case "audio":
			in.Audio = append(in.Audio, AudioInfo{Index: s.Index, Codec: s.CodecName, Channels: s.Channels})
		}
	}
	if !haveVideo {
		return nil, fmt.Errorf("sin pista de vídeo")
	}
	if in.Duration <= 0 {
		return nil, fmt.Errorf("duración desconocida")
	}
	return in, nil
}

// keyframes: del índice del MKV si lo hay (instantáneo); si no, ffprobe lee los paquetes
// del vídeo (lee el archivo entero: solo pasa con MP4/AVI en modo copia, y se guarda en caché).
func keyframes(ctx context.Context, ffprobe, path string, in *Info) ([]float64, error) {
	if strings.Contains(in.Container, "matroska") {
		if f, err := os.Open(path); err == nil {
			st, _ := f.Stat()
			keys, _, err := MKVKeyframes(f, st.Size())
			f.Close()
			if err == nil {
				return keys, nil
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "V:0", "-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe (fotogramas clave): %w", err)
	}
	var keys []float64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		ts, flags, ok := strings.Cut(sc.Text(), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}
		if t, err := strconv.ParseFloat(ts, 64); err == nil {
			keys = append(keys, t)
		}
	}
	sort.Float64s(keys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("no se encontraron fotogramas clave")
	}
	return keys, nil
}

// Variant: qué se pide a ffmpeg. Audio < 0: sin audio.
type Variant struct {
	Audio int
	Video string // ModeCopy | ModeH264
}

const (
	ModeCopy = "copy" // vídeo tal cual (H.264 o HEVC), audio a AAC
	ModeH264 = "h264" // vídeo recodificado a H.264 (XviD, HEVC en navegadores sin HEVC…)
)

// ParseVariant: «a2-copy», «a1-h264», «an-copy».
func ParseVariant(s string) (Variant, bool) {
	a, mode, ok := strings.Cut(s, "-")
	if !ok || len(a) < 2 || a[0] != 'a' || (mode != ModeCopy && mode != ModeH264) {
		return Variant{}, false
	}
	if a == "an" {
		return Variant{Audio: -1, Video: mode}, true
	}
	n, err := strconv.Atoi(a[1:])
	if err != nil || n < 0 || n > 999 || strconv.Itoa(n) != a[1:] {
		return Variant{}, false
	}
	return Variant{Audio: n, Video: mode}, true
}

func (v Variant) String() string {
	if v.Audio < 0 {
		return "an-" + v.Video
	}
	return "a" + strconv.Itoa(v.Audio) + "-" + v.Video
}

func secs(t float64) string { return strconv.FormatFloat(t, 'f', 3, 64) }

// ffmpegArgs: un MP4 fragmentado por la salida estándar, desde el segmento start.
// Las marcas de tiempo conservan la línea de tiempo del archivo (+tsOffset) en todas las
// ejecuciones, para que los segmentos de un salto encajen con los ya servidos.
func (m *Manager) ffmpegArgs(path string, in *Info, v Variant, plan *Plan, start int) []string {
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
	if strings.Contains(in.Container, "avi") {
		args = append(args, "-fflags", "+genpts")
	}
	if start > 0 {
		if v.Video == ModeCopy {
			// Sin recodificar, ffmpeg empieza en el fotograma clave anterior al punto pedido
			// (y con vídeo con fotogramas B resta ~0,13 s antes de buscar). Lo que salga antes
			// del inicio del segmento se descarta al agrupar.
			args = append(args, "-noaccurate_seek", "-ss", secs(plan.Starts[start]+eps))
		} else {
			args = append(args, "-ss", secs(plan.Starts[start]))
		}
	}
	args = append(args, "-copyts", "-i", path, "-map", "0:V:0")
	if v.Audio >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(v.Audio))
	}
	if v.Video == ModeCopy {
		args = append(args, "-c:v", "copy")
		if in.Video.Codec == "hevc" {
			args = append(args, "-tag:v", "hvc1") // Safari solo acepta HEVC como hvc1
		}
	} else {
		args = append(args, "-vf", m.videoFilter(in))
		switch m.encoder {
		case "h264_amf":
			// -forced_idr 1: sin él, h264_amf ignora -force_key_frames (IDR cada ~10 s en vez de
			// cada segmento) y ffmpeg se relanzaba cada dos segmentos (revisión del PR #3).
			args = append(args, "-c:v", "h264_amf", "-usage", "transcoding", "-quality", "speed", "-rc", "vbr_peak",
				"-b:v", "8M", "-maxrate", "12M", "-bufsize", "16M", "-profile:v", "high", "-forced_idr", "1")
		default:
			args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "21", "-maxrate", "12M", "-bufsize", "24M",
				"-profile:v", "high", "-level:v", "4.2")
		}
		// t cuenta desde el punto de arranque, que siempre es un corte del plan.
		args = append(args, "-force_key_frames", "expr:gte(t,n_forced*"+strconv.Itoa(int(SegmentTarget))+")")
	}
	if v.Audio >= 0 {
		// El codificador AAC por defecto (twoloop) es el cuello de botella del modo copia; «fast»
		// lo deja en ~60× tiempo real con calidad de sobra a 192 kb/s.
		args = append(args, "-c:a", "aac", "-aac_coder", "fast", "-ac", "2", "-b:a", "192k")
	}
	args = append(args, "-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1",
		"-output_ts_offset", secs(tsOffset), "-max_muxing_queue_size", "4096",
		"-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof+frag_discont+negative_cts_offsets", "pipe:1")
	return args
}

// videoFilter: como mucho 1080p, 8 bits 4:2:0 y, si es HDR y ffmpeg tiene zscale, mapeo de tonos a SDR.
func (m *Manager) videoFilter(in *Info) string {
	scale := "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2"
	if videoRange(in.Video.Transfer) != "" && m.tonemap {
		return "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv," + scale + ",format=yuv420p"
	}
	return scale + ",format=yuv420p"
}

// DetectEncoder prueba de verdad el codificador (h264_amf puede venir en ffmpeg y no
// funcionar si no hay GPU AMD). want: auto | h264_amf | libx264 | off.
func DetectEncoder(ctx context.Context, ffmpeg, want string) string {
	try := func(enc string) bool {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=black:s=320x240:r=25:d=0.4",
			"-c:v", enc, "-f", "null", "-").Run() == nil
	}
	var order []string
	switch want {
	case "off", "none":
		return ""
	case "", "auto":
		order = []string{"h264_amf", "libx264"}
	default:
		order = []string{want}
	}
	for _, e := range order {
		if try(e) {
			return e
		}
	}
	return ""
}

func hasTonemap(ctx context.Context, ffmpeg string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-filters").Output()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, " zscale ") && strings.Contains(s, " tonemap ")
}
