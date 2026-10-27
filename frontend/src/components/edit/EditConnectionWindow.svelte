<script lang="ts">
  import { editConnDialog, isReadOnly } from '../../lib/stores/ui';
  import { loadTree, checkUnsaved } from '../../lib/stores/config';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
  import Dialog from '../gtk/Dialog.svelte';
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import Button from '../gtk/Button.svelte';
  import MessageDialog from '../gtk/MessageDialog.svelte';

  import ConnectionTab from './ConnectionTab.svelte';
  import OptionsTab from './OptionsTab.svelte';
  import PrePostExecTab from './PrePostExecTab.svelte';
  import ExpectTab from './ExpectTab.svelte';
  import MacrosTab from './MacrosTab.svelte';
  import VariablesTab from './VariablesTab.svelte';
  import TerminalTab from './TerminalTab.svelte';

  let open = $derived($editConnDialog.open && !$editConnDialog.isQuick);
  let connId = $derived($editConnDialog.connId);

  let conn = $state<any>(null);
  let loading = $state(false);
  let activeOuterTab = $state('connection');
  let validationError = $state<string | null>(null);
  let cmdLineDialog = $state<{ open: boolean; cmd: string }>({ open: false, cmd: '' });

  let isProtected = $derived(
    $isReadOnly || Boolean(conn?.Protected)
  );

  let isSftpOnly = $derived(
    conn?.Method === 'sftp'
  );

  const outerTabs = $derived<TabItem[]>([
    { id: 'connection', label: 'Connection' },
    { id: 'options', label: 'Options' },
    { id: 'pre_exec', label: 'Pre Exec' },
    { id: 'post_exec', label: 'Post Exec' },
    { id: 'expect', label: 'Expect' },
    { id: 'remote_macros', label: 'Remote Macros' },
    { id: 'local_macros', label: 'Local Macros' },
    { id: 'variables', label: 'User Variables' },
    { id: 'terminal', label: 'Terminal Options' },
  ]);

  $effect(() => {
    if (open && connId) {
      loadConnection(connId);
    }
  });

  async function loadConnection(id: string) {
    loading = true;
    conn = null;
    validationError = null;
    try {
      const data = await ConfigService.GetConnectionForEdit(id);
      conn = JSON.parse(JSON.stringify(data));
      ensureDefaults();
    } catch (err) {
      console.error('Failed to load connection for edit:', err);
      conn = createBlankConnection(id);
    } finally {
      loading = false;
    }
  }

  function ensureDefaults() {
    if (!conn) return;
    conn.ID = conn.ID || conn.id || '';
    conn.Name = conn.Name || conn.name || 'New Connection';
    conn.Title = conn.Title || conn.title || '';
    conn.Method = conn.Method || conn.method || 'ssh';
    conn.Host = conn.Host || conn.host || '';
    conn.Port = conn.Port || conn.port || (conn.Method === 'telnet' ? 23 : 22);
    conn.LaunchAtStartup = Boolean(conn.LaunchAtStartup ?? conn.launch_at_startup);
    conn.AutoReconnect = Boolean(conn.AutoReconnect ?? conn.auto_reconnect);
    conn.SendDelayMs = conn.SendDelayMs ?? conn.send_delay_ms ?? 0;
    conn.Protected = Boolean(conn.Protected ?? conn.protected);
    conn.SFTPBrowser = conn.SFTPBrowser || conn.sftp_browser || 'global';
    conn.SFTPStartDir = conn.SFTPStartDir || conn.sftp_start_dir || '';

    const auth = conn.Auth || conn.auth || {};
    conn.Auth = {
      Type: auth.Type || auth.type || 'password',
      User: auth.User || auth.user || '',
      KeyFile: auth.KeyFile || auth.key_file || '',
      KeyUser: auth.KeyUser || auth.key_user || '',
      Exclusive: Boolean(auth.Exclusive ?? auth.exclusive),
    };

    const net = conn.Network || conn.network || {};
    const socks = net.Socks || net.socks || {};
    const jump = net.Jump || net.jump || {};
    conn.Network = {
      Mode: net.Mode || net.mode || 'global',
      Socks: {
        Host: socks.Host || socks.host || '',
        Port: socks.Port || socks.port || 1080,
        User: socks.User || socks.user || '',
      },
      Jump: {
        Host: jump.Host || jump.host || '',
        Port: jump.Port || jump.port || 22,
        User: jump.User || jump.user || '',
        KeyFile: jump.KeyFile || jump.key_file || '',
      },
      PseudoJump: Boolean(net.PseudoJump ?? net.pseudo_jump),
    };

    const ssh = conn.SSH || conn.ssh || {};
    conn.SSH = {
      IPVersion: ssh.IPVersion || ssh.ip_version || 'any',
      ForwardX11: Boolean(ssh.ForwardX11 ?? ssh.forward_x11),
      GatewayPorts: Boolean(ssh.GatewayPorts ?? ssh.gateway_ports),
      ForwardAgent: Boolean(ssh.ForwardAgent ?? ssh.forward_agent),
      NoRemoteCommand: Boolean(ssh.NoRemoteCommand ?? ssh.no_remote_command),
      RandomSOCKSTunnel: Boolean(ssh.RandomSOCKSTunnel ?? ssh.random_socks_tunnel),
      Options: ssh.Options || ssh.options || {},
      LocalForwards: (ssh.LocalForwards || ssh.local_forwards || []).map((f: any) => ({
        BindAddress: f.BindAddress || f.bind_address || '127.0.0.1',
        Port: f.Port || f.port || 8080,
        Host: f.Host || f.host || '127.0.0.1',
        HostPort: f.HostPort || f.host_port || 80,
      })),
      RemoteForwards: (ssh.RemoteForwards || ssh.remote_forwards || []).map((f: any) => ({
        BindAddress: f.BindAddress || f.bind_address || '127.0.0.1',
        Port: f.Port || f.port || 9090,
        Host: f.Host || f.host || '127.0.0.1',
        HostPort: f.HostPort || f.host_port || 80,
      })),
      DynamicForwards: (ssh.DynamicForwards || ssh.dynamic_forwards || []).map((f: any) => ({
        BindAddress: f.BindAddress || f.bind_address || '127.0.0.1',
        Port: f.Port || f.port || 1080,
      })),
    };

    const tel = conn.Telnet || conn.telnet || {};
    conn.Telnet = {
      IPVersion: tel.IPVersion || tel.ip_version || 'any',
      BindAddress: tel.BindAddress || tel.bind_address || '',
      EscapeChar: tel.EscapeChar || tel.escape_char || '^]',
      NAWS: Boolean(tel.NAWS ?? tel.naws ?? true),
      TerminalType: tel.TerminalType || tel.terminal_type || 'XTERM-256COLOR',
      LineEnding: tel.LineEnding || tel.line_ending || 'crlf',
    };

    const ss = conn.SendString || conn.send_string || {};
    conn.SendString = {
      Enabled: Boolean(ss.Enabled ?? ss.enabled),
      Text: ss.Text || ss.text || '',
      AppendReturn: Boolean(ss.AppendReturn ?? ss.append_return ?? true),
      Interval: ss.Interval ?? ss.interval ?? 60,
      OnlyWhenIdle: Boolean(ss.OnlyWhenIdle ?? ss.only_when_idle),
    };

    const sl = conn.SessionLogs || conn.session_logs || {};
    conn.SessionLogs = {
      Enabled: Boolean(sl.Enabled ?? sl.enabled),
      Folder: sl.Folder || sl.folder || '',
      Pattern: sl.Pattern || sl.pattern || '',
      Keep: sl.Keep ?? sl.keep ?? 0,
      StripControlChars: Boolean(sl.StripControlChars ?? sl.strip_control_chars ?? true),
      Timestamps: Boolean(sl.Timestamps ?? sl.timestamps ?? true),
    };

    conn.PreExec = (conn.PreExec || conn.pre_exec || []).map((p: any) => ({
      Command: p.Command || p.command || '',
      Ask: Boolean(p.Ask ?? p.ask),
      Default: Boolean(p.Default ?? p.default ?? true),
    }));

    conn.PostExec = (conn.PostExec || conn.post_exec || []).map((p: any) => ({
      Command: p.Command || p.command || '',
      Ask: Boolean(p.Ask ?? p.ask),
      Default: Boolean(p.Default ?? p.default ?? true),
    }));

    conn.Expect = (conn.Expect || conn.expect || []).map((e: any) => ({
      ID: e.ID || e.id || '',
      Enabled: Boolean(e.Enabled ?? e.enabled ?? true),
      Pattern: e.Pattern || e.pattern || '',
      Send: e.Send || e.send || '',
      Hidden: Boolean(e.Hidden ?? e.hidden),
      AppendReturn: Boolean(e.AppendReturn ?? e.append_return ?? true),
      OnMatch: e.OnMatch || e.on_match || 'next',
      OnMatchGoto: e.OnMatchGoto ?? e.on_match_goto ?? 1,
      OnFail: e.OnFail || e.on_fail || 'stop',
      OnFailGoto: e.OnFailGoto ?? e.on_fail_goto ?? 1,
      Timeout: e.Timeout ?? e.timeout ?? 10,
    }));

    const mac = conn.Macros || conn.macros || {};
    conn.Macros = {
      Local: (mac.Local || mac.local || []).map((m: any) => ({
        Command: m.Command || m.command || '',
        Description: m.Description || m.description || '',
        Confirm: Boolean(m.Confirm ?? m.confirm),
        AppendReturn: Boolean(m.AppendReturn ?? m.append_return ?? true),
      })),
      Remote: (mac.Remote || mac.remote || []).map((m: any) => ({
        Command: m.Command || m.command || '',
        Description: m.Description || m.description || '',
        Confirm: Boolean(m.Confirm ?? m.confirm),
        AppendReturn: Boolean(m.AppendReturn ?? m.append_return ?? true),
      })),
    };

    conn.Variables = (conn.Variables || conn.variables || []).map((v: any, idx: number) => ({
      ID: v.ID || v.id || `v${idx}`,
      Value: v.Value || v.value || '',
      Hidden: Boolean(v.Hidden ?? v.hidden),
    }));

    const trm = conn.Terminal || conn.terminal || {};
    const trmPrompts = trm.Prompts || trm.prompts || {};
    conn.Terminal = {
      UsePersonal: Boolean(trm.UsePersonal ?? trm.use_personal),
      OpenIn: trm.OpenIn || trm.open_in || 'tab',
      WindowWidth: trm.WindowWidth ?? trm.window_width ?? 800,
      WindowHeight: trm.WindowHeight ?? trm.window_height ?? 600,
      TimeoutConnect: trm.TimeoutConnect ?? trm.timeout_connect ?? 0,
      TimeoutCommand: trm.TimeoutCommand ?? trm.timeout_command ?? 10,
      TabColor: trm.TabColor || trm.tab_color || '',
      Prompts: {
        Command: trmPrompts.Command || trmPrompts.command || '',
        User: trmPrompts.User || trmPrompts.user || '',
        Password: trmPrompts.Password || trmPrompts.password || '',
        HostKeyVerification: trmPrompts.HostKeyVerification || trmPrompts.host_key_verification || '',
        RemoteHostKeyChanged: trmPrompts.RemoteHostKeyChanged || trmPrompts.remote_host_key_changed || '',
        PressAnyKey: trmPrompts.PressAnyKey || trmPrompts.press_any_key || '',
      },
      Look: trm.Look || trm.look || null,
    };

    conn.Password = conn.Password || conn.password || '';
    conn.KeyPassphrase = conn.KeyPassphrase || conn.keyPassphrase || '';
    conn.SocksPassword = conn.SocksPassword || conn.socksPassword || '';
    conn.JumpPassword = conn.JumpPassword || conn.jumpPassword || '';
  }

  function createBlankConnection(id: string) {
    return {
      ID: id,
      Name: 'New Connection',
      Title: 'New Connection',
      Method: 'ssh',
      Host: '',
      Port: 22,
      LaunchAtStartup: false,
      AutoReconnect: false,
      SendDelayMs: 0,
      Protected: false,
      SFTPBrowser: 'global',
      SFTPStartDir: '',
      Auth: { Type: 'password', User: '', KeyFile: '', KeyUser: '', Exclusive: false },
      Network: { Mode: 'global', Socks: { Host: '', Port: 1080, User: '' }, Jump: { Host: '', Port: 22, User: '', KeyFile: '' }, PseudoJump: false },
      SSH: { IPVersion: 'any', ForwardX11: false, GatewayPorts: false, ForwardAgent: false, NoRemoteCommand: false, RandomSOCKSTunnel: false, Options: {}, LocalForwards: [], RemoteForwards: [], DynamicForwards: [] },
      Telnet: { IPVersion: 'any', BindAddress: '', EscapeChar: '^]', NAWS: true, TerminalType: 'XTERM-256COLOR', LineEnding: 'crlf' },
      SendString: { Enabled: false, Text: '', AppendReturn: true, Interval: 60, OnlyWhenIdle: false },
      SessionLogs: { Enabled: false, Folder: '', Pattern: '', Keep: 0, StripControlChars: true, Timestamps: true },
      PreExec: [],
      PostExec: [],
      Expect: [],
      Macros: { Local: [], Remote: [] },
      Variables: [],
      Terminal: { UsePersonal: false, OpenIn: 'tab', WindowWidth: 800, WindowHeight: 600, TimeoutConnect: 0, TimeoutCommand: 10, TabColor: '', Prompts: {}, Look: null },
      Password: '',
      KeyPassphrase: '',
      SocksPassword: '',
      JumpPassword: '',
    };
  }

  function handleClose() {
    editConnDialog.set({ open: false, connId: '', isQuick: false });
  }

  function validate(): string | null {
    const host = (conn.Host || '').trim();
    if (!host) {
      return 'Missing IP/Hostname: Host cannot be empty.';
    }
    const port = conn.Port;
    if (!port || port < 1 || port > 65535) {
      return 'Invalid Port: Port must be between 1 and 65535.';
    }
    const authType = conn.Auth?.Type || 'password';
    const user = (conn.Auth?.User || '').trim();
    if (authType === 'password' && !user) {
      return 'Missing Username: User is required for User / Password authentication.';
    }

    // SSH port forwardings duplicates check
    if (conn.SSH) {
      const local = conn.SSH.LocalForwards || [];
      const localSeen = new Set<string>();
      for (const f of local) {
        const key = `${f.BindAddress || '127.0.0.1'}:${f.Port}`;
        if (localSeen.has(key)) {
          return `CONFIG ERROR: Local Port ${f.Port} on Local IP '${f.BindAddress || '127.0.0.1'}' defined in Local Port Forwarding is already in use!`;
        }
        localSeen.add(key);
      }

      const remote = conn.SSH.RemoteForwards || [];
      const remoteSeen = new Set<number>();
      for (const f of remote) {
        if (remoteSeen.has(f.Port)) {
          return `WARNING: Same Remote Port ${f.Port} defined more than once!`;
        }
        remoteSeen.add(f.Port);
      }
    }

    return null;
  }

  async function handleSave() {
    if (isProtected) return;
    const err = validate();
    if (err) {
      validationError = err;
      return;
    }
    validationError = null;

    try {
      await ConfigService.SaveConnection(conn.ID, conn);
      await loadTree();
      await checkUnsaved();
      handleClose();
    } catch (e: any) {
      validationError = `Failed to save connection: ${e?.message || e}`;
    }
  }

  function showCommandLine() {
    const method = conn.Method || 'ssh';
    const host = conn.Host || 'localhost';
    const port = conn.Port || 22;
    const user = conn.Auth?.User || '';

    let cmd = '';
    if (method === 'ssh') {
      const ipVer = conn.SSH?.IPVersion || 'any';
      const ipFlag = ipVer === '4' ? '-4 ' : ipVer === '6' ? '-6 ' : '';
      const userPart = user ? `${user}@` : '';
      cmd = `ssh ${ipFlag}-p ${port} ${userPart}${host}`;
    } else if (method === 'telnet') {
      const t = conn.Telnet || {};
      const ipFlag = t.IPVersion === '4' ? '-4 ' : t.IPVersion === '6' ? '-6 ' : '';
      const bindFlag = t.BindAddress ? `-b ${t.BindAddress} ` : '';
      const escFlag = t.EscapeChar ? `-e '${t.EscapeChar}' ` : '';
      cmd = `telnet ${ipFlag}${bindFlag}${escFlag}${host} ${port}`;
    } else if (method === 'sftp') {
      cmd = `sftp -P ${port} ${user ? `${user}@` : ''}${host}`;
    }
    cmdLineDialog = { open: true, cmd };
  }
