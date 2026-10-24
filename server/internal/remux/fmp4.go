package remux

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Lectura del MP4 fragmentado que ffmpeg escribe por su salida estándar: ftyp+moov (segmento
// de inicio) y luego pares moof+mdat, uno por fotograma clave (frag_keyframe). El servidor
// agrupa los fragmentos en segmentos HLS según el plan, así los cortes no dependen de ffmpeg.

// maxBox: un fragmento de 4K a 15 Mb/s con 10 s entre claves ronda los 20 MB.
const maxBox = 512 << 20

// readBox lee una caja completa (cabecera incluida).
func readBox(r *bufio.Reader) (typ string, raw []byte, err error) {
	var h [16]byte
	if _, err = io.ReadFull(r, h[:8]); err != nil {
		return "", nil, err
	}
	size := int64(binary.BigEndian.Uint32(h[:4]))
	typ = string(h[4:8])
	hl := 8
	if size == 1 {
		if _, err = io.ReadFull(r, h[8:16]); err != nil {
			return "", nil, err
		}
		size = int64(binary.BigEndian.Uint64(h[8:16]))
		hl = 16
	}
	if size < int64(hl) || size > maxBox {
		return "", nil, fmt.Errorf("MP4: caja %q de tamaño %d no admitido", typ, size)
	}
	raw = make([]byte, size)
	copy(raw, h[:hl])
	if _, err = io.ReadFull(r, raw[hl:]); err != nil {
		return "", nil, err
	}
	return typ, raw, nil
}

// eachBox recorre las cajas hijas de un cuerpo. payloadOff: posición del cuerpo de cada hija dentro de b.
func eachBox(b []byte, fn func(typ string, payload []byte, payloadOff int) error) error {
	off := 0
	for off+8 <= len(b) {
		size := int(binary.BigEndian.Uint32(b[off:]))
		typ := string(b[off+4 : off+8])
		hl := 8
		if size == 1 {
			if off+16 > len(b) {
				return errors.New("MP4: caja truncada")
			}
			size = int(binary.BigEndian.Uint64(b[off+8:]))
			hl = 16
		} else if size == 0 {
			size = len(b) - off
		}
		if size < hl || off+size > len(b) {
			return errors.New("MP4: caja truncada")
		}
		if err := fn(typ, b[off+hl:off+size], off+hl); err != nil {
			return err
		}
		off += size
	}
	return nil
}

func boxBody(raw []byte) []byte {
	if binary.BigEndian.Uint32(raw) == 1 {
		return raw[16:]
	}
	return raw[8:]
}

type track struct {
	timescale uint32
	video     bool
}

// parseMoov devuelve las pistas por track_ID.
func parseMoov(raw []byte) (map[uint32]track, error) {
	tracks := map[uint32]track{}
	err := eachBox(boxBody(raw), func(typ string, p []byte, _ int) error {
		if typ != "trak" {
			return nil
		}
		var id uint32
		var t track
		eachBox(p, func(typ string, p []byte, _ int) error {
			switch typ {
			case "tkhd":
				if len(p) >= 24 {
					if p[0] == 1 {
						id = binary.BigEndian.Uint32(p[20:])
					} else {
						id = binary.BigEndian.Uint32(p[12:])
					}
				}
			case "mdia":
				eachBox(p, func(typ string, p []byte, _ int) error {
					switch typ {
					case "mdhd":
						if p[0] == 1 && len(p) >= 24 {
							t.timescale = binary.BigEndian.Uint32(p[20:])
						} else if len(p) >= 16 {
							t.timescale = binary.BigEndian.Uint32(p[12:])
						}
					case "hdlr":
						if len(p) >= 12 {
							t.video = string(p[8:12]) == "vide"
						}
					}
					return nil
				})
			}
			return nil
		})
		if id != 0 && t.timescale != 0 {
			tracks[id] = t
		}
		return nil
	})
	if len(tracks) == 0 && err == nil {
		err = errors.New("MP4: moov sin pistas")
	}
	return tracks, err
}

// fragInfo: inicio de presentación del primer vídeo del fragmento (s) y dónde está su número de secuencia.
type fragInfo struct {
	videoStart float64
	hasVideo   bool
	seqOff     int // posición del sequence_number de mfhd dentro de raw (−1 si no hay)
}

func parseMoof(raw []byte, tracks map[uint32]track) (fragInfo, error) {
	fi := fragInfo{seqOff: -1}
	hl := len(raw) - len(boxBody(raw))
	err := eachBox(boxBody(raw), func(typ string, p []byte, off int) error {
		switch typ {
		case "mfhd":
			if len(p) >= 8 {
				fi.seqOff = hl + off + 4
			}
		case "traf":
			var id uint32
			var base uint64
			var cts int64
			hasBase := false
			eachBox(p, func(typ string, p []byte, _ int) error {
				switch typ {
				case "tfhd":
					if len(p) >= 8 {
						id = binary.BigEndian.Uint32(p[4:])
					}
				case "tfdt":
					if p[0] == 1 && len(p) >= 12 {
						base, hasBase = binary.BigEndian.Uint64(p[4:]), true
					} else if len(p) >= 8 {
						base, hasBase = uint64(binary.BigEndian.Uint32(p[4:])), true
					}
				case "trun":
					cts = firstCTS(p)
				}
				return nil
			})
			t, ok := tracks[id]
			if ok && t.video && hasBase && !fi.hasVideo {
				fi.hasVideo = true
				fi.videoStart = (float64(base) + float64(cts)) / float64(t.timescale)
			}
		}
		return nil
	})
	return fi, err
}

// firstCTS: desplazamiento de composición de la primera muestra de un trun (0 si no viene).
func firstCTS(p []byte) int64 {
	if len(p) < 8 {
		return 0
	}
	version := p[0]
	flags := uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
	if binary.BigEndian.Uint32(p[4:]) == 0 || flags&0x800 == 0 {
		return 0
	}
	o := 8
	if flags&0x1 != 0 {
		o += 4
	}
	if flags&0x4 != 0 {
		o += 4
	}
	for _, f := range []uint32{0x100, 0x200, 0x400} {
		if flags&f != 0 {
			o += 4
		}
	}
	if o+4 > len(p) {
		return 0
	}
	v := binary.BigEndian.Uint32(p[o:])
	if version == 1 {
		return int64(int32(v))
	}
	return int64(v)
}
