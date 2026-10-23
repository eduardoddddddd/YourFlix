package probe

import (
	"os"
	"testing"
)

// Salida real de ffprobe de «No hay salida» (1987): HEVC, 2 AAC, 3 PGS.
func TestParseRealFile(t *testing.T) {
	data, err := os.ReadFile("testdata/no_hay_salida.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.Container != "mkv" || r.Video == nil || r.Video.Codec != "hevc" || r.Video.Width != 1920 || r.Video.BitDepth != 8 {
		t.Fatalf("vídeo: %+v %+v", r, r.Video)
	}
	if len(r.Audio) != 2 || r.Audio[0].Language != "spa" || r.Audio[1].Channels != 6 {
		t.Fatalf("audio: %+v", r.Audio)
	}
	if len(r.Subs) != 3 || r.Subs[0].Text || r.Subs[0].Codec != "hdmv_pgs_subtitle" {
		t.Fatalf("subtítulos: %+v", r.Subs)
	}
	if r.Duration < 6800 || r.Bitrate == 0 {
		t.Fatalf("duración/tasa: %v %v", r.Duration, r.Bitrate)
	}
}
