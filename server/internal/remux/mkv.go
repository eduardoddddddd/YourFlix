package remux

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"
)

// Lectura mínima de Matroska (EBML) para sacar los fotogramas clave del vídeo del índice
// (Cues) sin leer el archivo entero: con un MKV de 40 GB, ffprobe tardaría minutos.

const (
	idEBML           = 0x1A45DFA3
	idSegment        = 0x18538067
	idSeekHead       = 0x114D9B74
	idSeek           = 0x4DBB
	idSeekID         = 0x53AB
	idSeekPosition   = 0x53AC
	idInfo           = 0x1549A966
	idTimestampScale = 0x2AD7B1
	idDuration       = 0x4489
	idTracks         = 0x1654AE6B
	idTrackEntry     = 0xAE
	idTrackNumber    = 0xD7
	idTrackType      = 0x83
	idCues           = 0x1C53BB6B
	idCuePoint       = 0xBB
	idCueTime        = 0xB3
	idCueTrackPos    = 0xB7
	idCueTrack       = 0xF7
	idCluster        = 0x1F43B675
)

const unknownSize = -1

var errNoCues = errors.New("el MKV no tiene índice de fotogramas clave")

// readVint lee un entero de tamaño variable EBML. keepMarker: los ID conservan el bit marcador.
func readVint(r io.ReaderAt, off int64, keepMarker bool) (val int64, n int, err error) {
	var b [8]byte
	if _, err = r.ReadAt(b[:1], off); err != nil {
		return 0, 0, err
	}
	first := b[0]
	n = 1
	for mask := byte(0x80); n <= 8 && first&mask == 0; mask >>= 1 {
		n++
	}
	if n > 8 || (keepMarker && n > 4) {
		return 0, 0, errors.New("EBML: entero no válido")
	}
	if n > 1 {
		if _, err = r.ReadAt(b[1:n], off+1); err != nil {
			return 0, 0, err
		}
	}
	v := uint64(first)
	if !keepMarker {
		v &= uint64(0xFF >> n)
	}
	allOnes := v == uint64(0xFF>>n)
	for i := 1; i < n; i++ {
		v = v<<8 | uint64(b[i])
		if b[i] != 0xFF {
			allOnes = false
		}
	}
	if !keepMarker && allOnes {
		return unknownSize, n, nil
	}
	return int64(v), n, nil
}

type element struct {
	id         int64
	start      int64 // inicio de los datos
	size       int64 // unknownSize si no se conoce
	headerSize int
}

func readElement(r io.ReaderAt, off int64) (element, error) {
	id, n1, err := readVint(r, off, true)
	if err != nil {
		return element{}, err
	}
	size, n2, err := readVint(r, off+int64(n1), false)
	if err != nil {
		return element{}, err
	}
	return element{id: id, start: off + int64(n1+n2), size: size, headerSize: n1 + n2}, nil
}

// children recorre los hijos de un elemento cuyo cuerpo está en memoria.
func children(body []byte, fn func(id int64, data []byte) error) error {
	r := bytesReaderAt(body)
	off := int64(0)
	for off < int64(len(body)) {
		e, err := readElement(r, off)
		if err != nil {
			return err
		}
		if e.size == unknownSize || e.start+e.size > int64(len(body)) {
			return errors.New("EBML: elemento truncado")
		}
		if err := fn(e.id, body[e.start:e.start+e.size]); err != nil {
			return err
		}
		off = e.start + e.size
	}
	return nil
}

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func ebmlUint(d []byte) int64 {
	var v int64
	for _, c := range d {
		v = v<<8 | int64(c)
	}
	return v
}

func ebmlFloat(d []byte) float64 {
	switch len(d) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(d)))
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(d))
	}
	return 0
}

// maxBody: Cues de una película de 3 h con un punto por fotograma clave ocupa < 5 MB.
const maxBody = 64 << 20

func readBody(r io.ReaderAt, e element) ([]byte, error) {
	if e.size == unknownSize || e.size > maxBody {
		return nil, errors.New("EBML: elemento demasiado grande")
	}
	b := make([]byte, e.size)
	_, err := r.ReadAt(b, e.start)
	if err == io.EOF && e.size == 0 {
		err = nil
	}
	return b, err
}

