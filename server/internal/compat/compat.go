// Package compat aplica el perfil de la TV (medido en la fase 0) a cada archivo.
// El perfil es un dato: profiles/samsung-ls01b-2022.json, copiado aquí para incrustarlo.
package compat

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
)

//go:embed samsung-ls01b-2022.json
var defaultProfile []byte

type Profile struct {
	ID         string `json:"id"`
	Containers struct {
		Direct []string `json:"direct"`
	} `json:"containers"`
	Video struct {
		Direct   []string `json:"direct"`
		MaxWidth int      `json:"maxWidth"`
	} `json:"video"`
	Audio struct {
		Direct          []string `json:"direct"`
		SilentlyDropped []string `json:"silentlyDropped"`
	} `json:"audio"`
	Network struct {
		WarnAboveMbps float64 `json:"warnAboveMbps"`
	} `json:"network"`
}

// Load lee un perfil; ruta vacía = el incrustado.
func Load(path string) (*Profile, error) {
	data := defaultProfile
	if path != "" {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			return nil, err
		}
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("perfil: %w", err)
	}
	return &p, nil
}

type Verdict struct {
	Status  string   `json:"status"` // direct | warn | optimize
	Reasons []string `json:"reasons,omitempty"`
}

var containerNames = map[string]string{"mkv": "matroska", "mp4": "mp4", "avi": "avi"}

// Check decide si el archivo se reproduce tal cual en la TV.
func (p *Profile) Check(container string, bitrate int64, v *probe.Video, audio []probe.Audio, subs []probe.Subtitle) Verdict {
	out := Verdict{Status: "direct"}
	optimize := func(r string) { out.Status = "optimize"; out.Reasons = append(out.Reasons, r) }
	warn := func(r string) {
		if out.Status == "direct" {
			out.Status = "warn"
		}
		out.Reasons = append(out.Reasons, r)
	}
	if name := containerNames[container]; !has(p.Containers.Direct, name) {
		optimize("contenedor " + container + " sin probar en la TV")
	}
	if v == nil {
		optimize("sin pista de vídeo")
	} else {
		if !has(p.Video.Direct, v.Codec) {
			optimize("vídeo " + v.Codec + " no admitido")
		}
		if p.Video.MaxWidth > 0 && v.Width > p.Video.MaxWidth {
			optimize(fmt.Sprintf("resolución %d×%d", v.Width, v.Height))
		}
		if v.HDR == "DV" {
			warn("Dolby Vision sin probar")
		}
	}
	playable, dropped := 0, []string{}
	for _, a := range audio {
		switch {
		case has(p.Audio.Direct, a.Codec):
			playable++
		case has(p.Audio.SilentlyDropped, a.Codec):
			dropped = append(dropped, strings.ToUpper(a.Codec))
		default:
			dropped = append(dropped, strings.ToUpper(a.Codec)+" (sin probar)")
		}
	}
	switch {
	case len(audio) == 0:
		warn("sin audio")
	case playable == 0:
		optimize("audio " + strings.Join(dropped, ", ") + ": sin sonido en la TV")
	case len(dropped) > 0:
		warn("pistas de audio no disponibles: " + strings.Join(dropped, ", "))
	}
	img := 0
	for _, s := range subs {
		if !s.Text {
			img++
		}
	}
	if img > 0 {
		warn(fmt.Sprintf("%d subtítulos en imagen (PGS/DVD) no visibles", img))
	}
	if lim := p.Network.WarnAboveMbps; lim > 0 && float64(bitrate)/1e6 > lim {
		warn(fmt.Sprintf("%.0f Mb/s: puede cortarse por Wi-Fi", float64(bitrate)/1e6))
	}
	return out
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
