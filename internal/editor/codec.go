// Package editor backs LeanTerm's text editor window: decoding and encoding file
// contents (character set, line endings) and reading/writing local or remote files.
package editor

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Supported encodings (the editor's Tools → Encoding menu).
const (
	EncodingUTF8      = "utf-8"
	EncodingUTF8BOM   = "utf-8-bom"
	EncodingWin1252   = "windows-1252"
	EncodingISO88591  = "iso-8859-1"
	EncodingISO885915 = "iso-8859-15"
)

// Line endings.
const (
	LineEndingLF   = "lf"
	LineEndingCRLF = "crlf"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// binarySniffLen is how much of a file is checked for NUL bytes.
const binarySniffLen = 8192

func charmapFor(enc string) (*charmap.Charmap, bool) {
	switch enc {
	case EncodingWin1252:
		return charmap.Windows1252, true
	case EncodingISO88591:
		return charmap.ISO8859_1, true
	case EncodingISO885915:
		return charmap.ISO8859_15, true
	}
	return nil, false
}

// ValidEncoding reports whether enc is one of the supported encodings.
func ValidEncoding(enc string) bool {
	if enc == EncodingUTF8 || enc == EncodingUTF8BOM {
		return true
	}
	_, ok := charmapFor(enc)
	return ok
}

// LooksBinary reports whether data contains a NUL byte in its first 8 KiB.
func LooksBinary(data []byte) bool {
	if len(data) > binarySniffLen {
		data = data[:binarySniffLen]
	}
	return bytes.IndexByte(data, 0) >= 0
}

// Decode converts raw file bytes to text. With enc == "" the encoding is detected:
// a UTF-8 BOM gives utf-8-bom, valid UTF-8 gives utf-8 and anything else is read
// as Windows-1252 (which maps every byte). It returns the text and the encoding used.
func Decode(raw []byte, enc string) (string, string, error) {
	if enc == "" {
		switch {
		case bytes.HasPrefix(raw, utf8BOM):
			enc = EncodingUTF8BOM
		case utf8.Valid(raw):
			enc = EncodingUTF8
		default:
			enc = EncodingWin1252
		}
	}
	switch enc {
	case EncodingUTF8:
		return strings.ToValidUTF8(string(raw), "�"), enc, nil
	case EncodingUTF8BOM:
		return strings.ToValidUTF8(string(bytes.TrimPrefix(raw, utf8BOM)), "�"), enc, nil
	}
	cm, ok := charmapFor(enc)
	if !ok {
		return "", "", fmt.Errorf("unsupported encoding %q", enc)
	}
	out, err := cm.NewDecoder().Bytes(raw)
	if err != nil {
		return "", "", err
	}
	return string(out), enc, nil
}

// DetectLineEnding returns the dominant line ending of text and whether both
// kinds occur. Text without line breaks counts as LF.
func DetectLineEnding(text string) (string, bool) {
	crlf := strings.Count(text, "\r\n")
	lf := strings.Count(text, "\n") - crlf
	le := LineEndingLF
	if crlf > lf {
		le = LineEndingCRLF
	}
	return le, crlf > 0 && lf > 0
}

// NormalizeNewlines turns CRLF and lone CR into LF (the editor works with LF only).
func NormalizeNewlines(text string) string {
	if !strings.Contains(text, "\r") {
		return text
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

// Encode converts editor text (LF line breaks) to file bytes with the given
// encoding and line ending. Characters the encoding cannot represent are an error.
func Encode(text, enc, lineEnding string) ([]byte, error) {
	text = NormalizeNewlines(text)
	if lineEnding == LineEndingCRLF {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	switch enc {
	case EncodingUTF8, "":
		return []byte(text), nil
	case EncodingUTF8BOM:
		return append(append([]byte{}, utf8BOM...), text...), nil
	}
	cm, ok := charmapFor(enc)
	if !ok {
		return nil, fmt.Errorf("unsupported encoding %q", enc)
	}
	out := make([]byte, 0, len(text))
	line := 1
	for _, r := range text {
		if r == '\n' {
			line++
		}
		b, ok := cm.EncodeRune(r)
		if !ok {
			return nil, fmt.Errorf("character %q in line %d cannot be saved as %s", r, line, enc)
		}
		out = append(out, b)
	}
	return out, nil
}
