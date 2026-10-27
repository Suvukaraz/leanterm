<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import {
    sftpViews,
    openSftp,
    loadDir,
    goHome,
    navigateBack,
    navigateForward,
    navigateUp,
    refresh,
    toggleShowHidden,
    toggleFollowTerminal,
    setSelectedPaths,
    createFolder,
    createEmptyFile,
    removeItems,
    renameItem,
    chmodItems,
    downloadFiles,
    uploadFiles,
    cancelTransfer,
    dismissTransfer,
    openWithDefaultApp,
    uploadBack,
    goToTerminalFolder,
    findConflicts,
    copyOrMoveLocal,
  } from '../../lib/stores/sftp';
  import { fsBackend, type BrowserKind } from '../../lib/stores/fsBackend';
  import { settings } from '../../lib/stores/config';
  import { sftpPanelWidth } from '../../lib/stores/ui';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import { sessions, sendSessionInput } from '../../lib/stores/sessions';
  import SftpToolbar from './SftpToolbar.svelte';
  import SftpPathBar from './SftpPathBar.svelte';
  import SftpFileList from './SftpFileList.svelte';
  import SftpTransferQueue from './SftpTransferQueue.svelte';
  import SftpPermissionsDialog from './SftpPermissionsDialog.svelte';
  import Dialog from '../gtk/Dialog.svelte';
  import MessageDialog from '../gtk/MessageDialog.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    tmpID: string;
    /** SFTP-only connections show the browser over the whole tab. */
    fullWidth?: boolean;
    /** Side of the terminal the panel is shown on (the resize handle faces the terminal). */
    side?: 'left' | 'right';
    /** 'local' browses the local file system next to a local shell. */
    kind?: BrowserKind;
    onclose?: () => void;
  }

  let { tmpID, fullWidth = false, side = 'left', kind = 'sftp', onclose }: Props = $props();
  let isLocal = $derived(kind === 'local');

  let view = $derived($sftpViews[tmpID]);
  let sessionInfo = $derived($sessions.find((s) => s.tmpID === tmpID));
  let isConnected = $derived(sessionInfo?.state === 'connected');

  // Shared width of all SFTP panels: last dragged width, else sftp_browser.width.
  let panelWidth = $derived($sftpPanelWidth > 0 ? $sftpPanelWidth : ($settings?.sftpBrowser?.Width || 340));
  let dragWidth = $state<number | null>(null);
  let shownWidth = $derived(Math.round(dragWidth ?? panelWidth));
  let isResizing = $state(false);


  // Prompt Dialogs state
  let newFolderOpen = $state(false);
  let newFolderName = $state('');

  let newFileOpen = $state(false);
  let newFileName = $state('');

  let renameOpen = $state(false);
  let renameFromPath = $state('');
  let renameNewName = $state('');

  let deleteConfirmOpen = $state(false);
  let deletePaths = $state<string[]>([]);

  let permissionsOpen = $state(false);
  let permissionsPaths = $state<string[]>([]);
  let permissionsInitialMode = $state(0o755);

  let osc7Unsubscribe: (() => void) | null = null;
  /** Last folder the terminal reported (OSC 7 or /proc/<pid>/cwd). */
  let lastTerminalCwd = '';
  let dropUnsubscribe: (() => void) | null = null;
  let fileModifiedUnsubscribe: (() => void) | null = null;
  let pollInterval: any = null;

  // File modification dialog state
  let uploadBackDialogOpen = $state(false);
  let modifiedRemotePath = $state('');
  let modifiedLocalPath = $state('');

  onMount(() => {
    // If not already opened, initialize the browser; a panel shown again resumes
    // following the terminal folder if that was on.
    if (!view || !view.sftpID) {
      openSftp(tmpID, kind);
    } else if (view.followTerminal) {
      fsBackend(view.kind).setFollow(view.sftpID, true).catch(() => {});
    }

    const offDrop = Events.On('files:dropped', handleNativeDrop);
    dropUnsubscribe = offDrop;

    // Subscribe to OSC 7 cwd events
    try {
      osc7Unsubscribe = Events.On(`session:cwd:${tmpID}`, (event: any) => {
        const cwd = typeof event?.data === 'string' ? event.data : String(event || '');
        if (cwd) lastTerminalCwd = cwd;
        if (cwd && view?.followTerminal && cwd !== view?.currentPath) {
          loadDir(tmpID, cwd);
        }
      });
    } catch (err) {
      console.warn('Could not register OSC 7 event listener:', err);
    }

    // Subscribe to external file modifications
    try {
      fileModifiedUnsubscribe = Events.On('sftp:file:modified', (event: any) => {
        const data = event?.data || event;
        if (data && data.sftpID === view?.sftpID) {
          modifiedRemotePath = data.remotePath;
          modifiedLocalPath = data.localPath;
          uploadBackDialogOpen = true;
        }
      });
    } catch (err) {
      console.warn('Could not register sftp:file:modified listener:', err);
    }

    // Optional polling
    const pollSec = ($settings as any)?.sftpBrowser?.PollSeconds ?? 0;
    if (pollSec > 0) {
      pollInterval = setInterval(() => {
        if (isConnected && view?.sftpID) {
          refresh(tmpID);
        }
      }, pollSec * 1000);
    }
  });

  onDestroy(() => {
    dropUnsubscribe?.();
    if (view?.sftpID) fsBackend(view.kind).close(view.sftpID).catch(() => {});
    if (osc7Unsubscribe) {
      osc7Unsubscribe();
    }
    if (fileModifiedUnsubscribe) {
      fileModifiedUnsubscribe();
    }
    if (pollInterval) {
      clearInterval(pollInterval);
    }
  });

  // Resizing via the handle on the border towards the terminal (pointer capture, rAF-throttled).
  let resizeStartX = 0;
  let resizeStartW = 0;
  let resizeFrame = 0;
  let resizePendingX: number | null = null;

  function startResize(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    isResizing = true;
    resizeStartX = e.clientX;
    resizeStartW = shownWidth;
    document.body.classList.add('paned-dragging-col');
  }

  function moveResize(e: PointerEvent) {
    if (!isResizing) return;
    resizePendingX = e.clientX;
    if (!resizeFrame) {
      resizeFrame = requestAnimationFrame(() => {
        resizeFrame = 0;
        if (resizePendingX === null) return;
        const maxW = Math.max(300, window.innerWidth * 0.7);
        const delta = side === 'right' ? resizeStartX - resizePendingX : resizePendingX - resizeStartX;
        dragWidth = Math.min(maxW, Math.max(220, resizeStartW + delta));
      });
    }
  }

  function endResize(e: PointerEvent) {
    if (!isResizing) return;
    isResizing = false;
    (e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId);
    document.body.classList.remove('paned-dragging-col');
    if (dragWidth !== null) {
      sftpPanelWidth.set(Math.round(dragWidth));
      dragWidth = null;
    }
  }

  // Open a file in its default desktop application. Remote files are downloaded to
  // a temp folder (saving there offers to upload the changes back); local files are
  // opened directly.
  let openingFile = $state('');
  let openError = $state('');
  async function handleOpenFile(p: string) {
    if (openingFile) return;
    openingFile = p.split('/').pop() || p;
    try {
      await openWithDefaultApp(tmpID, p);
    } catch (err: any) {
      openError = `Could not open '${openingFile}':\n${err?.message || err}`;
    } finally {
      openingFile = '';
    }
  }

  // Footer statistics
  function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  let totalItems = $derived(view?.entries?.length || 0);
  let selectedCount = $derived(view?.selectedPaths?.length || 0);
  let selectedBytes = $derived.by(() => {
    if (!view?.entries || !view?.selectedPaths) return 0;
    const set = new Set(view.selectedPaths);
    return view.entries
      .filter((e) => set.has(e.path) && !e.isDir)
      .reduce((sum, e) => sum + (e.size || 0), 0);
  });

  let freeSpaceText = $derived.by(() => {
    if (view?.vfsInfo && view.vfsInfo.freeSpace > 0) {
      return ` · free: ${formatBytes(view.vfsInfo.freeSpace)}`;
    }
    return '';
  });

  // Action handlers
  function handleNavigate(targetPath: string) {
    loadDir(tmpID, targetPath);
  }

  function handleOpenStartFolder() {
    loadDir(tmpID, view?.startPath || '/');
  }

  function handleCopyCurrentPath() {
    if (view?.currentPath) {
      navigator.clipboard.writeText(view.currentPath);
    }
  }

  function handleCopyPath(p: string) {
    navigator.clipboard.writeText(p);
  }

  function handleCopyPathToTerminal(p: string) {
    const quoted = `'${p.replace(/'/g, "'\\''")}' `;
    sendSessionInput(tmpID, quoted);
  }

  function handleCdInTerminal(p: string) {
    const cmd = `cd '${p.replace(/'/g, "'\\''")}'\n`;
    sendSessionInput(tmpID, cmd);
  }

  function handleGoToTerminalFolder() {
    goToTerminalFolder(tmpID, lastTerminalCwd);
  }

  // Errors of file operations (e.g. missing permissions) are shown in a dialog.
  let actionError = $state('');
  async function run(action: () => Promise<unknown>) {
    try {
      await action();
    } catch (err: any) {
      actionError = err?.message || String(err);
    }
  }

  // Existing targets: ask whether to overwrite or skip them (or cancel).
  let conflict = $state<{ names: string[]; resolve: (choice: 'overwrite' | 'skip' | 'cancel') => void } | null>(null);
  function askConflict(names: string[]): Promise<'overwrite' | 'skip' | 'cancel'> {
    return new Promise((resolve) => (conflict = { names, resolve }));
  }
  function answerConflict(choice: 'overwrite' | 'skip' | 'cancel') {
    conflict?.resolve(choice);
    conflict = null;
  }

  /** Copies/moves (local tab) or uploads (SFTP) paths into dstDir after resolving
   *  name conflicts. */
  async function transfer(op: 'upload' | 'copy' | 'move', paths: string[], dstDir: string) {
    if (paths.length === 0) return;
    await run(async () => {
      const names = await findConflicts(tmpID, paths, dstDir);
      let overwrite = false;
      if (names.length > 0) {
        const choice = await askConflict(names);
        if (choice === 'cancel') return;
        overwrite = choice === 'overwrite';
        if (!overwrite) {
          const skip = new Set(names);
          paths = paths.filter((p) => !skip.has(p.split(/[/\\]/).pop() || p));
          if (paths.length === 0) return;
        }
      }
      if (op === 'upload') {
        await uploadFiles(tmpID, paths);
      } else {
        await copyOrMoveLocal(tmpID, op, paths, dstDir, overwrite);
      }
    });
  }

  async function handleTriggerUploadDialog() {
    try {
      const path = await SystemService.ChooseFile('Upload file', '', false);
      if (path) await transfer('upload', [path], view?.currentPath || '/');
    } catch (err) {
      console.warn('Upload dialog failed:', err);
    }
  }

  // Files dropped from the desktop arrive from the backend with their real paths:
  // uploaded on SSH tabs, copied into the current folder on local tabs.
  function handleNativeDrop(ev: any) {
    const data = ev?.data ?? ev;
    if (data?.attributes?.['data-sftp-drop'] !== tmpID) return;
    const files: string[] = Array.isArray(data?.files) ? data.files : [];
    if (files.length === 0 || !view?.currentPath) return;
    if (isLocal) {
      transfer('copy', files, view.currentPath);
    } else if (isConnected) {
      transfer('upload', files, view.currentPath);
    }
  }

  function handleDownload(paths: string[], targetDir = '') {
    run(() => downloadFiles(tmpID, paths, targetDir));
  }

  // Local tabs: copy/move the selection into a folder chosen in a dialog.
  async function handleCopyOrMoveTo(op: 'copy' | 'move', paths: string[]) {
    if (paths.length === 0) return;
    let dir = '';
    try {
      dir = await SystemService.ChooseFile(op === 'copy' ? 'Copy to folder' : 'Move to folder', view?.currentPath || '', true);
    } catch (err) {
      console.warn('Folder dialog failed:', err);
    }
    if (dir) await transfer(op, paths, dir);
  }

  // Dialog actions
  function openNewFolderDialog() {
    newFolderName = '';
    newFolderOpen = true;
  }

  async function confirmNewFolder() {
    if (newFolderName.trim()) {
      newFolderOpen = false;
      await run(() => createFolder(tmpID, newFolderName.trim()));
    }
  }

  function openNewFileDialog() {
    newFileName = '';
    newFileOpen = true;
  }

  async function confirmNewFile() {
    if (newFileName.trim()) {
      newFileOpen = false;
      await run(() => createEmptyFile(tmpID, newFileName.trim()));
    }
  }

  function openRenameDialog(fromPath: string) {
    renameFromPath = fromPath;
    renameNewName = fromPath.split('/').pop() || '';
    renameOpen = true;
  }

  async function confirmRename() {
    if (renameNewName.trim() && renameFromPath) {
      renameOpen = false;
      await run(() => renameItem(tmpID, renameFromPath, renameNewName.trim()));
    }
  }

  // Deleting asks first unless "Confirm before deleting" is turned off.
  function openDeleteDialog(paths: string[]) {
    deletePaths = paths;
    if ($settings?.sftpBrowser?.ConfirmDelete ?? true) {
      deleteConfirmOpen = true;
    } else {
      confirmDelete();
    }
  }

  async function confirmDelete() {
    deleteConfirmOpen = false;
    const paths = deletePaths;
    if (paths.length > 0) {
      await run(() => removeItems(tmpID, paths));
    }
  }

  function openPermissionsDialog(paths: string[]) {
    permissionsPaths = paths;
    const entry = view?.entries?.find((e) => e.path === paths[0]);
    permissionsInitialMode = entry ? entry.mode : 0o755;
    permissionsOpen = true;
  }

  async function confirmPermissions(mode: number, recursive: boolean, scope: string) {
    permissionsOpen = false;
    if (permissionsPaths.length > 0) {
      await run(() => chmodItems(tmpID, permissionsPaths, mode, recursive, scope));
    }
  }
