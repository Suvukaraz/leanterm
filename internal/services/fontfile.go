package services

import (
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf16"
)

// fontFace is one face of a font file: family and style as the font names them,
// and whether it is monospaced (post table isFixedPitch).
type fontFace struct {
	Family string
	Style  string
	Mono   bool
}

var errNotFont = errors.New("not a TrueType/OpenType font")

// parseFontFile reads the faces of a .ttf, .otf or .ttc file. Only the table
// directory and the 'name' and 'post' tables are read.
func parseFontFile(r io.ReaderAt) ([]fontFace, error) {
	var tag [4]byte
	if _, err := r.ReadAt(tag[:], 0); err != nil {
		return nil, err
	}
	if string(tag[:]) != "ttcf" {
		f, err := parseFace(r, 0)
		if err != nil {
			return nil, err
		}
		return []fontFace{f}, nil
	}
	// TrueType collection: header, then one offset per face.
	var hdr [12]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[8:])
	if n == 0 || n > 256 {
		return nil, errNotFont
	}
	offs := make([]byte, 4*n)
	if _, err := r.ReadAt(offs, 12); err != nil {
		return nil, err
	}
	var faces []fontFace
	for i := uint32(0); i < n; i++ {
		if f, err := parseFace(r, int64(binary.BigEndian.Uint32(offs[4*i:]))); err == nil {
			faces = append(faces, f)
		}
	}
	if len(faces) == 0 {
		return nil, errNotFont
	}
	return faces, nil
}

func parseFace(r io.ReaderAt, at int64) (fontFace, error) {
	var hdr [12]byte
	if _, err := r.ReadAt(hdr[:], at); err != nil {
		return fontFace{}, err
	}
	switch binary.BigEndian.Uint32(hdr[:4]) {
	case 0x00010000, 0x4F54544F /* OTTO */, 0x74727565 /* true */ :
	default:
		return fontFace{}, errNotFont
	}
	numTables := int(binary.BigEndian.Uint16(hdr[4:]))
	if numTables == 0 || numTables > 512 {
		return fontFace{}, errNotFont
	}
	dir := make([]byte, 16*numTables)
	if _, err := r.ReadAt(dir, at+12); err != nil {
		return fontFace{}, err
	}
	tables := map[string][2]uint32{}
	for i := 0; i < numTables; i++ {
		e := dir[16*i:]
		tables[string(e[:4])] = [2]uint32{binary.BigEndian.Uint32(e[8:]), binary.BigEndian.Uint32(e[12:])}
	}

	var face fontFace
	if t, ok := tables["name"]; ok && t[1] < 1<<20 {
		buf := make([]byte, t[1])
		if _, err := r.ReadAt(buf, int64(t[0])); err != nil {
			return fontFace{}, err
		}
		names := parseNameTable(buf)
		face.Family = firstNonEmpty(names[16], names[1])
		face.Style = firstNonEmpty(names[17], names[2])
	}
	if face.Family == "" {
		return fontFace{}, errNotFont
	}
	if t, ok := tables["post"]; ok && t[1] >= 16 {
		var post [16]byte
		if _, err := r.ReadAt(post[:], int64(t[0])); err == nil {
			face.Mono = binary.BigEndian.Uint32(post[12:]) != 0
		}
	}
	return face, nil
}

// parseNameTable returns name IDs 1, 2, 16 and 17, preferring Windows English
// (US) Unicode records, then any Windows Unicode record, then Mac Roman.
func parseNameTable(b []byte) map[int]string {
	out := map[int]string{}
	rank := map[int]int{}
	if len(b) < 6 {
		return out
	}
	count := int(binary.BigEndian.Uint16(b[2:]))
	strOff := int(binary.BigEndian.Uint16(b[4:]))
	for i := 0; i < count; i++ {
		rec := 6 + 12*i
		if rec+12 > len(b) {
			break
		}
		platform := binary.BigEndian.Uint16(b[rec:])
		encoding := binary.BigEndian.Uint16(b[rec+2:])
		lang := binary.BigEndian.Uint16(b[rec+4:])
		id := int(binary.BigEndian.Uint16(b[rec+6:]))
		length := int(binary.BigEndian.Uint16(b[rec+8:]))
		off := strOff + int(binary.BigEndian.Uint16(b[rec+10:]))
		if (id != 1 && id != 2 && id != 16 && id != 17) || off+length > len(b) {
			continue
		}
		raw := b[off : off+length]
		var s string
		r := 0
		switch {
		case platform == 3 && (encoding == 1 || encoding == 10):
			u := make([]uint16, len(raw)/2)
			for j := range u {
				u[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			s = string(utf16.Decode(u))
			r = 2
			if lang == 0x0409 {
				r = 3
			}
		case platform == 1 && encoding == 0:
			s = string(raw) // Mac Roman; family names are ASCII in practice
			r = 1
		default:
			continue
		}
		if s != "" && r > rank[id] {
			out[id] = s
			rank[id] = r
		}
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
