package compat

import (
	"bytes"
	"os"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
)

func TestEmbeddedProfileMatchesRepo(t *testing.T) {
	repo, err := os.ReadFile("../../../profiles/samsung-ls01b-2022.json")
	if err != nil {
		t.Skip("perfil del repo no disponible")
	}
	if !bytes.Equal(repo, defaultProfile) {
		t.Fatal("internal/compat/samsung-ls01b-2022.json difiere de profiles/: cópialo de nuevo")
	}
}

func TestCheck(t *testing.T) {
	p, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	hevc := &probe.Video{Codec: "hevc", Width: 1920, Height: 1040}
	cases := []struct {
		name      string
		container string
		bitrate   int64
		audio     []probe.Audio
		subs      []probe.Subtitle
		want      string
	}{
		{"No hay salida: 2 AAC + PGS", "mkv", 2_759_250, []probe.Audio{{Codec: "aac"}, {Codec: "aac"}}, []probe.Subtitle{{Codec: "hdmv_pgs_subtitle"}}, "warn"},
		{"solo DTS", "mkv", 8_000_000, []probe.Audio{{Codec: "dts"}}, nil, "optimize"},
		{"AC3 + DTS", "mkv", 8_000_000, []probe.Audio{{Codec: "ac3"}, {Codec: "dts"}}, nil, "warn"},
		{"directo", "mp4", 5_000_000, []probe.Audio{{Codec: "eac3"}}, []probe.Subtitle{{Codec: "subrip", Text: true}}, "direct"},
		{"remux 4K por Wi-Fi", "mkv", 60_000_000, []probe.Audio{{Codec: "eac3"}}, nil, "warn"},
		{"contenedor raro", "wmv", 5_000_000, []probe.Audio{{Codec: "aac"}}, nil, "optimize"},
	}
	for _, c := range cases {
		if got := p.Check(c.container, c.bitrate, hevc, c.audio, c.subs); got.Status != c.want {
			t.Errorf("%s: %s %v, want %s", c.name, got.Status, got.Reasons, c.want)
		}
	}
}
