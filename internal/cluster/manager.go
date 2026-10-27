package cluster

import (
	"fmt"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/Suvukaraz/leanterm/internal/config"
)

// EventEmitter defines the interface to emit events to frontend or listeners.
type EventEmitter interface {
	Emit(event string, data any)
}

// ClusterInfo represents a cluster and its active member tmpIDs.
type ClusterInfo struct {
	Name     string   `json:"name"`
	Sessions []string `json:"sessions"`
}

// Manager manages runtime clusters, configured clusters, auto-clustering, and broadcast.
type Manager struct {
	mu                       sync.RWMutex
	clusters                 map[string]map[string]bool // clusterName -> set of tmpIDs
	sessionClusters          map[string]string          // tmpID -> clusterName
	preventSingleBroadcast   bool                       // from PCC
	emitter                  EventEmitter
	store                    *config.Store
}

// NewManager creates a new cluster Manager.
func NewManager(store *config.Store, emitter EventEmitter) *Manager {
	return &Manager{
		clusters:        make(map[string]map[string]bool),
		sessionClusters: make(map[string]string),
		store:           store,
		emitter:         emitter,
	}
}

// SetPreventSingleBroadcast toggles single window keyboard broadcast prevention.
func (m *Manager) SetPreventSingleBroadcast(prevent bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preventSingleBroadcast = prevent
}

// IsPreventSingleBroadcast returns the current setting.
func (m *Manager) IsPreventSingleBroadcast() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.preventSingleBroadcast
}

// List returns a list of all active clusters and their session IDs.
func (m *Manager) List() []ClusterInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listLocked()
}

func (m *Manager) listLocked() []ClusterInfo {
	var result []ClusterInfo
	for name, set := range m.clusters {
		var sessions []string
		for s := range set {
			sessions = append(sessions, s)
		}
		result = append(result, ClusterInfo{
			Name:     name,
			Sessions: sessions,
		})
	}
	return result
}

// Create creates a new empty runtime cluster.
func (m *Manager) Create(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if _, exists := m.clusters[name]; !exists {
		m.clusters[name] = make(map[string]bool)
		m.emitChangedLocked()
	}
	return nil
}

// Rename renames an existing cluster.
func (m *Manager) Rename(oldName, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if oldName == "" || newName == "" {
		return fmt.Errorf("cluster names cannot be empty")
	}
	sessions, exists := m.clusters[oldName]
	if !exists {
		return fmt.Errorf("cluster '%s' does not exist", oldName)
	}

	delete(m.clusters, oldName)
	m.clusters[newName] = sessions
	for s := range sessions {
		m.sessionClusters[s] = newName
	}

	m.emitChangedLocked()
	return nil
}

// Delete removes a cluster and detaches all members.
func (m *Manager) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sessions, exists := m.clusters[name]; exists {
		for s := range sessions {
			delete(m.sessionClusters, s)
		}
		delete(m.clusters, name)
		m.emitChangedLocked()
	}
}

// Add adds a session to a named cluster (removes from previous cluster if any).
func (m *Manager) Add(tmpID, clusterName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tmpID == "" || clusterName == "" {
		return
	}

	// Remove from old cluster if any
	if oldCluster, ok := m.sessionClusters[tmpID]; ok {
		if set, ok := m.clusters[oldCluster]; ok {
			delete(set, tmpID)
		}
	}

	if _, exists := m.clusters[clusterName]; !exists {
		m.clusters[clusterName] = make(map[string]bool)
	}
	m.clusters[clusterName][tmpID] = true
	m.sessionClusters[tmpID] = clusterName

	m.emitChangedLocked()
}

// Remove removes a session from its cluster.
func (m *Manager) Remove(tmpID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clusterName, ok := m.sessionClusters[tmpID]; ok {
		if set, ok := m.clusters[clusterName]; ok {
			delete(set, tmpID)
		}
		delete(m.sessionClusters, tmpID)
		m.emitChangedLocked()
	}
}

// GetCluster returns the cluster name for a session, or empty if unclustered.
func (m *Manager) GetCluster(tmpID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionClusters[tmpID]
}

// GetMembers returns all session IDs belonging to the cluster.
func (m *Manager) GetMembers(clusterName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	set, exists := m.clusters[clusterName]
	if !exists {
		return nil
	}

	var members []string
	for s := range set {
		members = append(members, s)
	}
	return members
}

// MatchAutoClusters checks a connection against configured auto clusters and returns the matched cluster name.
func MatchAutoClusters(conn config.Connection, autoClusters []config.AutoClusterConfig) string {
	for _, ac := range autoClusters {
		if ac.Name == "" {
			continue
		}

		matchedAll := true

		if ac.MatchName != "" {
			if !regexMatches(ac.MatchName, conn.Name) {
				matchedAll = false
			}
		}
		if ac.MatchTitle != "" {
			if !regexMatches(ac.MatchTitle, conn.Title) {
				matchedAll = false
			}
		}
		if ac.MatchHost != "" {
			if !regexMatches(ac.MatchHost, conn.Host) {
				matchedAll = false
			}
		}
		if ac.MatchDescription != "" {
			if !regexMatches(ac.MatchDescription, conn.Description) {
				matchedAll = false
			}
		}

		if matchedAll && (ac.MatchName != "" || ac.MatchTitle != "" || ac.MatchHost != "" || ac.MatchDescription != "") {
			return ac.Name
		}
	}
	return ""
}

func regexMatches(pattern, text string) bool {
	re, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return false
	}
	matched, _ := re.MatchString(text)
	return matched
}

func (m *Manager) emitChangedLocked() {
	if m.emitter != nil {
		list := m.listLocked()
		m.emitter.Emit("cluster:changed", list)
	}
}
