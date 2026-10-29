package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// UIState holds window/panel layout that is remembered between runs (state/ui.toml).
type UIState struct {
	MainWindow          WindowGeometry `toml:"main_window" json:"mainWindow"`
	EditorWindow        WindowGeometry `toml:"editor_window" json:"editorWindow"`
	CommandPanelWidth   int            `toml:"tree_pane" json:"commandPanelWidth"`
	CommandPanelVisible bool           `toml:"tree_visible" json:"commandPanelVisible"`
	ClustersVisible     bool           `toml:"clusters_visible" json:"clustersVisible"`
	SFTPVisible         bool           `toml:"sftp_visible" json:"sftpVisible"`
	SFTPPanelWidth      int            `toml:"sftp_panel_width" json:"sftpPanelWidth"`
	ActiveLeftTab       string         `toml:"active_left_tab" json:"activeLeftTab"`
	ExpandedGroups      []string       `toml:"expanded_groups" json:"expandedGroups"`
	// Position and size of movable dialogs (Preferences, Edit Connection, About),
	// keyed by dialog name, in CSS pixels of the main window.
	Dialogs map[string]DialogGeometry `toml:"dialogs,omitempty" json:"dialogs"`
}

// DialogGeometry is the remembered position and size of an in-window dialog.
type DialogGeometry struct {
	X      int `toml:"x" json:"x"`
	Y      int `toml:"y" json:"y"`
	Width  int `toml:"width" json:"width"`
	Height int `toml:"height" json:"height"`
}

// WindowGeometry is the size/position of a top-level window.
type WindowGeometry struct {
	Maximized bool `toml:"maximized" json:"maximized"`
	X         int  `toml:"x" json:"x"`
	Y         int  `toml:"y" json:"y"`
	Width     int  `toml:"width" json:"width"`
	Height    int  `toml:"height" json:"height"`
}

// DefaultUIState is used when state/ui.toml does not exist yet.
func DefaultUIState() UIState {
	return UIState{
		MainWindow:          WindowGeometry{Width: 1200, Height: 760},
		CommandPanelWidth:   260,
		CommandPanelVisible: true,
		SFTPVisible:         true,
		SFTPPanelWidth:      0, // 0 = use sftp_browser.width
		ActiveLeftTab:       "",
	}
}

var uiStateMu sync.Mutex

func (s *Store) uiStatePath() string {
	return filepath.Join(s.ConfigDir(), "state", "ui.toml")
}

// LoadUIState reads state/ui.toml; missing or broken files yield the defaults.
func (s *Store) LoadUIState() UIState {
	uiStateMu.Lock()
	defer uiStateMu.Unlock()

	st := DefaultUIState()
	data, err := os.ReadFile(s.uiStatePath())
	if err != nil {
		return st
	}
	if err := toml.Unmarshal(data, &st); err != nil {
		return DefaultUIState()
	}
	if st.CommandPanelWidth < 150 {
		st.CommandPanelWidth = 260
	}
	return st
}

// SaveUIState writes state/ui.toml atomically (never in read-only mode).
func (s *Store) SaveUIState(st UIState) error {
	if s.opts.ReadOnly {
		return nil
	}
	uiStateMu.Lock()
	defer uiStateMu.Unlock()

	data, err := toml.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.uiStatePath()), 0700); err != nil {
		return err
	}
	return s.atomicWrite(s.uiStatePath(), data)
}
