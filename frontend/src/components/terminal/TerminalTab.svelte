<script lang="ts">
  import { settings } from '../../lib/stores/config';
  import {
    sessions,
    sessionStatus,
    disconnectSession,
    closeSession,
    setActiveTab,
  } from '../../lib/stores/sessions';
  import { clustersVisible, sftpVisible } from '../../lib/stores/ui';
  import TerminalView from './TerminalView.svelte';
  import MacrosBar from './MacrosBar.svelte';
  import SftpPanel from '../sftp/SftpPanel.svelte';
  import ChainSelectionDialog from '../dialogs/ChainSelectionDialog.svelte';
  import PipedOutputWindow from '../dialogs/PipedOutputWindow.svelte';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    tmpID: string;
    /** Whether this tab is the visible one (keyboard shortcuts, focus, new-data colour). */
    active?: boolean;
  }

  let { tmpID, active = true }: Props = $props();

  let sessionInfo = $derived($sessions.find((s) => s.tmpID === tmpID));
  let isConnected = $derived(sessionInfo?.state === 'connected');
  let isSFTPOnly = $derived(sessionInfo?.kind === 'sftp');
  let hasSFTP = $derived(sessionInfo?.hasSFTP ?? false);

  // Connection data (personal terminal look, per-connection SFTP setting).
  let conn = $state<any>(null);
  $effect(() => {
    const id = sessionInfo?.connID;
    if (!id || id === 'local-shell' || id.startsWith('quick-')) {
      conn = null;
      return;
    }
    ConfigService.GetConnection(id)
      .then((c: any) => (conn = c))
      .catch(() => (conn = null));
  });

  let personalLook = $derived(
    sessionInfo?.kind === 'local'
      ? (($settings?.localShell as any)?.Look ?? null)
      : conn?.Terminal?.UsePersonal
        ? (conn?.Terminal?.Look ?? null)
        : null
  );

  let sftpOpen = $state(false);
  let autoOpened = false;

  // Open the SFTP panel once after connecting, honouring the per-connection
  // setting (global | open | closed) and sftp_browser.auto_open.
  $effect(() => {
    if (!isConnected || !hasSFTP || isSFTPOnly || autoOpened) return;
    autoOpened = true;
    const perConn = conn?.SFTPBrowser || 'global';
    const auto = perConn === 'open' || (perConn === 'global' && ($settings?.sftpBrowser?.AutoOpen ?? true));
    if (auto && $sftpVisible) sftpOpen = true;
  });

  // The SFTP toggle in the button bar shows/hides the panels of all terminals.
  let lastSftpVisible = $sftpVisible;
  $effect(() => {
    const visible = $sftpVisible;
    if (visible === lastSftpVisible) return;
    lastSftpVisible = visible;
    if (hasSFTP && !isSFTPOnly) sftpOpen = visible;
  });

  let sftpSide = $derived<'left' | 'right'>($settings?.sftpBrowser?.Position === 'right' ? 'right' : 'left');

  function toggleSFTP() {
    sftpOpen = !sftpOpen;
  }

  let showStatusBar = $derived($settings?.terminal?.ShowStatusBar ?? true);
  let macrosMode = $derived(($settings?.terminal?.MacrosBar as 'never' | 'combo' | 'buttons') || 'never');

  let statusHistory = $derived($sessionStatus[tmpID] ?? []);
  let statusText = $derived.by(() => {
    const last = statusHistory[statusHistory.length - 1];
    const base = last || (sessionInfo?.state ?? 'unknown').toUpperCase();
    return sessionInfo?.cluster ? `[IN CLUSTER: ${sessionInfo.cluster}] - Status: ${base}` : `Status: ${base}`;
  });

  // Chains (drop a connection from the tree onto the terminal)
  let isDragOver = $state(false);
  let chainDialogOpen = $state(false);
  let chainConn = $state<any>(null);
  let pipeWindowOpen = $state(false);

  function handleDragOver(e: DragEvent) {
    if (e.dataTransfer?.types.includes('application/x-leanterm-conn-id')) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
      isDragOver = true;
    }
  }

  async function handleDropConnection(e: DragEvent) {
    isDragOver = false;
    const connId = e.dataTransfer?.getData('application/x-leanterm-conn-id');
    if (!connId) return;
    e.preventDefault();
    try {
      const c = await ConfigService.GetConnection(connId);
      if (!c) return;
      if ($settings?.terminal?.ConfirmChains ?? true) {
        chainConn = c;
        chainDialogOpen = true;
      } else {
        await SessionService.ExecuteChain(tmpID, connId, []);
      }
    } catch (err) {
      console.error('Failed to prepare chain execution:', err);
    }
  }

  async function executeChain(selectedIndices: number[]) {
    if (!chainConn) return;
    try {
      await SessionService.ExecuteChain(tmpID, chainConn.ID, selectedIndices);
    } catch (err) {
      console.error('Failed to execute chain:', err);
    }
  }
</script>