</script>

<!-- Shown only once the data is loaded: no "Loading..." frame, no stale data flash. -->
{#if open && conn && !loading}
  <Dialog
    title="Edit Connection"
    bannerText="Edit Connection"
    width={760}
    height={620}
    {open}
    onclose={handleClose}
  >
    {#snippet children()}
      <div class="edit-window-content">
        {#if validationError}
          <div class="validation-error-banner" role="alert">
            <span class="error-icon">⚠️</span>
            <span class="error-text">{validationError}</span>
            <button
              type="button"
              class="error-dismiss"
              onclick={() => (validationError = null)}
            >×</button>
          </div>
        {/if}

        {#if loading || !conn}
          <div class="loading-state">Loading connection data...</div>
        {:else}
          <div class="notebook-wrapper">
            <Notebook
              tabs={outerTabs}
              bind:activeTab={activeOuterTab}
              position="top"
            >
              {#snippet children(tabId: string)}
                <div class="tab-page-content">
                  {#if tabId === 'connection'}
                    <ConnectionTab
                      bind:conn={conn}
                      disabled={isProtected}
                      onShowCmdLine={showCommandLine}
                    />
                  {:else if tabId === 'options'}
                    <OptionsTab
                      bind:conn={conn}
                      disabled={isProtected}
                    />
                  {:else if tabId === 'pre_exec'}
                    <PrePostExecTab
                      title="Pre-connections execute"
                      tooltip="Execute commands LOCALLY BEFORE the connection is launched"
                      bind:items={conn.PreExec}
                      disabled={isProtected}
                    />
                  {:else if tabId === 'post_exec'}
                    <PrePostExecTab
                      title="Post-connections execute"
                      tooltip="Execute commands LOCALLY AFTER the connection has closed"
                      bind:items={conn.PostExec}
                      disabled={isProtected}
                    />
                  {:else if tabId === 'expect'}
                    {#if isSftpOnly}
                      <div class="disabled-hint">Expect rules are not available for SFTP browser-only connections.</div>
                    {:else}
                      <ExpectTab
                        bind:items={conn.Expect}
                        disabled={isProtected}
                      />
                    {/if}
                  {:else if tabId === 'remote_macros'}
                    {#if isSftpOnly}
                      <div class="disabled-hint">Remote Macros are not available for SFTP browser-only connections.</div>
                    {:else}
                      <MacrosTab
                        title="Remote commands list"
                        isRemote={true}
                        bind:items={conn.Macros.Remote}
                        disabled={isProtected}
                      />
                    {/if}
                  {:else if tabId === 'local_macros'}
                    {#if isSftpOnly}
                      <div class="disabled-hint">Local Macros are not available for SFTP browser-only connections.</div>
                    {:else}
                      <MacrosTab
                        title="Local commands list"
                        isRemote={false}
                        bind:items={conn.Macros.Local}
                        disabled={isProtected}
                      />
                    {/if}
                  {:else if tabId === 'variables'}
                    <VariablesTab
                      bind:items={conn.Variables}
                      disabled={isProtected}
                    />
                  {:else if tabId === 'terminal'}
                    {#if isSftpOnly}
                      <div class="disabled-hint">Terminal options are not available for SFTP browser-only connections.</div>
                    {:else}
                      <TerminalTab
                        bind:conn={conn}
                        disabled={isProtected}
                      />
                    {/if}
                  {/if}
                </div>
              {/snippet}
            </Notebook>
          </div>
        {/if}
      </div>
    {/snippet}

    {#snippet actions()}
      <div class="edit-footer">
        <div class="footer-left">
          {#if isProtected}
            <span class="lbl-protected-edit">🔒 This node is protected against modification</span>
          {/if}
        </div>
        <div class="footer-right">
          <Button
            label="_Close without saving"
            icon="gtk-cancel"
            onclick={handleClose}
          />
          <Button
            label="_Save and Close"
            icon="gtk-save"
            isDefault={true}
            disabled={isProtected || loading}
            tooltip="Save current config"
            onclick={handleSave}
          />
        </div>
      </div>
    {/snippet}
  </Dialog>
{/if}

{#if cmdLineDialog.open}
  <MessageDialog
    title="Equivalent Command Line"
    primaryText="Equivalent terminal command:"
    secondaryText={cmdLineDialog.cmd}
    type="info"
    buttons="ok"
    onresponse={() => (cmdLineDialog.open = false)}
  />
{/if}

<style>
  .edit-window-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 480px;
    position: relative;
    overflow: hidden;
  }

  .validation-error-banner {
    background-color: var(--gtk-error-bg);
    border: 1px solid var(--gtk-error-border);
    color: var(--gtk-error-fg);
    padding: 6px 12px;
    border-radius: 3px;
    margin-bottom: 6px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 0.9em;
  }

  .error-icon {
    font-size: 1.1em;
  }

  .error-text {
    flex: 1;
  }

  .error-dismiss {
    background: transparent;
    border: none;
    font-size: 1.2em;
    cursor: pointer;
    color: var(--gtk-error-fg);
    padding: 0 4px;
  }

  .loading-state {
    padding: 32px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
  }

  .notebook-wrapper {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .tab-page-content {
    height: 100%;
    overflow-y: auto;
  }

  .disabled-hint {
    padding: 32px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
  }

  .edit-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }

  .lbl-protected-edit {
    color: var(--gtk-error-fg);
    font-size: 0.85em;
    font-weight: 500;
  }

  .footer-right {
    display: flex;
    gap: 8px;
  }
</style>