// MKVKeyframes devuelve los instantes (s) de los fotogramas clave de la primera pista de
// vídeo según el índice Cues del MKV, y la duración declarada.
func MKVKeyframes(r io.ReaderAt, fileSize int64) (keys []float64, duration float64, err error) {
	head, err := readElement(r, 0)
	if err != nil || head.id != idEBML || head.size == unknownSize {
		return nil, 0, errors.New("no es un archivo Matroska")
	}
	seg, err := readElement(r, head.start+head.size)
	if err != nil || seg.id != idSegment {
		return nil, 0, errors.New("Matroska sin Segment")
	}
	segEnd := fileSize
	if seg.size != unknownSize && seg.start+seg.size < segEnd {
		segEnd = seg.start + seg.size
	}

	pos := map[int64]int64{} // ID → posición absoluta del elemento de nivel 1
	var seekHeads []int64
	parseSeekHead := func(body []byte) error {
		return children(body, func(id int64, d []byte) error {
			if id != idSeek {
				return nil
			}
			var sid, spos int64 = 0, -1
			children(d, func(id int64, v []byte) error {
				switch id {
				case idSeekID:
					sid = ebmlUint(v)
				case idSeekPosition:
					spos = ebmlUint(v)
				}
				return nil
			})
			if sid != 0 && spos >= 0 {
				abs := seg.start + spos
				if sid == idSeekHead {
					seekHeads = append(seekHeads, abs)
				} else if _, ok := pos[sid]; !ok {
					pos[sid] = abs
				}
			}
			return nil
		})
	}

	// Elementos de nivel 1 hasta el primer Cluster (o todos si no hay SeekHead que diga dónde está Cues).
	off := seg.start
	for off < segEnd {
		e, err := readElement(r, off)
		if err != nil {
			break
		}
		if e.id == idCluster {
			if _, ok := pos[idCues]; ok || e.size == unknownSize {
				break
			}
		} else if _, ok := pos[e.id]; !ok {
			pos[e.id] = off
		}
		if e.id == idSeekHead {
			if b, err := readBody(r, e); err == nil {
				parseSeekHead(b)
			}
		}
		if e.size == unknownSize {
			break
		}
		off = e.start + e.size
	}
	for i := 0; i < len(seekHeads) && i < 4; i++ {
		if e, err := readElement(r, seekHeads[i]); err == nil && e.id == idSeekHead {
			if b, err := readBody(r, e); err == nil {
				parseSeekHead(b)
			}
		}
	}

	body := func(id int64) []byte {
		p, ok := pos[id]
		if !ok {
			return nil
		}
		e, err := readElement(r, p)
		if err != nil || e.id != id {
			return nil
		}
		b, err := readBody(r, e)
		if err != nil {
			return nil
		}
		return b
	}

	scale := int64(1000000)
	if b := body(idInfo); b != nil {
		children(b, func(id int64, d []byte) error {
			switch id {
			case idTimestampScale:
				if v := ebmlUint(d); v > 0 {
					scale = v
				}
			case idDuration:
				duration = ebmlFloat(d)
			}
			return nil
		})
	}
	duration = duration * float64(scale) / 1e9

	video := int64(-1)
	if b := body(idTracks); b != nil {
		children(b, func(id int64, d []byte) error {
			if id != idTrackEntry || video >= 0 {
				return nil
			}
			var num, typ int64
			children(d, func(id int64, v []byte) error {
				switch id {
				case idTrackNumber:
					num = ebmlUint(v)
				case idTrackType:
					typ = ebmlUint(v)
				}
				return nil
			})
			if typ == 1 {
				video = num
			}
			return nil
		})
	}
	if video < 0 {
		return nil, duration, errors.New("Matroska sin pista de vídeo")
	}

	b := body(idCues)
	if b == nil {
		return nil, duration, errNoCues
	}
	seen := map[int64]bool{}
	children(b, func(id int64, d []byte) error {
		if id != idCuePoint {
			return nil
		}
		t := int64(-1)
		forVideo := false
		children(d, func(id int64, v []byte) error {
			switch id {
			case idCueTime:
				t = ebmlUint(v)
			case idCueTrackPos:
				children(v, func(id int64, w []byte) error {
					if id == idCueTrack && ebmlUint(w) == video {
						forVideo = true
					}
					return nil
				})
			}
			return nil
		})
		if t >= 0 && forVideo && !seen[t] {
			seen[t] = true
			keys = append(keys, float64(t)*float64(scale)/1e9)
		}
		return nil
	})
	if len(keys) == 0 {
		return nil, duration, errNoCues
	}
	sort.Float64s(keys)
	return keys, duration, nil
}
