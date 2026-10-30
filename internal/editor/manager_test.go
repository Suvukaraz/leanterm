package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readAll(t *testing.T, m *Manager, loadID string) string {
	t.Helper()
	var sb strings.Builder
	for {
		c, err := m.ReadChunk(loadID)
		require.NoError(t, err)
		sb.WriteString(c.Text)
		if c.Done {
			return sb.String()
		}
	}
}

func save(t *testing.T, m *Manager, fsys FS, path, text string, opts SaveOptions) SaveResult {
	t.Helper()
	id := m.BeginSave()
	require.NoError(t, m.AppendSave(id, text))
	res, err := m.CommitSave(id, fsys, path, opts)
	require.NoError(t, err)
	return res
}

func TestLoadAndSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.conf")
	require.NoError(t, os.WriteFile(path, []byte("key=1\r\nname=grüß\r\n"), 0o640))

	m := NewManager()
	res, err := m.Load(LocalFS{}, path, "", false)
	require.NoError(t, err)
	assert.Equal(t, EncodingUTF8, res.Encoding)
	assert.Equal(t, LineEndingCRLF, res.LineEnding)
	assert.Equal(t, "key=1\nname=grüß\n", readAll(t, m, res.LoadID))

	_, err = m.ReadChunk(res.LoadID)
	assert.Error(t, err, "load is released after the last chunk")

	out := save(t, m, LocalFS{}, path, "key=2\n", SaveOptions{
		Encoding: res.Encoding, LineEnding: res.LineEnding, ExpectedModTime: res.ModTime, ExpectedSize: res.Size,
	})
	assert.False(t, out.Conflict)
	data, _ := os.ReadFile(path)
	assert.Equal(t, "key=2\r\n", string(data))
	fi, _ := os.Stat(path)
	assert.Equal(t, os.FileMode(0o640), fi.Mode().Perm(), "permissions are kept")

	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1, "no temporary file is left behind")
}

func TestSaveDetectsConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	require.NoError(t, os.WriteFile(path, []byte("one"), 0o644))
	m := NewManager()
	res, err := m.Load(LocalFS{}, path, "", false)
	require.NoError(t, err)
	readAll(t, m, res.LoadID)

	// Someone else changes the file.
	require.NoError(t, os.WriteFile(path, []byte("other change"), 0o644))
	later := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(path, later, later))

	opts := SaveOptions{Encoding: EncodingUTF8, LineEnding: LineEndingLF, ExpectedModTime: res.ModTime, ExpectedSize: res.Size}
	out := save(t, m, LocalFS{}, path, "mine", opts)
	assert.True(t, out.Conflict)
	data, _ := os.ReadFile(path)
	assert.Equal(t, "other change", string(data), "nothing written on conflict")

	opts.Force = true
	out = save(t, m, LocalFS{}, path, "mine", opts)
	assert.False(t, out.Conflict)
	data, _ = os.ReadFile(path)
	assert.Equal(t, "mine", string(data))
}

func TestSaveNewFileAndExistingWithoutExpectation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	m := NewManager()
	out := save(t, m, LocalFS{}, path, "hello", SaveOptions{Encoding: EncodingUTF8})
	assert.False(t, out.Conflict)
	assert.Equal(t, int64(5), out.Size)

	// A file that appeared at a path the editor thought was new is a conflict.
	out = save(t, m, LocalFS{}, path, "again", SaveOptions{Encoding: EncodingUTF8})
	assert.True(t, out.Conflict)
}

func TestSaveThroughSymlinkKeepsLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need developer mode on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.txt")
	link := filepath.Join(dir, "link.txt")
	require.NoError(t, os.WriteFile(real, []byte("a"), 0o644))
	require.NoError(t, os.Symlink(real, link))

	m := NewManager()
	save(t, m, LocalFS{}, link, "b", SaveOptions{Encoding: EncodingUTF8, Force: true})
	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0, "link is still a link")
	data, _ := os.ReadFile(real)
	assert.Equal(t, "b", string(data))
}

func TestBinaryAndEncodingErrors(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "a.bin")
	require.NoError(t, os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 0, 0}, 0o644))
	m := NewManager()
	res, err := m.Load(LocalFS{}, bin, "", false)
	require.NoError(t, err)
	assert.True(t, res.Binary)
	assert.Empty(t, res.LoadID)

	res, err = m.Load(LocalFS{}, bin, "", true)
	require.NoError(t, err)
	assert.False(t, res.Binary)
	m.Discard(res.LoadID)

	_, err = m.Load(LocalFS{}, dir, "", false)
	assert.Error(t, err, "folders cannot be opened")

	id := m.BeginSave()
	require.NoError(t, m.AppendSave(id, "日本"))
	_, err = m.CommitSave(id, LocalFS{}, filepath.Join(dir, "latin.txt"), SaveOptions{Encoding: EncodingWin1252})
	assert.Error(t, err)
}

func TestReadChunkKeepsCharactersWhole(t *testing.T) {
	// Big enough for several chunks, with a 3-byte character at every position.
	text := strings.Repeat("€a", ChunkSize/2+7)
	m := NewManager()
	m.loads["x"] = &pendingLoad{text: text}
	var got strings.Builder
	chunks := 0
	for {
		c, err := m.ReadChunk("x")
		require.NoError(t, err)
		assert.True(t, strings.ToValidUTF8(c.Text, "?") == c.Text, "chunk %d is valid UTF-8", chunks)
		got.WriteString(c.Text)
		chunks++
		if c.Done {
			break
		}
	}
	assert.Greater(t, chunks, 1)
	assert.Equal(t, text, got.String())
}
