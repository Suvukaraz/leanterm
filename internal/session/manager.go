package session

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/config"
	"github.com/Suvukaraz/leanterm/internal/secrets"
	"github.com/Suvukaraz/leanterm/internal/stats"
	"github.com/Suvukaraz/leanterm/internal/telnet"
	"github.com/Suvukaraz/leanterm/internal/vars"
	"golang.org/x/crypto/ssh"
)

type SessionInfo struct {
	TmpID     string    `json:"tmpID"`
	ConnID    string    `json:"connID"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`  // "ssh", "telnet", "local", "sftp"
	State     string    `json:"state"` // "connecting", "connected", "disconnected", "closed"
	Cluster   string    `json:"cluster,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	HasSFTP   bool      `json:"hasSFTP"`
}

type OpenRequest struct {
	ConnIDs   []string `json:"connIDs"`
	Where     string   `json:"where"` // "tab" | "window"
	Cluster   string   `json:"cluster,omitempty"`
	Instances int      `json:"instances,omitempty"`
}

type Session interface {
	TmpID() string
	Title() string
	Kind() string
	State() string
	Stream() *Stream
	Start() error
	Write(data []byte) error
	Resize(cols, rows int) error
	Ack(seq uint64)
	Disconnect() error
	Close() error
	Restart() error
}

type SessionManager struct {
	mu         sync.Mutex
	store      *config.Store
	vault      *secrets.Vault
	emitter    EventEmitter
	sessions   map[string]Session
	infos      map[string]*SessionInfo
	statsMgr   *stats.Manager
	clusterMgr ClusterBroadcastProvider
	pipeTeeMgr *PipeTeeManager
	kpClient   vars.KeePassResolver
	prompter   Prompter
	// automations holds the per-session automation (expect, logs, reconnect, ...).
	automations map[string]*automation
}

// SetPrompter connects backend dialogs (passwords, host keys) to the UI.
func (sm *SessionManager) SetPrompter(p Prompter) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.prompter = p
}

// secretSaver returns a callback that stores a secret the user asked to remember.
func (sm *SessionManager) secretSaver(connID string) func(field, value string) {
	if sm.vault == nil || connID == "" || strings.HasPrefix(connID, "quick-") {
		return nil
	}
	return func(field, value string) {
		sm.vault.SetConnection(connID, field, value)
	}
}

// ClusterBroadcastProvider provides cluster members and settings.
type ClusterBroadcastProvider interface {
	GetCluster(tmpID string) string
	GetMembers(clusterName string) []string
	IsPreventSingleBroadcast() bool
}

func (sm *SessionManager) SetClusterManager(cm ClusterBroadcastProvider) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.clusterMgr = cm
}

func (sm *SessionManager) SetKeePassClient(kp vars.KeePassResolver) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kpClient = kp
}

func NewSessionManager(store *config.Store, vault *secrets.Vault, emitter EventEmitter) *SessionManager {
	var statsMgr *stats.Manager
	if store != nil {
		statsMgr = stats.NewManager(store.ConfigDir())
	}
	return &SessionManager{
		store:       store,
		vault:       vault,
		emitter:     emitter,
		sessions:    make(map[string]Session),
		infos:       make(map[string]*SessionInfo),
		automations: make(map[string]*automation),
		statsMgr:    statsMgr,
		pipeTeeMgr:  NewPipeTeeManager(emitter),
	}
}

func (sm *SessionManager) List() []SessionInfo {
	sm.mu.Lock()
	type pair struct {
		info SessionInfo
		sess Session
	}
	pairs := make([]pair, 0, len(sm.infos))
	for _, info := range sm.infos {
		pairs = append(pairs, pair{info: *info, sess: sm.sessions[info.TmpID]})
	}
	sm.mu.Unlock()

	result := make([]SessionInfo, 0, len(pairs))
	for _, p := range pairs {
		if p.sess != nil {
			p.info.State = p.sess.State()
		}
		result = append(result, p.info)
	}
	// Stable tab order: oldest session first.
	sort.Slice(result, func(i, j int) bool { return result[i].StartedAt.Before(result[j].StartedAt) })
	return result
}

func (sm *SessionManager) Get(tmpID string) Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[tmpID]
}

func (sm *SessionManager) GetInfo(tmpID string) *SessionInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.infos[tmpID]
}

func (sm *SessionManager) GetSSHClient(tmpID string) *ssh.Client {
	sess := sm.Get(tmpID)
	if sshSess, ok := sess.(*SSHSession); ok {
		return sshSess.SSHClient()
	}
	return nil
}

// LocalShellPID returns the shell process id of a local session (0 if tmpID is
// not a running local shell).
func (sm *SessionManager) LocalShellPID(tmpID string) int {
	if ls, ok := sm.Get(tmpID).(*LocalSession); ok {
		return ls.ShellPID()
	}
	return 0
}

func (sm *SessionManager) Open(req OpenRequest) ([]string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var tmpIDs []string
	cfg := sm.store.Config()

	for _, connID := range req.ConnIDs {
		var conn *config.Connection
		for i := range cfg.Connections {
			if cfg.Connections[i].ID == connID {
				conn = &cfg.Connections[i]
				break
			}
		}
		if conn == nil {
			continue
		}

		instances := req.Instances
		if instances <= 0 {
			instances = 1
		}

		for inst := 0; inst < instances; inst++ {
			tmpID := uuid.NewString()

			var pwd, keyPass, socksPass, jumpPass string
			if sm.vault != nil {
				pwd = sm.vault.GetConnection(connID, "password")
				keyPass = sm.vault.GetConnection(connID, "key_passphrase")
				socksPass = sm.vault.GetConnection(connID, "socks_password")
				jumpPass = sm.vault.GetConnection(connID, "jump_password")
			}

			var sess Session
			hasSFTP := false
			connCopy := *conn
			auto := newAutomation(sm, tmpID, connCopy, *cfg)

			switch conn.Method {
			case "ssh", "sftp":
				hasSFTP = true
				sshSess := NewSSHSession(SSHSessionOptions{
					TmpID:         tmpID,
					Conn:          *conn,
					Password:      pwd,
					KeyPassphrase: keyPass,
					SocksPassword: socksPass,
					JumpPassword:  jumpPass,
					GlobalNet:     cfg.Network,
					AutoAccept:    cfg.General.AutoAcceptHostKeys,
					Emitter:       auto,
					Prompter:      sm.prompter,
					SaveSecret:    sm.secretSaver(conn.ID),
					Shellless:     conn.Method == "sftp",
				})
				sess = sshSess
				auto.attach(sess)
				go func() {
					auto.runPrePost("PRE_EXEC", connCopy.PreExec)
					_ = sshSess.Start()
				}()

			case "telnet":
				hasSFTP = false
				telnetSess := NewTelnetSession(TelnetSessionOptions{
					TmpID:         tmpID,
					Conn:          *conn,
					Password:      pwd,
					SocksPassword: socksPass,
					JumpPassword:  jumpPass,
					GlobalNet:     cfg.Network,
					Emitter:       auto,
				})
				sess = telnetSess
				auto.attach(sess)
				go func() {
					auto.runPrePost("PRE_EXEC", connCopy.PreExec)
					_ = telnetSess.Start()
				}()

			default:
				// Unknown methods (should not pass validation) fall back to a local shell.
				auto = nil
				localCfg := cfg.LocalShell
				localSess := NewLocalSession(tmpID, localCfg, sm.emitter)
				sess = localSess
				go func() {
					_ = localSess.Start()
				}()
			}

			info := &SessionInfo{
				TmpID:     tmpID,
				ConnID:    conn.ID,
				Title:     conn.Title,
				Kind:      conn.Method,
				State:     "connecting",
				Cluster:   req.Cluster,
				StartedAt: time.Now(),
				HasSFTP:   hasSFTP,
			}

			sm.sessions[tmpID] = sess
			sm.infos[tmpID] = info
			if auto != nil {
				sm.automations[tmpID] = auto
			}
			tmpIDs = append(tmpIDs, tmpID)

			if sm.statsMgr != nil && conn.ID != "" && !strings.HasPrefix(conn.ID, "quick-") {
				sm.statsMgr.RecordStart(conn.ID, conn.Title, conn.Method)
			}

			if sm.emitter != nil {
				sm.emitter.Emit("session:opened", map[string]any{
					"session": info,
					"where":   req.Where,
				})
			}
		}
	}

	return tmpIDs, nil
}

func (sm *SessionManager) OpenLocalShell(where string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tmpID := uuid.NewString()
	localCfg := sm.store.Config().LocalShell

	sess := NewLocalSession(tmpID, localCfg, sm.emitter)
	info := &SessionInfo{
		TmpID:     tmpID,
		ConnID:    "local-shell",
		Title:     "Local Shell",
		Kind:      "local",
		State:     "connecting",
		StartedAt: time.Now(),
		HasSFTP:   false,
	}

	sm.sessions[tmpID] = sess
	sm.infos[tmpID] = info

	// Start asynchronous shell
	go func() {
		_ = sess.Start()
	}()

	if sm.emitter != nil {
		sm.emitter.Emit("session:opened", map[string]any{
			"session": info,
			"where":   where,
		})
	}

	return tmpID, nil
}

func (sm *SessionManager) OpenQuick(quickID, title, method, host string, port int, user, password string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tmpID := quickID
	if tmpID == "" {
		tmpID = uuid.NewString()
	}

	cfg := sm.store.Config()

	conn := config.Connection{
		ID:     tmpID,
		Name:   title,
		Title:  title,
		Method: method,
		Host:   host,
		Port:   port,
		Auth: config.ConnectionAuth{
			Type: "password",
			User: user,
		},
	}

	var sess Session
	hasSFTP := false

	switch method {
	case "ssh", "sftp":
		hasSFTP = true
		sshSess := NewSSHSession(SSHSessionOptions{
			TmpID:      tmpID,
			Conn:       conn,
			Password:   password,
			GlobalNet:  cfg.Network,
			AutoAccept: cfg.General.AutoAcceptHostKeys,
			Emitter:    sm.emitter,
			Prompter:   sm.prompter,
			Shellless:  method == "sftp",
		})
		sess = sshSess
		go func() { _ = sshSess.Start() }()

	case "telnet":
		hasSFTP = false
		telnetSess := NewTelnetSession(TelnetSessionOptions{
			TmpID:     tmpID,
			Conn:      conn,
			Password:  password,
			GlobalNet: cfg.Network,
			Emitter:   sm.emitter,
		})
		sess = telnetSess
		go func() { _ = telnetSess.Start() }()

	default:
		return "", fmt.Errorf("unsupported quick connect method: %s", method)
	}

	info := &SessionInfo{
		TmpID:     tmpID,
		ConnID:    tmpID,
		Title:     title,
		Kind:      method,
		State:     "connecting",
		StartedAt: time.Now(),
		HasSFTP:   hasSFTP,
	}

	sm.sessions[tmpID] = sess
	sm.infos[tmpID] = info

	if sm.emitter != nil {
		sm.emitter.Emit("session:opened", map[string]any{
			"session": info,
			"where":   "tab",
		})
	}

	return tmpID, nil
}

func (sm *SessionManager) Write(tmpID string, dataB64 string) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}

	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return err
	}

	err = sess.Write(data)
	if a := sm.automationFor(tmpID); a != nil {
		a.userInput()
	}

	// Broadcast to cluster members if active and single-broadcast is not prevented
	if sm.clusterMgr != nil && !sm.clusterMgr.IsPreventSingleBroadcast() {
		clusterName := sm.clusterMgr.GetCluster(tmpID)
		if clusterName != "" {
			members := sm.clusterMgr.GetMembers(clusterName)
			for _, otherTmpID := range members {
				if otherTmpID != tmpID {
					if otherSess := sm.Get(otherTmpID); otherSess != nil {
						_ = otherSess.Write(data)
					}
				}
			}
		}
	}

	return err
}

func (sm *SessionManager) buildSubstContext(connID string) vars.SubstContext {
	ctx := vars.SubstContext{
		ConnID: connID,
		Vault:  sm.vault,
	}
	if sm.store != nil {
		cfg := sm.store.Config()
		ctx.Globals = cfg.Variables
		for _, conn := range cfg.Connections {
			if conn.ID == connID {
				ctx.UUID = conn.ID
				ctx.Name = conn.Name
				ctx.Title = conn.Title
				ctx.IP = conn.Host
				ctx.Port = conn.Port
				ctx.User = conn.Auth.User
				ctx.Variables = conn.Variables
				break
			}
		}
	}
	if sm.prompter != nil {
		prompter := sm.prompter
		ctx.AskHandler = func(prompt string, options []string, masked bool) (string, bool, error) {
			field := PromptField{Name: "value", Label: "Value", Secret: masked}
			resp, err := ask(context.Background(), prompter, PromptRequest{
				Kind:    "ask",
				Title:   "Variable substitution",
				Message: prompt,
				Fields:  []PromptField{field},
				Options: options,
			})
			if err != nil {
				return "", true, nil
			}
			if len(options) > 0 && resp.Choice != "" {
				return resp.Choice, false, nil
			}
			return resp.Values["value"], false, nil
		}
	}
	return ctx
}

func (sm *SessionManager) RunRemoteMacro(tmpID string, cmd string, appendReturn bool) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}

	info := sm.GetInfo(tmpID)
	var connID string
	if info != nil {
		connID = info.ConnID
	}

	substRes := vars.Subst(cmd, sm.buildSubstContext(connID))
	text := substRes.Text
	if appendReturn {
		text += "\n"
	}

	data := []byte(text)
	err := sess.Write(data)

	// Broadcast if in a cluster
	if sm.clusterMgr != nil && !sm.clusterMgr.IsPreventSingleBroadcast() {
		clusterName := sm.clusterMgr.GetCluster(tmpID)
		if clusterName != "" {
			for _, otherTmpID := range sm.clusterMgr.GetMembers(clusterName) {
				if otherTmpID != tmpID {
					if otherSess := sm.Get(otherTmpID); otherSess != nil {
						_ = otherSess.Write(data)
					}
				}
			}
		}
	}

	return err
}

func (sm *SessionManager) RunLocalMacro(tmpID string, cmd string) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}

	info := sm.GetInfo(tmpID)
	var connID string
	if info != nil {
		connID = info.ConnID
	}

	substRes := vars.Subst(cmd, sm.buildSubstContext(connID))
	cmdStr := substRes.Text

	stream := sess.Stream()
	if stream != nil {
		header := fmt.Sprintf("\r\n\x1b[90m[LOCAL MACRO] %s\x1b[0m\r\n", cmdStr)
		_, _ = stream.Write([]byte(header))
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		c := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
		var out bytes.Buffer
		c.Stdout = &out
		c.Stderr = &out

		err := c.Run()
		if stream != nil && out.Len() > 0 {
			scanner := bufio.NewScanner(&out)
			for scanner.Scan() {
				line := fmt.Sprintf("\x1b[90m%s\x1b[0m\r\n", scanner.Text())
				_, _ = stream.Write([]byte(line))
			}
		}
		if err != nil && stream != nil {
			errLine := fmt.Sprintf("\x1b[31m[LOCAL MACRO ERROR] %v\x1b[0m\r\n", err)
			_, _ = stream.Write([]byte(errLine))
		}
	}()

	return nil
}

func (sm *SessionManager) SaveSessionLog(tmpID string, path string, stripControl bool) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}
	stream := sess.Stream()
	if stream == nil {
		return fmt.Errorf("stream for session %s not found", tmpID)
	}
	return SaveManualLog(path, stream.ring.Bytes(), stripControl)
}

func (sm *SessionManager) Resize(tmpID string, cols, rows int) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}
	return sess.Resize(cols, rows)
}

func (sm *SessionManager) Ack(tmpID string, seq uint64) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return nil
	}
	sess.Ack(seq)
	return nil
}

func (sm *SessionManager) Disconnect(tmpID string) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return fmt.Errorf("session %s not found", tmpID)
	}
	if a := sm.automationFor(tmpID); a != nil {
		a.userDisconnect()
	}
	return sess.Disconnect()
}

func (sm *SessionManager) automationFor(tmpID string) *automation {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.automations[tmpID]
}

func (sm *SessionManager) Close(tmpID string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[tmpID]
	info := sm.infos[tmpID]
	auto := sm.automations[tmpID]
	delete(sm.sessions, tmpID)
	delete(sm.infos, tmpID)
	delete(sm.automations, tmpID)
	sm.mu.Unlock()
	if auto != nil {
		auto.userDisconnect()
	}

	if !ok {
		return nil
	}

	if sm.statsMgr != nil && info != nil && info.ConnID != "" && !strings.HasPrefix(info.ConnID, "quick-") {
		sm.statsMgr.RecordStop(info.ConnID, time.Since(info.StartedAt))
	}

	err := sess.Close()
	if auto != nil {
		auto.close()
	}
	if sm.emitter != nil {
		sm.emitter.Emit("session:closed", map[string]any{"tmpID": tmpID})
	}
	return err
}

func (sm *SessionManager) Attach(tmpID string) (string, uint64, error) {
	sess := sm.Get(tmpID)
	if sess == nil {
		return "", 0, errors.New("session not found")
	}
	replayB64, lastSeq := sess.Stream().Attach()
	return replayB64, lastSeq, nil
}

func (sm *SessionManager) SendTelnetCommand(tmpID string, cmd string) error {
	sess := sm.Get(tmpID)
	if sess == nil {
		return errors.New("session not found")
	}
	telnetSess, ok := sess.(*TelnetSession)
	if !ok {
		return errors.New("not a telnet session")
	}

	var cmdByte byte
	switch strings.ToLower(cmd) {
	case "brk", "break":
		cmdByte = telnet.CmdBRK
	case "ayt":
		cmdByte = telnet.CmdAYT
	case "ip":
		cmdByte = telnet.CmdIP
	case "ao":
		cmdByte = telnet.CmdAO
	case "ec":
		cmdByte = telnet.CmdEC
	case "el":
		cmdByte = telnet.CmdEL
	case "ga":
		cmdByte = telnet.CmdGA
	case "nop":
		cmdByte = telnet.CmdNOP
	default:
		return fmt.Errorf("unknown telnet command: %s", cmd)
	}

	return telnetSess.SendCommand(cmdByte)
}

func (sm *SessionManager) GetStats(connID string) stats.ConnectionStat {
	if sm.statsMgr != nil {
		return sm.statsMgr.GetStats(connID)
	}
	return stats.ConnectionStat{}
}

func (sm *SessionManager) ResetStats(connID string) error {
	if sm.statsMgr != nil {
		return sm.statsMgr.ResetStats(connID)
	}
	return nil
}

func (sm *SessionManager) GetHistory() []stats.HistoryEntry {
	if sm.statsMgr != nil {
		return sm.statsMgr.GetHistory()
	}
	return []stats.HistoryEntry{}
}

func (sm *SessionManager) ClearHistory() error {
	if sm.statsMgr != nil {
		return sm.statsMgr.ClearHistory()
	}
	return nil
}

// StartTee mirrors incoming session output to a local file.
func (sm *SessionManager) StartTee(tmpID string, filePath string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[tmpID]
	sm.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", tmpID)
	}
	return sm.pipeTeeMgr.StartTee(tmpID, sess.Stream(), filePath)
}

// StopTee stops mirroring session output.
func (sm *SessionManager) StopTee(tmpID string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[tmpID]
	sm.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", tmpID)
	}
	return sm.pipeTeeMgr.StopTee(tmpID, sess.Stream())
}

// StartPipe starts piping session output to an external command.
func (sm *SessionManager) StartPipe(tmpID string, command string, prompt string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[tmpID]
	sm.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", tmpID)
	}
	return sm.pipeTeeMgr.StartPipe(context.Background(), tmpID, sess.Stream(), command, prompt)
}

// StopPipe stops the active command pipe for a session.
func (sm *SessionManager) StopPipe(tmpID string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[tmpID]
	sm.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", tmpID)
	}
	return sm.pipeTeeMgr.StopPipe(tmpID, sess.Stream())
}

// ExecuteChain runs a chained connection's Expect sequence on an active terminal session.
func (sm *SessionManager) ExecuteChain(targetTmpID string, chainConnID string, selectedIndices []int) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[targetTmpID]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %s not found", targetTmpID)
	}
	cfg := sm.store.Config()
	sm.mu.Unlock()

	var chainConn *config.Connection
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == chainConnID {
			chainConn = &cfg.Connections[i]
			break
		}
	}
	if chainConn == nil {
		return fmt.Errorf("chained connection %s not found", chainConnID)
	}

	entries := chainConn.Expect
	if len(selectedIndices) > 0 {
		var filtered []config.ExpectEntry
		for _, idx := range selectedIndices {
			if idx >= 0 && idx < len(entries) {
				filtered = append(filtered, entries[idx])
			}
		}
		entries = filtered
	} else if cfg.Terminal.SkipFirstChainExpect && len(entries) > 1 {
		entries = entries[1:]
	}

	if len(entries) == 0 {
		return nil
	}

	substCtx := vars.SubstContext{
		UUID:      chainConn.ID,
		Name:      chainConn.Name,
		Title:     chainConn.Title,
		IP:        chainConn.Host,
		Port:      chainConn.Port,
		User:      chainConn.Auth.User,
		Variables: chainConn.Variables,
		Globals:   cfg.Variables,
		Vault:     sm.vault,
		ConnID:    chainConn.ID,
		KeePass:   sm.kpClient,
	}

	runner := NewExpectRunner(ExpectOptions{
		TmpID:          targetTmpID,
		ConnID:         chainConn.ID,
		Entries:        entries,
		DefaultTimeout: chainConn.Terminal.TimeoutCommand,
		SendDelayMs:    chainConn.SendDelayMs,
		LineEnding:     "\n",
		SubstCtx:       substCtx,
		Vault:          sm.vault,
		SendFunc: func(b []byte) error {
			return sess.Write(b)
		},
		StatusFunc: func(status string) {
			if sm.emitter != nil {
				sm.emitter.Emit("session:status", map[string]any{
					"tmpID": targetTmpID,
					"text":  status,
				})
			}
		},
		ExpectProgFunc: func(index int, total int, state string) {
			if sm.emitter != nil {
				sm.emitter.Emit("session:expect", map[string]any{
					"tmpID": targetTmpID,
					"index": index,
					"total": total,
					"state": state,
				})
			}
		},
		CloseFunc: func(reason string) {
			// Do not terminate parent session on expect fail during chain
		},
	})

	stream := sess.Stream()
	listenerID := stream.AddListener(runner.Feed)

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer stream.RemoveListener(listenerID)

		if sm.emitter != nil {
			sm.emitter.Emit("session:chain:started", map[string]any{
				"targetTmpID": targetTmpID,
				"chainConnID": chainConnID,
				"chainTitle":  chainConn.Title,
			})
		}

		runner.Run(ctx)

		if sm.emitter != nil {
			sm.emitter.Emit("session:chain:finished", map[string]any{
				"targetTmpID": targetTmpID,
				"chainConnID": chainConnID,
			})
		}
	}()

	return nil
}
