package stats

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

type ConnectionStat struct {
	FirstStart       time.Time `toml:"first_start" json:"firstStart"`
	LastStart        time.Time `toml:"last_start" json:"lastStart"`
	LastStop         time.Time `toml:"last_stop" json:"lastStop"`
	TotalConnections int       `toml:"total_connections" json:"totalConnections"`
	TotalSeconds     int64     `toml:"total_seconds" json:"totalSeconds"`
}

type StatsFile struct {
	Connections map[string]ConnectionStat `toml:"connections"`
}

type HistoryEntry struct {
	ID        string    `toml:"id" json:"id"`
	Name      string    `toml:"name" json:"name"`
	Method    string    `toml:"method" json:"method"`
	StartedAt time.Time `toml:"started_at" json:"startedAt"`
}

type HistoryFile struct {
	Entries []HistoryEntry `toml:"entry"`
}

type Manager struct {
	mu        sync.Mutex
	configDir string
	stats     StatsFile
	history   HistoryFile
}

func NewManager(configDir string) *Manager {
	m := &Manager{
		configDir: configDir,
		stats: StatsFile{
			Connections: make(map[string]ConnectionStat),
		},
		history: HistoryFile{
			Entries: make([]HistoryEntry, 0),
		},
	}
	_ = m.load()
	return m
}

func (m *Manager) stateDir() string {
	return filepath.Join(m.configDir, "state")
}

func (m *Manager) statsPath() string {
	return filepath.Join(m.stateDir(), "stats.toml")
}

func (m *Manager) historyPath() string {
	return filepath.Join(m.stateDir(), "history.toml")
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_ = os.MkdirAll(m.stateDir(), 0755)

	// Load stats.toml
	if data, err := os.ReadFile(m.statsPath()); err == nil && len(data) > 0 {
		var sf StatsFile
		if err := toml.Unmarshal(data, &sf); err == nil && sf.Connections != nil {
			m.stats = sf
		}
	}

	// Load history.toml
	if data, err := os.ReadFile(m.historyPath()); err == nil && len(data) > 0 {
		var hf HistoryFile
		if err := toml.Unmarshal(data, &hf); err == nil && hf.Entries != nil {
			m.history = hf
		}
	}

	return nil
}

func (m *Manager) RecordStart(connID, name, method string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Update stats
	st, exists := m.stats.Connections[connID]
	if !exists {
		st = ConnectionStat{
			FirstStart: now,
		}
	}
	st.TotalConnections++
	st.LastStart = now
	m.stats.Connections[connID] = st

	// Update history (newest first, max 500)
	entry := HistoryEntry{
		ID:        connID,
		Name:      name,
		Method:    method,
		StartedAt: now,
	}
	newEntries := append([]HistoryEntry{entry}, m.history.Entries...)
	if len(newEntries) > 500 {
		newEntries = newEntries[:500]
	}
	m.history.Entries = newEntries

	_ = m.saveStatsLocked()
	_ = m.saveHistoryLocked()
}

func (m *Manager) RecordStop(connID string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, exists := m.stats.Connections[connID]
	if !exists {
		return
	}

	st.LastStop = time.Now()
	st.TotalSeconds += int64(duration.Seconds())
	m.stats.Connections[connID] = st

	_ = m.saveStatsLocked()
}

func (m *Manager) GetStats(connID string) ConnectionStat {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats.Connections[connID]
}

func (m *Manager) ResetStats(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.stats.Connections, connID)
	return m.saveStatsLocked()
}

func (m *Manager) GetHistory() []HistoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	res := make([]HistoryEntry, len(m.history.Entries))
	copy(res, m.history.Entries)
	return res
}

func (m *Manager) ClearHistory() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history.Entries = make([]HistoryEntry, 0)
	return m.saveHistoryLocked()
}

func (m *Manager) saveStatsLocked() error {
	data, err := toml.Marshal(m.stats)
	if err != nil {
		return err
	}
	return atomicWriteFile(m.statsPath(), data)
}

func (m *Manager) saveHistoryLocked() error {
	data, err := toml.Marshal(m.history)
	if err != nil {
		return err
	}
	return atomicWriteFile(m.historyPath(), data)
}

func atomicWriteFile(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, targetPath)
}
