package services

import (
	"fmt"

	"github.com/Suvukaraz/leanterm/internal/cluster"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/session"
	"github.com/Suvukaraz/leanterm/internal/vars"
)

// PCCOptions specifies execution options for Power Cluster Controller sends.
type PCCOptions struct {
	SendToAll               bool `json:"sendToAll"`
	PreventSingleBroadcast  bool `json:"preventSingleBroadcast"`
	UseVariables            bool `json:"useVariables"`
	AppendReturn            bool `json:"appendReturn"`
}

// ClusterService exposes cluster and PCC operations to the frontend.
type ClusterService struct {
	clusterMgr *cluster.Manager
	sessionMgr *session.SessionManager
	store      *config.Store
}

// NewClusterService creates a new ClusterService.
func NewClusterService(
	clusterMgr *cluster.Manager,
	sessionMgr *session.SessionManager,
	store *config.Store,
) *ClusterService {
	return &ClusterService{
		clusterMgr: clusterMgr,
		sessionMgr: sessionMgr,
		store:      store,
	}
}

// List returns all active clusters and their session IDs.
func (cs *ClusterService) List() []cluster.ClusterInfo {
	if cs.clusterMgr == nil {
		return nil
	}
	return cs.clusterMgr.List()
}

// Create creates a new empty cluster.
func (cs *ClusterService) Create(name string) error {
	if cs.clusterMgr == nil {
		return fmt.Errorf("cluster manager not initialized")
	}
	return cs.clusterMgr.Create(name)
}

// Rename renames an existing cluster.
func (cs *ClusterService) Rename(oldName, newName string) error {
	if cs.clusterMgr == nil {
		return fmt.Errorf("cluster manager not initialized")
	}
	return cs.clusterMgr.Rename(oldName, newName)
}

// Delete removes a cluster.
func (cs *ClusterService) Delete(name string) {
	if cs.clusterMgr != nil {
		cs.clusterMgr.Delete(name)
	}
}

// Add adds a session to a cluster.
func (cs *ClusterService) Add(tmpID, clusterName string) {
	if cs.clusterMgr != nil {
		cs.clusterMgr.Add(tmpID, clusterName)
	}
}

// Remove detaches a session from its cluster.
func (cs *ClusterService) Remove(tmpID string) {
	if cs.clusterMgr != nil {
		cs.clusterMgr.Remove(tmpID)
	}
}

// GetCluster returns the cluster name for a session.
func (cs *ClusterService) GetCluster(tmpID string) string {
	if cs.clusterMgr == nil {
		return ""
	}
	return cs.clusterMgr.GetCluster(tmpID)
}

// SetPreventSingleBroadcast toggles keyboard broadcast for individual windows.
func (cs *ClusterService) SetPreventSingleBroadcast(prevent bool) {
	if cs.clusterMgr != nil {
		cs.clusterMgr.SetPreventSingleBroadcast(prevent)
	}
}

// SaveConfigured updates the configured clusters list for a connection.
func (cs *ClusterService) SaveConfigured(connID string, clusters []string) error {
	if cs.store == nil {
		return fmt.Errorf("store not initialized")
	}
	cfg := cs.store.Config()
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == connID {
			cfg.Connections[i].Clusters = clusters
			return cs.store.Save()
		}
	}
	return fmt.Errorf("connection '%s' not found", connID)
}

// AutoClusterMatches previews connection matches for an auto cluster rule.
func (cs *ClusterService) AutoClusterMatches(rule config.AutoClusterConfig) []string {
	if cs.store == nil {
		return nil
	}
	var matches []string
	cfg := cs.store.Config()
	for _, conn := range cfg.Connections {
		if m := cluster.MatchAutoClusters(conn, []config.AutoClusterConfig{rule}); m != "" {
			matches = append(matches, fmt.Sprintf("%s (%s)", conn.Title, conn.Host))
		}
	}
	return matches
}

// PCCSend broadcasts text commands to all members of a cluster or all open terminals.
func (cs *ClusterService) PCCSend(clusterName string, text string, opts PCCOptions) error {
	if cs.sessionMgr == nil {
		return fmt.Errorf("session manager not initialized")
	}

	var targetTmpIDs []string

	if opts.SendToAll {
		for _, info := range cs.sessionMgr.List() {
			if info.Kind != "sftp" && (info.State == "connected" || info.State == "connecting") {
				targetTmpIDs = append(targetTmpIDs, info.TmpID)
			}
		}
	} else if clusterName != "" && cs.clusterMgr != nil {
		targetTmpIDs = cs.clusterMgr.GetMembers(clusterName)
	}

	for _, tmpID := range targetTmpIDs {
		sess := cs.sessionMgr.Get(tmpID)
		if sess == nil {
			continue
		}

		sendText := text
		if opts.UseVariables {
			// Find connection details if available
			var substCtx vars.SubstContext
			if info := cs.sessionMgr.GetInfo(tmpID); info != nil && cs.store != nil {
				for _, conn := range cs.store.Config().Connections {
					if conn.ID == info.ConnID {
						substCtx = vars.SubstContext{
							UUID:  conn.ID,
							Name:  conn.Name,
							Title: conn.Title,
							IP:    conn.Host,
							Port:  conn.Port,
							User:  conn.Auth.User,
						}
						break
					}
				}
			}
			sendText = vars.Subst(sendText, substCtx).Text
		}

		if opts.AppendReturn {
			sendText += "\n"
		}

		_ = sess.Write([]byte(sendText))
	}

	return nil
}

// RestartAll restarts all sessions belonging to the cluster.
func (cs *ClusterService) RestartAll(clusterName string) error {
	if cs.clusterMgr == nil || cs.sessionMgr == nil {
		return nil
	}
	members := cs.clusterMgr.GetMembers(clusterName)
	for _, tmpID := range members {
		if sess := cs.sessionMgr.Get(tmpID); sess != nil {
			_ = sess.Restart()
		}
	}
	return nil
}

// CloseAll closes all sessions belonging to the cluster.
func (cs *ClusterService) CloseAll(clusterName string) error {
	if cs.clusterMgr == nil || cs.sessionMgr == nil {
		return nil
	}
	members := cs.clusterMgr.GetMembers(clusterName)
	for _, tmpID := range members {
		if sess := cs.sessionMgr.Get(tmpID); sess != nil {
			_ = sess.Close()
		}
	}
	return nil
}
