package editor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// ChunkSize is the maximum text size (bytes of UTF-8) of one transfer chunk
// between the backend and the editor window.
const ChunkSize = 4 << 20

// LoadResult describes a file read into memory; its text is fetched with ReadChunk.
type LoadResult struct {
	LoadID           string `json:"loadId"`
	Size             int64  `json:"size"`
	ModTime          int64  `json:"modTime"`
	Encoding         string `json:"encoding"`
	LineEnding       string `json:"lineEnding"`
	MixedLineEndings bool   `json:"mixedLineEndings"`
	// Binary is set when the file looks binary and allowBinary was false; nothing
	// is loaded then.
	Binary bool `json:"binary"`
}

// Chunk is one piece of a loaded text.
type Chunk struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// SaveOptions control CommitSave.
type SaveOptions struct {
	Encoding   string `json:"encoding"`
	LineEnding string `json:"lineEnding"`
	// ExpectedModTime/ExpectedSize are what the file had when it was loaded or last
	// saved (0/0 for a new file). A different state on disk is a conflict.
	ExpectedModTime int64 `json:"expectedModTime"`
	ExpectedSize    int64 `json:"expectedSize"`
	// Force writes even when the file changed on disk.
	Force bool `json:"force"`
}

// SaveResult is the outcome of CommitSave.
type SaveResult struct {
	// Conflict: the file was changed elsewhere since it was loaded; nothing was written.
	Conflict bool  `json:"conflict"`
	Size     int64 `json:"size"`
	ModTime  int64 `json:"modTime"`
}

type pendingLoad struct {
	text   string
	offset int
}

// Manager holds texts between Load and the last ReadChunk, and save buffers
// between BeginSave and CommitSave.
type Manager struct {
	mu     sync.Mutex
	nextID uint64
	loads  map[string]*pendingLoad
	saves  map[string]*strings.Builder
}

func NewManager() *Manager {
	return &Manager{loads: map[string]*pendingLoad{}, saves: map[string]*strings.Builder{}}
}

func (m *Manager) newID() string {
	m.nextID++
	return strconv.FormatUint(m.nextID, 10)
}

// Load reads and decodes path. enc "" detects the encoding.
func (m *Manager) Load(fsys FS, path, enc string, allowBinary bool) (LoadResult, error) {
	info, err := fsys.Stat(path)
	if err != nil {
		return LoadResult{}, err
	}
	if !info.Exists {
		return LoadResult{}, fmt.Errorf("%s does not exist", path)
	}
	if info.IsDir {
		return LoadResult{}, fmt.Errorf("%s is a folder", path)
	}
	raw, err := fsys.ReadFile(path)
	if err != nil {
		return LoadResult{}, err
	}
	res := LoadResult{Size: int64(len(raw)), ModTime: info.ModTime}
	if !allowBinary && LooksBinary(raw) {
		res.Binary = true
		return res, nil
	}
	text, usedEnc, err := Decode(raw, enc)
	if err != nil {
		return LoadResult{}, err
	}
	raw = nil
	res.Encoding = usedEnc
	res.LineEnding, res.MixedLineEndings = DetectLineEnding(text)
	text = NormalizeNewlines(text)

	m.mu.Lock()
	defer m.mu.Unlock()
	res.LoadID = m.newID()
	m.loads[res.LoadID] = &pendingLoad{text: text}
	return res, nil
}

// ReadChunk returns the next piece of a loaded text (never splitting a character).
// After the last chunk the text is released.
func (m *Manager) ReadChunk(loadID string) (Chunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loads[loadID]
	if !ok {
		return Chunk{}, errors.New("unknown or finished load")
	}
	end := l.offset + ChunkSize
	if end >= len(l.text) {
		end = len(l.text)
	} else {
		for end > l.offset && !utf8.RuneStart(l.text[end]) {
			end--
		}
	}
	chunk := Chunk{Text: l.text[l.offset:end]}
	l.offset = end
	if l.offset >= len(l.text) {
		chunk.Done = true
		delete(m.loads, loadID)
	}
	return chunk, nil
}

// BeginSave starts collecting the text of a save.
func (m *Manager) BeginSave() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.newID()
	m.saves[id] = &strings.Builder{}
	return id
}

// AppendSave adds the next piece of text to a save.
func (m *Manager) AppendSave(saveID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.saves[saveID]
	if !ok {
		return errors.New("unknown save")
	}
	b.WriteString(text)
	return nil
}

// Discard drops an unfinished load or save.
func (m *Manager) Discard(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.loads, id)
	delete(m.saves, id)
}

// CommitSave encodes the collected text and writes it to path, unless the file
// was changed elsewhere (see SaveOptions). The save buffer is released either way.
func (m *Manager) CommitSave(saveID string, fsys FS, path string, opts SaveOptions) (SaveResult, error) {
	m.mu.Lock()
	b, ok := m.saves[saveID]
	delete(m.saves, saveID)
	m.mu.Unlock()
	if !ok {
		return SaveResult{}, errors.New("unknown save")
	}

	if !opts.Force {
		cur, err := fsys.Stat(path)
		if err != nil {
			return SaveResult{}, err
		}
		existed := opts.ExpectedModTime != 0
		if cur.Exists && (!existed || cur.ModTime != opts.ExpectedModTime || cur.Size != opts.ExpectedSize) {
			return SaveResult{Conflict: true, Size: cur.Size, ModTime: cur.ModTime}, nil
		}
	}

	if opts.Encoding != "" && !ValidEncoding(opts.Encoding) {
		return SaveResult{}, fmt.Errorf("unsupported encoding %q", opts.Encoding)
	}
	data, err := Encode(b.String(), opts.Encoding, opts.LineEnding)
	if err != nil {
		return SaveResult{}, err
	}
	if err := fsys.WriteFile(path, data); err != nil {
		return SaveResult{}, err
	}
	info, err := fsys.Stat(path)
	if err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Size: info.Size, ModTime: info.ModTime}, nil
}
