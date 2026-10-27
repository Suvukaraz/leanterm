<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import {
    sftpViews,
    openSftp,
    loadDir,
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
  } from '../../lib/stores/sftp';
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
    onclose?: () => void;
  }

  let { tmpID, fullWidth = false, side = 'left', onclose }: Props = $props();

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
  let dropUnsubscribe: (() => void) | null = null;
  let fileModifiedUnsubscribe: (() => void) | null = null;
  let pollInterval: any = null;

  // File modification dialog state
  let uploadBackDialogOpen = $state(false);
  let modifiedRemotePath = $state('');
  let modifiedLocalPath = $state('');

  onMount(() => {
    // If not already opened, initialize SFTP
    if (!view || !view.sftpID) {
      openSftp(tmpID);
    }

    const offDrop = Events.On('files:dropped', handleNativeDrop);
    dropUnsubscribe = offDrop;

    // Subscribe to OSC 7 cwd events
    try {
      osc7Unsubscribe = Events.On(`session:cwd:${tmpID}`, (event: any) => {
        const cwd = typeof event?.data === 'string' ? event.data : String(event || '');
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

  // Open a remote file in its default desktop application (downloaded to a temp
  // folder; saving it there offers to upload the changes back).
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
    loadDir(tmpID, '/');
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
    // Navigates to terminal folder if known
    if (view?.currentPath) {
      loadDir(tmpID, view.currentPath);
    }
  }

  async function handleTriggerUploadDialog() {
    try {
      const path = await SystemService.ChooseFile('Upload file', '', false);
      if (path) await uploadFiles(tmpID, [path]);
    } catch (err) {
      console.warn('Upload dialog failed:', err);
    }
  }

  // Files dropped from the desktop arrive from the backend with their real paths.
  function handleNativeDrop(ev: any) {
    const data = ev?.data ?? ev;
    if (data?.attributes?.['data-sftp-drop'] !== tmpID) return;
    const files: string[] = Array.isArray(data?.files) ? data.files : [];
    if (files.length > 0 && isConnected) uploadFiles(tmpID, files);
  }

  function handleDownload(paths: string[], targetDir = '') {
    downloadFiles(tmpID, paths, targetDir);
  }

  // Dialog actions
  function openNewFolderDialog() {
    newFolderName = '';
    newFolderOpen = true;
  }

  async function confirmNewFolder() {
    if (newFolderName.trim()) {
      newFolderOpen = false;
      await createFolder(tmpID, newFolderName.trim());
    }
  }

  function openNewFileDialog() {
    newFileName = '';
    newFileOpen = true;
  }

  async function confirmNewFile() {
    if (newFileName.trim()) {
      newFileOpen = false;
      await createEmptyFile(tmpID, newFileName.trim());
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
      await renameItem(tmpID, renameFromPath, renameNewName.trim());
    }
  }

  function openDeleteDialog(paths: string[]) {
    deletePaths = paths;
    deleteConfirmOpen = true;
  }

  async function confirmDelete() {
    deleteConfirmOpen = false;
    if (deletePaths.length > 0) {
      await removeItems(tmpID, deletePaths);
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
      await chmodItems(tmpID, permissionsPaths, mode, recursive, scope);
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
    <span class="header-title"><b>SFTP Browser</b></span>
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
    onback={() => navigateBack(tmpID)}
    onforward={() => navigateForward(tmpID)}
    onup={() => navigateUp(tmpID)}
    onhome={() => loadDir(tmpID, '/')}
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

  <!-- File List Area -->
  <div class="file-list-wrap">
    <SftpFileList
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
    padding: 3px 6px;
    background: var(--gtk-header-bg, #ececec);
    border-top: 1px solid var(--gtk-border, #d3d7cf);
    font-size: 10px;
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
