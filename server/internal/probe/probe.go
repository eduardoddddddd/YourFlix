// Package probe lee las pistas de un vídeo con ffprobe y las normaliza.
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Video struct {
	Codec    string `json:"codec"`
	Profile  string `json:"profile,omitempty"`
	PixFmt   string `json:"pixFmt,omitempty"`
	BitDepth int    `json:"bitDepth,omitempty"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	HDR      string `json:"hdr,omitempty"` // HDR10 | HLG | DV
}

type Audio struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

type Subtitle struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Forced   bool   `json:"forced,omitempty"`
	Text     bool   `json:"text"` // false: imagen (PGS, DVD), AVPlay no la entrega
}

type Result struct {
	Container string
	Duration  float64
	Bitrate   int64
	Video     *Video
	Audio     []Audio
	Subs      []Subtitle
}

var textSubs = map[string]bool{"subrip": true, "srt": true, "ass": true, "ssa": true, "mov_text": true, "webvtt": true, "text": true}

type rawStream struct {
	Index       int               `json:"index"`
	CodecType   string            `json:"codec_type"`
	CodecName   string            `json:"codec_name"`
	Profile     string            `json:"profile"`
	PixFmt      string            `json:"pix_fmt"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Channels    int               `json:"channels"`
	BitsRaw     string            `json:"bits_per_raw_sample"`
	ColorTrc    string            `json:"color_transfer"`
	Disposition map[string]int    `json:"disposition"`
	Tags        map[string]string `json:"tags"`
	SideData    []struct {
		Type string `json:"side_data_type"`
	} `json:"side_data_list"`
}

type raw struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []rawStream `json:"streams"`
}

// Run ejecuta ffprobe (con tiempo límite: un disco USB dormido puede tardar).
func Run(ctx context.Context, ffprobe, path string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	return Parse(out)
}

// Parse convierte la salida JSON de ffprobe.
func Parse(data []byte) (*Result, error) {
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	res := &Result{Container: container(r.Format.FormatName)}
	res.Duration, _ = strconv.ParseFloat(r.Format.Duration, 64)
	res.Bitrate, _ = strconv.ParseInt(r.Format.BitRate, 10, 64)
	for _, s := range r.Streams {
		lang, title := s.Tags["language"], s.Tags["title"]
		if lang == "und" {
			lang = ""
		}
		switch s.CodecType {
		case "video":
			if res.Video != nil || s.Disposition["attached_pic"] == 1 {
				continue // carátulas embebidas y segundas pistas de vídeo
			}
			res.Video = &Video{Codec: s.CodecName, Profile: s.Profile, PixFmt: s.PixFmt, Width: s.Width, Height: s.Height, BitDepth: bitDepth(s), HDR: hdr(s)}
		case "audio":
			res.Audio = append(res.Audio, Audio{Index: s.Index, Codec: s.CodecName, Channels: s.Channels, Language: lang, Title: title, Default: s.Disposition["default"] == 1})
		case "subtitle":
			res.Subs = append(res.Subs, Subtitle{Index: s.Index, Codec: s.CodecName, Language: lang, Title: title,
				Default: s.Disposition["default"] == 1, Forced: s.Disposition["forced"] == 1, Text: textSubs[s.CodecName]})
		}
	}
	return res, nil
}

func container(formatName string) string {
	switch {
	case strings.Contains(formatName, "matroska"):
		return "mkv"
	case strings.Contains(formatName, "mp4"):
		return "mp4"
	case formatName == "avi":
		return "avi"
	case formatName == "mpegts":
		return "ts"
	}
	return formatName
}

func bitDepth(s rawStream) int {
	if n, err := strconv.Atoi(s.BitsRaw); err == nil && n > 0 {
		return n
	}
	switch {
	case strings.Contains(s.PixFmt, "10"):
		return 10
	case strings.Contains(s.PixFmt, "12"):
		return 12
	case s.PixFmt != "":
		return 8
	}
	return 0
}

func hdr(s rawStream) string {
	for _, sd := range s.SideData {
		if strings.Contains(sd.Type, "DOVI") || strings.Contains(sd.Type, "Dolby Vision") {
			return "DV"
		}
	}
	switch s.ColorTrc {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	return ""
}
