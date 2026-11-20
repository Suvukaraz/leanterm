package main

import (
	"fmt"
	"image"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
)

// reloadSettings re-reads the configuration after a settings:changed event
// (the configuration was saved or reloaded).
func (u *ui) reloadSettings() {
	cfg := u.core.Store.Config()
	u.cfg = cfg

	scheme := cfg.General.ColorScheme
	if scheme == "" {
		scheme = "dark"
	}
	if u.th != nil && u.scheme != scheme {
		u.scheme = scheme
		u.th.SetScheme(scheme)
	}

	// The tree follows the configuration.
	u.rebuildTree()

	u.notesNode = "" // force the notes to reload from the selection
}

// loadUIState reads the persisted UI state (window size, panel geometry,
// expanded groups, the clusters visibility).
func (u *ui) loadUIState() {
	st := u.core.Store.LoadUIState()
	u.panelWidth = st.CommandPanelWidth
	if u.panelWidth >= 150 {
		u.panel.Pos = u.panelWidth
	}
	u.panelVisible = st.CommandPanelVisible
	u.clustersVisible = st.ClustersVisible
	u.sftpVisible = st.SFTPVisible
	u.activeLeftTab = st.ActiveLeftTab
	if u.activeLeftTab == "" {
		u.activeLeftTab = "connections"
	}
	if u.tree.Expanded == nil {
		u.tree.Expanded = map[string]bool{}
	}
	// The root is always expanded at startup (TreeView.svelte); the saved
	// state only lists groups.
	u.tree.Expanded[""] = true
	for _, g := range st.ExpandedGroups {
		u.tree.Expanded[g] = true
	}
}

// saveUIState writes the persisted UI state.
func (u *ui) saveUIState() {
	st := u.core.Store.LoadUIState()
	if u.panel.Pos >= 150 {
		u.panelWidth = u.panel.Pos
	}
	st.CommandPanelWidth = u.panelWidth
	st.CommandPanelVisible = u.panelVisible
	st.ClustersVisible = u.clustersVisible
	st.SFTPVisible = u.sftpVisible
	st.ActiveLeftTab = u.activeLeftTab
	st.ExpandedGroups = expandedGroupIDs(u.rawTree, u.tree.Expanded)
	if u.frameSize != (image.Point{}) {
		st.MainWindow.Width = u.frameSize.X
		st.MainWindow.Height = u.frameSize.Y
	}
	_ = u.core.Store.SaveUIState(st)
}

// expandedGroupIDs lists the expanded groups in tree order.
func expandedGroupIDs(root *config.TreeNode, expanded map[string]bool) []string {
	var out []string
	var walk func(n *config.TreeNode)
	walk = func(n *config.TreeNode) {
		if n == nil {
			return
		}
		if n.IsGroup && expanded[n.ID] {
			out = append(out, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}

// noteUIStateSoon marks the UI state dirty; it is saved on exit and on
// important changes.
func (u *ui) noteUIStateSoon() {
	// The uistate is saved eagerly on each change to keep it simple and
	// crash-safe (it is a small file).
	u.saveUIState()
}

// formatDuration renders seconds as hh:mm:ss.
func formatDuration(totalSeconds int64) string {
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	return pad2(h) + ":" + pad2(m) + ":" + pad2(s)
}

func pad2(n int64) string {
	return fmt.Sprintf("%02d", n)
}

// formatLastConn renders a stat timestamp; zero time => "Never".
func formatLastConn(t time.Time) string {
	if t.IsZero() || t.Year() <= 1 {
		return "Never"
	}
	return t.Format("01/02/2006, 15:04:05")
}
