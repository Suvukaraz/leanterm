package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeDetectsEncoding(t *testing.T) {
	text, enc, err := Decode([]byte("grüße"), "")
	require.NoError(t, err)
	assert.Equal(t, "grüße", text)
	assert.Equal(t, EncodingUTF8, enc)

	text, enc, err = Decode([]byte("\xEF\xBB\xBFhi"), "")
	require.NoError(t, err)
	assert.Equal(t, "hi", text)
	assert.Equal(t, EncodingUTF8BOM, enc)

	// "grüße" in Windows-1252 is not valid UTF-8.
	text, enc, err = Decode([]byte{'g', 'r', 0xFC, 0xDF, 'e', 0x80}, "")
	require.NoError(t, err)
	assert.Equal(t, "grüße€", text)
	assert.Equal(t, EncodingWin1252, enc)
}

func TestDecodeWithExplicitEncoding(t *testing.T) {
	text, _, err := Decode([]byte{0xA4}, EncodingISO885915)
	require.NoError(t, err)
	assert.Equal(t, "€", text)

	text, _, err = Decode([]byte{0xA4}, EncodingISO88591)
	require.NoError(t, err)
	assert.Equal(t, "¤", text)

	_, _, err = Decode([]byte("x"), "ebcdic")
	assert.Error(t, err)
}

func TestLineEndings(t *testing.T) {
	le, mixed := DetectLineEnding("a\r\nb\r\nc\n")
	assert.Equal(t, LineEndingCRLF, le)
	assert.True(t, mixed)

	le, mixed = DetectLineEnding("a\nb")
	assert.Equal(t, LineEndingLF, le)
	assert.False(t, mixed)

	assert.Equal(t, "a\nb\nc\n", NormalizeNewlines("a\r\nb\rc\n"))
}

func TestEncode(t *testing.T) {
	out, err := Encode("a\nb", EncodingUTF8, LineEndingCRLF)
	require.NoError(t, err)
	assert.Equal(t, []byte("a\r\nb"), out)

	out, err = Encode("ü", EncodingUTF8BOM, LineEndingLF)
	require.NoError(t, err)
	assert.Equal(t, []byte("\xEF\xBB\xBFü"), out)

	out, err = Encode("grüße €", EncodingWin1252, LineEndingLF)
	require.NoError(t, err)
	assert.Equal(t, []byte{'g', 'r', 0xFC, 0xDF, 'e', ' ', 0x80}, out)

	_, err = Encode("ok\n日本", EncodingWin1252, LineEndingLF)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 2")
}

func TestLooksBinary(t *testing.T) {
	assert.False(t, LooksBinary([]byte("plain text")))
	assert.True(t, LooksBinary([]byte{'E', 'L', 'F', 0, 1}))
}