</script>

<aside
  class="sftp-panel-container {isResizing ? 'is-resizing' : ''}"
  class:full-width={fullWidth}
  class:on-right={side === 'right'}
  style={fullWidth ? '' : `width: ${shownWidth}px;`}
  aria-label="SFTP Side Panel"
>

  <!-- Header -->
  <div class="sftp-header">
    <span class="header-title"><b>{isLocal ? 'File Browser' : 'SFTP Browser'}</b></span>
    {#if onclose}
      <button type="button" class="close-sftp-btn" title="Close SFTP" onclick={onclose}>
        ×
      </button>
    {/if}
  </div>

  <!-- Toolbar -->
  <SftpToolbar
    canBack={(view?.historyIndex ?? 0) > 0}
    canForward={(view?.historyIndex ?? 0) < (view?.history?.length ?? 0) - 1}
    canUp={(view?.currentPath ?? '/') !== '/' && (view?.currentPath ?? '') !== ''}
    hasSelection={(view?.selectedPaths?.length ?? 0) > 0}
    showHidden={view?.showHidden ?? false}
    followTerminal={view?.followTerminal ?? false}
    local={isLocal}
    onback={() => navigateBack(tmpID)}
    onforward={() => navigateForward(tmpID)}
    onup={() => navigateUp(tmpID)}
    onhome={() => goHome(tmpID)}
    onrefresh={() => refresh(tmpID)}
    onnewfolder={openNewFolderDialog}
    onupload={handleTriggerUploadDialog}
    ondownload={() => handleDownload(view?.selectedPaths || [])}
    ontoggleshowhidden={() => toggleShowHidden(tmpID)}
    ontogglefollowterminal={() => toggleFollowTerminal(tmpID)}
    ongototerminalfolder={handleGoToTerminalFolder}
    onopenstartfolder={handleOpenStartFolder}
    oncopycurrentpath={handleCopyCurrentPath}
    onhidebrowser={() => onclose && onclose()}
  />

  <!-- Path Bar -->
  <SftpPathBar
    currentPath={view?.currentPath || '/'}
    history={view?.history || []}
    followTerminal={view?.followTerminal ?? false}
    onnavigate={handleNavigate}
    ontogglefollow={() => toggleFollowTerminal(tmpID)}
  />

  {#if view?.error}
    <div class="sftp-error" role="alert" title={view.error}>{view.error}</div>
  {/if}

  <!-- File List Area -->
  <div class="file-list-wrap">
    <SftpFileList
      local={isLocal}
      oncopyTo={(paths) => handleCopyOrMoveTo('copy', paths)}
      onmoveTo={(paths) => handleCopyOrMoveTo('move', paths)}
      entries={view?.entries || []}
      currentPath={view?.currentPath || '/'}
      selectedPaths={view?.selectedPaths || []}
      loading={view?.loading ?? false}
      onselect={(paths) => setSelectedPaths(tmpID, paths)}
      onopen={handleNavigate}
      onopenWithDefaultApp={handleOpenFile}
      onnavigateUp={() => navigateUp(tmpID)}
      ondownload={handleDownload}
      dropId={tmpID}
      onrename={openRenameDialog}
      ondelete={openDeleteDialog}
      onnewfolder={openNewFolderDialog}
      onnewfile={openNewFileDialog}
      onpermissions={openPermissionsDialog}
      onrefresh={() => refresh(tmpID)}
      oncopyPath={handleCopyPath}
      oncopyPathToTerminal={handleCopyPathToTerminal}
      oncdInTerminal={handleCdInTerminal}
      onuploadDialog={handleTriggerUploadDialog}
      ontoggleshowhidden={() => toggleShowHidden(tmpID)}
    />

    <!-- Disconnected Overlay -->
    {#if !isConnected}
      <div class="disconnected-overlay">
        <span>Disconnected</span>
      </div>
    {/if}
  </div>

  <!-- Transfer Queue (collapsible) -->
  <SftpTransferQueue
    transfers={view?.transfers || []}
    oncancel={(id) => cancelTransfer(tmpID, id)}
    ondismiss={(id) => dismissTransfer(tmpID, id)}
  />

  <!-- Footer -->
  <div class="sftp-footer">
    <span class="footer-text">
      {#if openingFile}Opening {openingFile} …{:else}{totalItems} items{#if selectedCount > 0}, {selectedCount} selected ({formatBytes(selectedBytes)}){/if}{freeSpaceText}{/if}
    </span>
  </div>

  {#if !fullWidth}
    <!-- Resizer handle on the border towards the terminal -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="sftp-resizer"
      class:on-right={side === 'right'}
      class:active={isResizing}
      role="separator"
      aria-orientation="vertical"
      onpointerdown={startResize}
      onpointermove={moveResize}
      onpointerup={endResize}
      onpointercancel={endResize}
    ></div>
  {/if}
</aside>

<!-- Dialogs -->
<Dialog open={newFolderOpen} title="New Folder" width={340} onclose={() => (newFolderOpen = false)}>
  <div class="dialog-body">
    <label for="new-folder-name">Folder name:</label>
    <input
      id="new-folder-name"
      type="text"
      class="gtk-entry"
      bind:value={newFolderName}
      onkeydown={(e) => e.key === 'Enter' && confirmNewFolder()}
    />
  </div>
  {#snippet actions()}
    <Button label="Cancel" onclick={() => (newFolderOpen = false)} />
    <Button label="Create" isDefault onclick={confirmNewFolder} />
  {/snippet}
</Dialog>

<Dialog open={newFileOpen} title="New Empty File" width={340} onclose={() => (newFileOpen = false)}>
  <div class="dialog-body">
    <label for="new-file-name">File name:</label>
    <input
      id="new-file-name"
      type="text"
      class="gtk-entry"
      bind:value={newFileName}
      onkeydown={(e) => e.key === 'Enter' && confirmNewFile()}
    />
  </div>
  {#snippet actions()}
    <Button label="Cancel" onclick={() => (newFileOpen = false)} />
    <Button label="Create" isDefault onclick={confirmNewFile} />
  {/snippet}
</Dialog>

<Dialog open={renameOpen} title="Rename" width={340} onclose={() => (renameOpen = false)}>
  <div class="dialog-body">
    <label for="rename-input">New name:</label>
    <input
      id="rename-input"
      type="text"
      class="gtk-entry"
      bind:value={renameNewName}
      onkeydown={(e) => e.key === 'Enter' && confirmRename()}
    />
  </div>
  {#snippet actions()}
    <Button label="Cancel" onclick={() => (renameOpen = false)} />
    <Button label="Rename" isDefault onclick={confirmRename} />
  {/snippet}
</Dialog>

<MessageDialog
  open={deleteConfirmOpen}
  type="warning"
  title="Confirm Delete"
  primaryText={deletePaths.length === 1
    ? `Delete '${deletePaths[0].split('/').pop()}'? This cannot be undone.`
    : `Delete ${deletePaths.length} items? This cannot be undone.`}
  buttons="yes_no"
  onresponse={(resp) => {
    if (resp === 'yes') {
      confirmDelete();
    } else {
      deleteConfirmOpen = false;
    }
  }}
/>

<MessageDialog
  open={uploadBackDialogOpen}
  type="question"
  title="File Modified Externally"
  primaryText="Upload changes back to server?"
  secondaryText={`Remote file '${modifiedRemotePath}' was modified externally. Do you want to upload the saved changes back?`}
  buttons="yes_no"
  onresponse={async (resp) => {
    uploadBackDialogOpen = false;
    if (resp === 'yes' && modifiedLocalPath && modifiedRemotePath) {
      await uploadBack(tmpID, modifiedLocalPath, modifiedRemotePath);
    }
  }}
/>

<MessageDialog
  open={!!actionError}
  type="error"
  title={isLocal ? 'File Browser' : 'SFTP Browser'}
  secondaryText={actionError}
  buttons="ok"
  onresponse={() => (actionError = '')}
/>

<Dialog open={!!conflict} title="File already exists" width={420} onclose={() => answerConflict('cancel')}>
  <div class="dialog-body">
    <span>
      {conflict?.names.length === 1
        ? `'${conflict.names[0]}' already exists in the target folder.`
        : `${conflict?.names.length} items already exist in the target folder:`}
    </span>
    {#if (conflict?.names.length ?? 0) > 1}
      <ul class="conflict-list">
        {#each conflict?.names.slice(0, 10) ?? [] as n (n)}
          <li>{n}</li>
        {/each}
        {#if (conflict?.names.length ?? 0) > 10}
          <li>… and {(conflict?.names.length ?? 0) - 10} more</li>
        {/if}
      </ul>
    {/if}
  </div>
  {#snippet actions()}
    <Button label="Cancel" onclick={() => answerConflict('cancel')} />
    <Button label="Skip existing" onclick={() => answerConflict('skip')} />
    <Button label="Overwrite" isDefault onclick={() => answerConflict('overwrite')} />
  {/snippet}
</Dialog>

<MessageDialog
  open={!!openError}
  type="error"
  title="Open file"
  secondaryText={openError}
  buttons="ok"
  onresponse={() => (openError = '')}
/>

<SftpPermissionsDialog
  open={permissionsOpen}
  paths={permissionsPaths}
  initialMode={permissionsInitialMode}
  onapply={confirmPermissions}
  onclose={() => (permissionsOpen = false)}
/>

<style>
  .sftp-panel-container {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 220px;
    background: var(--gtk-bg, #f6f6f6);
    border-right: 1px solid var(--gtk-border, #d3d7cf);
    box-sizing: border-box;
    flex-shrink: 0;
    overflow: hidden;
  }

  .sftp-panel-container.on-right {
    border-right: none;
    border-left: 1px solid var(--gtk-border, #d3d7cf);
  }

  .sftp-panel-container.full-width {
    width: 100%;
    flex: 1;
    border-right: none;
  }


  .sftp-panel-container.is-resizing {
    transition: none;
    user-select: none;
  }

  .sftp-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    background: var(--gtk-header-bg, #ececec);
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    font-size: 12px;
    color: var(--gtk-fg, #2e3436);
  }

  .close-sftp-btn {
    background: transparent;
    border: none;
    font-size: 16px;
    line-height: 1;
    cursor: pointer;
    color: var(--gtk-muted-fg);
    padding: 0 4px;
    border-radius: 2px;
  }

  .close-sftp-btn:hover {
    background: var(--gtk-row-hover);
    color: var(--gtk-fg-color);
  }

  .file-list-wrap {
    flex: 1;
    position: relative;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .disconnected-overlay {
    position: absolute;
    inset: 0;
    background: color-mix(in srgb, var(--gtk-bg-color) 85%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: bold;
    color: var(--gtk-muted-fg);
    z-index: 10;
  }

  .sftp-footer {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    height: var(--leanterm-bottom-bar-height);
    box-sizing: border-box;
    padding: 0 6px;
    background: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
    font-size: 11px;
    color: var(--gtk-muted-fg);
  }

  .footer-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sftp-resizer {
    position: absolute;
    top: 0;
    right: 0;
    width: 5px;
    height: 100%;
    cursor: col-resize;
    z-index: 20;
  }

  .sftp-error {
    padding: 4px 8px;
    font-size: 11px;
    color: var(--gtk-error-fg);
    background: var(--gtk-error-bg);
    border-bottom: 1px solid var(--gtk-error-border);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .conflict-list {
    margin: 0;
    padding-left: 18px;
    max-height: 160px;
    overflow-y: auto;
  }

  .sftp-resizer.on-right {
    right: auto;
    left: 0;
  }

  .sftp-resizer:hover,
  .sftp-resizer.active {
    background: color-mix(in srgb, var(--gtk-accent) 45%, transparent);
  }

  .dialog-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 12px;
  }

  .gtk-entry {
    font-size: 12px;
    padding: 5px 8px;
    background: var(--gtk-base, #ffffff);
    border: 1px solid var(--gtk-border, #c0c0c0);
    border-radius: 3px;
    color: var(--gtk-fg, #2e3436);
    outline: none;
  }

  .gtk-entry:focus {
    border-color: var(--gtk-selected-bg, #4a90d9);
  }
</style>
