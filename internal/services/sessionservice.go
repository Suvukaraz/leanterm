package services

import (
	"sync"
	"time"

	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/session"
	"github.com/Suvukaraz/leanterm/internal/stats"
)

type SessionKind string

const (
	KindSSH      SessionKind = "ssh"
	KindTelnet   SessionKind = "telnet"
	KindLocal    SessionKind = "local"
	KindSFTPOnly SessionKind = "sftp"
)

type SessionInfo struct {
	TmpID     string      `json:"tmpID"`
	ConnID    string      `json:"connID"`
	Title     string      `json:"title"`
	Kind      SessionKind `json:"kind"`
	State     string      `json:"state"` // "connecting", "connected", "disconnected", "closed"
	Cluster   string      `json:"cluster"`
	StartedAt time.Time   `json:"startedAt"`
	HasSFTP   bool        `json:"hasSFTP"`
}

type OpenRequest struct {
	ConnIDs   []string `json:"connIDs"`
	Where     string   `json:"where"` // "tab" | "window"
	Cluster   string   `json:"cluster,omitempty"`
	Instances int      `json:"instances,omitempty"`
}

type AttachResult struct {
	ReplayB64 string `json:"replayB64"`
	LastSeq   uint64 `json:"lastSeq"`
}

type SessionService struct {
	mu    sync.Mutex
	store *config.Store
	mgr   *session.SessionManager
}

func NewSessionService(store *config.Store, mgr *session.SessionManager) *SessionService {
	return &SessionService{
		store: store,
		mgr:   mgr,
	}
}

func (ss *SessionService) SetManager(mgr *session.SessionManager) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.mgr = mgr
}

func (ss *SessionService) List() []SessionInfo {
	if ss.mgr != nil {
		mgrList := ss.mgr.List()
		res := make([]SessionInfo, len(mgrList))
		for i, item := range mgrList {
			res[i] = SessionInfo{
				TmpID:     item.TmpID,
				ConnID:    item.ConnID,
				Title:     item.Title,
				Kind:      SessionKind(item.Kind),
				State:     item.State,
				Cluster:   item.Cluster,
				StartedAt: item.StartedAt,
				HasSFTP:   item.HasSFTP,
			}
		}
		return res
	}
	return nil
}

func (ss *SessionService) Open(req OpenRequest) ([]string, error) {
	if ss.mgr != nil {
		return ss.mgr.Open(session.OpenRequest{
			ConnIDs:   req.ConnIDs,
			Where:     req.Where,
			Cluster:   req.Cluster,
			Instances: req.Instances,
		})
	}
	return nil, nil
}

func (ss *SessionService) OpenLocalShell(where string) (string, error) {
	if ss.mgr != nil {
		return ss.mgr.OpenLocalShell(where)
	}
	return "", nil
}

func (ss *SessionService) OpenQuick(quickID, title, method, host string, port int, user, password string) (string, error) {
	if ss.mgr != nil {
		return ss.mgr.OpenQuick(quickID, title, method, host, port, user, password)
	}
	return "", nil
}

func (ss *SessionService) Write(tmpID string, dataB64 string) error {
	if ss.mgr != nil {
		return ss.mgr.Write(tmpID, dataB64)
	}
	return nil
}

func (ss *SessionService) Resize(tmpID string, cols, rows int) error {
	if ss.mgr != nil {
		return ss.mgr.Resize(tmpID, cols, rows)
	}
	return nil
}

func (ss *SessionService) Ack(tmpID string, seq uint64) error {
	if ss.mgr != nil {
		return ss.mgr.Ack(tmpID, seq)
	}
	return nil
}

func (ss *SessionService) Disconnect(tmpID string) error {
	if ss.mgr != nil {
		return ss.mgr.Disconnect(tmpID)
	}
	return nil
}

func (ss *SessionService) Close(tmpID string) error {
	if ss.mgr != nil {
		return ss.mgr.Close(tmpID)
	}
	return nil
}

func (ss *SessionService) Attach(tmpID string) (AttachResult, error) {
	if ss.mgr != nil {
		replayB64, lastSeq, err := ss.mgr.Attach(tmpID)
		if err != nil {
			return AttachResult{}, err
		}
		return AttachResult{ReplayB64: replayB64, LastSeq: lastSeq}, nil
	}
	return AttachResult{}, nil
}

func (ss *SessionService) SendTelnetCommand(tmpID string, cmd string) error {
	if ss.mgr != nil {
		return ss.mgr.SendTelnetCommand(tmpID, cmd)
	}
	return nil
}

func (ss *SessionService) GetStats(connID string) stats.ConnectionStat {
	if ss.mgr != nil {
		return ss.mgr.GetStats(connID)
	}
	return stats.ConnectionStat{}
}

func (ss *SessionService) ResetStats(connID string) error {
	if ss.mgr != nil {
		return ss.mgr.ResetStats(connID)
	}
	return nil
}

func (ss *SessionService) GetHistory() []stats.HistoryEntry {
	if ss.mgr != nil {
		return ss.mgr.GetHistory()
	}
	return []stats.HistoryEntry{}
}

func (ss *SessionService) ClearHistory() error {
	if ss.mgr != nil {
		return ss.mgr.ClearHistory()
	}
	return nil
}

func (ss *SessionService) RunRemoteMacro(tmpID string, cmd string, appendReturn bool) error {
	if ss.mgr != nil {
		return ss.mgr.RunRemoteMacro(tmpID, cmd, appendReturn)
	}
	return nil
}

func (ss *SessionService) RunLocalMacro(tmpID string, cmd string) error {
	if ss.mgr != nil {
		return ss.mgr.RunLocalMacro(tmpID, cmd)
	}
	return nil
}

func (ss *SessionService) SaveSessionLog(tmpID string, path string, stripControl bool) error {
	if ss.mgr != nil {
		return ss.mgr.SaveSessionLog(tmpID, path, stripControl)
	}
	return nil
}

func (ss *SessionService) ExecuteChain(targetTmpID string, chainConnID string, selectedIndices []int) error {
	if ss.mgr != nil {
		return ss.mgr.ExecuteChain(targetTmpID, chainConnID, selectedIndices)
	}
	return nil
}

func (ss *SessionService) StartTee(tmpID string, filePath string) error {
	if ss.mgr != nil {
		return ss.mgr.StartTee(tmpID, filePath)
	}
	return nil
}

func (ss *SessionService) StopTee(tmpID string) error {
	if ss.mgr != nil {
		return ss.mgr.StopTee(tmpID)
	}
	return nil
}

func (ss *SessionService) StartPipe(tmpID string, command string, prompt string) error {
	if ss.mgr != nil {
		return ss.mgr.StartPipe(tmpID, command, prompt)
	}
	return nil
}

func (ss *SessionService) StopPipe(tmpID string) error {
	if ss.mgr != nil {
		return ss.mgr.StopPipe(tmpID)
	}
	return nil
}