<div class="terminal-tab-container">
  {#if isSFTPOnly}
    <!-- SFTP-only connection: full width browser -->
    <div class="terminal-tab-body sftp-only-mode">
      <SftpPanel {tmpID} fullWidth={true} />
    </div>
  {:else}
    <!-- SSH / local session: SFTP side panel (left or right) + terminal -->
    <div class="terminal-tab-body" class:sftp-right={sftpSide === 'right'}>
      {#if hasSFTP && sftpOpen}
        <SftpPanel {tmpID} side={sftpSide} onclose={toggleSFTP} />
      {/if}

      <div
        class="terminal-view-area"
        class:chain-drag-over={isDragOver}
        ondragover={handleDragOver}
        ondragleave={() => (isDragOver = false)}
        ondrop={handleDropConnection}
        role="region"
        aria-label="Terminal"
      >
        {#if macrosMode !== 'never'}
          <MacrosBar {tmpID} connID={sessionInfo?.connID} mode={macrosMode} />
        {/if}
        <div class="terminal-view-slot">
          <TerminalView
            {tmpID}
            {active}
            {personalLook}
            kind={sessionInfo?.kind || 'local'}
            {hasSFTP}
            {sftpOpen}
            ontoggleSftp={toggleSFTP}
            ondisconnect={() => disconnectSession(tmpID)}
            onclose={() => closeSession(tmpID)}
          />
        </div>
      </div>
    </div>
  {/if}

  {#if showStatusBar}
    <div class="terminal-status-bar" role="status">
      <div class="status-left">
        <img
          src={getIcon(isConnected ? 'asbru-terminal-ok-small' : 'asbru-terminal-ko-small')}
          alt={isConnected ? 'Connected' : 'Disconnected'}
          title={isConnected ? 'Connected' : 'Disconnected'}
          width="16"
          height="16"
        />
        {#if $clustersVisible || sessionInfo?.cluster}
          <img
            src={getIcon(sessionInfo?.cluster ? 'asbru-cluster-manager' : 'asbru-cluster-manager-off')}
            alt=""
            title={sessionInfo?.cluster ? `In CLUSTER: ${sessionInfo.cluster}` : 'Unclustered'}
            width="16"
            height="16"
          />
        {/if}
        {#if sessionInfo?.state === 'connecting'}
          <span class="status-progress" aria-hidden="true"></span>
        {/if}
        <span class="status-text" title={statusHistory.join('\n')}>{statusText}</span>
      </div>

      <div class="status-right">
        {#if hasSFTP && !isSFTPOnly}
          <button
            type="button"
            class="status-btn"
            class:is-active={sftpOpen}
            title="Show/Hide SFTP browser (Alt+Ctrl+S)"
            onclick={toggleSFTP}
          >
            <img src={getIcon('asbru-method-sftp')} alt="SFTP" width="16" height="16" />
          </button>
        {/if}
        <button type="button" class="status-btn" title="Show Info Tab" onclick={() => setActiveTab('info')}>
          <img src={getIcon('gtk-info')} alt="Info" width="16" height="16" />
        </button>
      </div>
    </div>
  {/if}
</div>

{#if chainDialogOpen}
  <ChainSelectionDialog
    open={chainDialogOpen}
    targetTitle={sessionInfo?.title || 'Terminal'}
    targetTmpID={tmpID}
    {chainConn}
    skipFirstExpect={$settings?.terminal?.SkipFirstChainExpect ?? true}
    onexecute={executeChain}
    onclose={() => (chainDialogOpen = false)}
  />
{/if}

{#if pipeWindowOpen}
  <PipedOutputWindow bind:open={pipeWindowOpen} {tmpID} onclose={() => (pipeWindowOpen = false)} />
{/if}

<style>
  .terminal-tab-container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--gtk-terminal-bg);
  }

  .terminal-tab-body {
    flex: 1;
    display: flex;
    min-height: 0;
    overflow: hidden;
    position: relative;
  }

  /* Reversed order keeps the terminal mounted when the side changes. */
  .terminal-tab-body.sftp-right {
    flex-direction: row-reverse;
  }

  .sftp-only-mode {
    width: 100%;
  }

  .terminal-view-area {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    position: relative;
  }

  .terminal-view-slot {
    flex: 1;
    min-height: 0;
    position: relative;
  }

  .terminal-view-area.chain-drag-over {
    outline: 2px dashed var(--gtk-accent);
    outline-offset: -2px;
  }

  .terminal-status-bar {
    height: 24px;
    background: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 6px;
    font-size: 11px;
    color: var(--gtk-fg-color);
    user-select: none;
    flex-shrink: 0;
  }

  .status-left,
  .status-right {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .status-text {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .status-progress {
    width: 40px;
    height: 6px;
    border-radius: 3px;
    background: linear-gradient(90deg, transparent, var(--gtk-accent), transparent);
    background-size: 200% 100%;
    animation: pulse 1s linear infinite;
    flex-shrink: 0;
  }

  @keyframes pulse {
    from {
      background-position: 100% 0;
    }
    to {
      background-position: -100% 0;
    }
  }

  .status-btn {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 1px 3px;
    margin: 0;
  }

  .status-btn:hover {
    background: var(--gtk-btn-hover-bg);
    border-color: var(--gtk-border-color);
  }

  .status-btn.is-active {
    background: var(--gtk-btn-active-bg);
    border-color: var(--gtk-accent);
  }
</style>
