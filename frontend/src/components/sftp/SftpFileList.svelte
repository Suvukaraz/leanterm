<script lang="ts">
  import type { FileEntry } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/sftpsvc/models.js';
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    entries: FileEntry[];
    currentPath: string;
    selectedPaths: string[];
    loading: boolean;
    onselect: (paths: string[]) => void;
    onopen: (p: string) => void;
    onopenWithDefaultApp?: (p: string) => void;
    /** Opens a file in LeanTerm's editor window. */
    onopenInEditor?: (p: string) => void;
    /** Whether a double-click opens the file in the editor (else the default application). */
    opensInEditor?: (p: string) => boolean;
    onnavigateUp: () => void;
    ondownload: (paths: string[], targetDir?: string) => void;
    /** Identifies this list in native "files:dropped" events. */
    dropId: string;
    onrename: (fromPath: string) => void;
    ondelete: (paths: string[]) => void;
    onnewfolder: () => void;
    onnewfile: () => void;
    onpermissions: (paths: string[]) => void;
    onrefresh: () => void;
    oncopyPath: (p: string) => void;
    oncopyPathToTerminal: (p: string) => void;
    oncdInTerminal: (p: string) => void;
    onuploadDialog: () => void;
    ontoggleshowhidden: () => void;
    /** Local file browser: "Copy to…"/"Move to…" instead of download/upload. */
    local?: boolean;
    /** At the top of the tree (no ".." row). */
    atRoot?: boolean;
    /** Windows paths: no Unix permissions. */
    win?: boolean;
    oncopyTo?: (paths: string[]) => void;
    onmoveTo?: (paths: string[]) => void;
  }

  let {
    entries,
    currentPath,
    selectedPaths,
    loading,
    onselect,
    onopen,
    onopenWithDefaultApp,
    onopenInEditor,
    opensInEditor,
    onnavigateUp,
    ondownload,
    dropId,
    onrename,
    ondelete,
    onnewfolder,
    onnewfile,
    onpermissions,
    onrefresh,
    oncopyPath,
    oncopyPathToTerminal,
    oncdInTerminal,
    onuploadDialog,
    ontoggleshowhidden,
    local = false,
    atRoot = false,
    win = false,
    oncopyTo,
    onmoveTo,
  }: Props = $props();

  let sortCol = $state<'name' | 'size' | 'modTime' | 'permissions'>('name');
  let sortAsc = $state(true);

  let isDragOver = $state(false);

  // Context menu state
  let contextMenuOpen = $state(false);
  let contextMenuX = $state(0);
  let contextMenuY = $state(0);
  let contextMenuItem = $state<FileEntry | null>(null);

  // Type-ahead search buffer
  let typeAheadBuffer = '';
  let typeAheadTimer: ReturnType<typeof setTimeout> | null = null;

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  function formatDate(val: any): string {
    if (!val) return '';
    const d = new Date(val);
    if (isNaN(d.getTime())) return '';
    const pad = (n: number) => (n < 10 ? '0' + n : n.toString());
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  // Sorted entries (folders always first, then files)
  let sortedEntries = $derived.by(() => {
    const list = [...entries];
    list.sort((a, b) => {
      // Folders always first
      if (a.isDir && !b.isDir) return -1;
      if (!a.isDir && b.isDir) return 1;

      let res = 0;
      if (sortCol === 'name') {
        res = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' });
      } else if (sortCol === 'size') {
        res = (a.size || 0) - (b.size || 0);
      } else if (sortCol === 'modTime') {
        const tA = new Date(a.modTime).getTime() || 0;
        const tB = new Date(b.modTime).getTime() || 0;
        res = tA - tB;
      } else if (sortCol === 'permissions') {
        res = (a.permissions || '').localeCompare(b.permissions || '');
      }

      return sortAsc ? res : -res;
    });
    return list;
  });

  function toggleSort(col: 'name' | 'size' | 'modTime' | 'permissions') {
    if (sortCol === col) {
      sortAsc = !sortAsc;
    } else {
      sortCol = col;
      sortAsc = true;
    }
  }

  function handleRowClick(e: MouseEvent, entry: FileEntry) {
    if (e.ctrlKey || e.metaKey) {
      if (selectedPaths.includes(entry.path)) {
        onselect(selectedPaths.filter((p) => p !== entry.path));
      } else {
        onselect([...selectedPaths, entry.path]);
      }
    } else if (e.shiftKey && selectedPaths.length > 0) {
      const lastSelected = selectedPaths[selectedPaths.length - 1];
      const fromIdx = sortedEntries.findIndex((item) => item.path === lastSelected);
      const toIdx = sortedEntries.findIndex((item) => item.path === entry.path);
      if (fromIdx !== -1 && toIdx !== -1) {
        const start = Math.min(fromIdx, toIdx);
        const end = Math.max(fromIdx, toIdx);
        const slice = sortedEntries.slice(start, end + 1).map((item) => item.path);
        onselect(Array.from(new Set([...selectedPaths, ...slice])));
      }
    } else {
      onselect([entry.path]);
    }
  }

  // Double click / Enter: folders are entered, text files open in the editor (if
  // configured), other files in their default application.
  function handleRowDoubleClick(entry: FileEntry) {
    if (entry.isDir) {
      onopen(entry.path);
    } else if (onopenInEditor && opensInEditor?.(entry.path)) {
      onopenInEditor(entry.path);
    } else if (onopenWithDefaultApp) {
      onopenWithDefaultApp(entry.path);
    } else {
      ondownload([entry.path]);
    }
  }

  function handleItemContextMenu(e: MouseEvent, entry: FileEntry) {
    e.preventDefault();
    e.stopPropagation();

    if (!selectedPaths.includes(entry.path)) {
      onselect([entry.path]);
    }

    contextMenuItem = entry;
    contextMenuX = e.clientX;
    contextMenuY = e.clientY;
    contextMenuOpen = true;
  }

  function handleEmptyContextMenu(e: MouseEvent) {
    e.preventDefault();
    contextMenuItem = null;
    contextMenuX = e.clientX;
    contextMenuY = e.clientY;
    contextMenuOpen = true;
  }

  function closeContextMenu() {
    contextMenuOpen = false;
  }

  // Only reacts while the file list itself has keyboard focus (not the terminal).
  function handleKeyDown(e: KeyboardEvent) {
    if ((e.target as HTMLElement)?.closest?.('input, textarea, select')) return;
    if (e.key === 'F5') {
      e.preventDefault();
      onrefresh();
    } else if (e.key === 'F2') {
      if (selectedPaths.length === 1) {
        e.preventDefault();
        onrename(selectedPaths[0]);
      }
    } else if (e.key === 'F7') {
      e.preventDefault();
      onnewfolder();
    } else if (e.key === 'Delete') {
      if (selectedPaths.length > 0) {
        e.preventDefault();
        ondelete(selectedPaths);
      }
    } else if (e.key === 'Enter') {
      if (selectedPaths.length === 1) {
        const item = sortedEntries.find((en) => en.path === selectedPaths[0]);
        if (item) {
          e.preventDefault();
          handleRowDoubleClick(item);
        }
      }
    } else if (e.key === 'Backspace' || (e.altKey && e.key === 'ArrowUp')) {
      e.preventDefault();
      onnavigateUp();
    } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // Type-ahead jump
      typeAheadBuffer += e.key.toLowerCase();
      if (typeAheadTimer) clearTimeout(typeAheadTimer);
      typeAheadTimer = setTimeout(() => {
        typeAheadBuffer = '';
      }, 700);

      const match = sortedEntries.find((en) =>
        en.name.toLowerCase().startsWith(typeAheadBuffer)
      );
      if (match) {
        onselect([match.path]);
      }
    }
  }

  // Desktop files are delivered with real paths by the backend ("files:dropped",
  // see data-file-drop-target below); this only drives the highlight.
  function handleDragOver(e: DragEvent) {
    if (e.dataTransfer?.types.includes('Files')) {
      e.preventDefault();
      isDragOver = true;
    }
  }

  function handleDragLeave() {
    isDragOver = false;
  }

  function handleDrop(e: DragEvent) {
    isDragOver = false;
    if (e.dataTransfer?.types.includes('Files')) e.preventDefault();
  }
</script>

<svelte:window onclick={closeContextMenu} />

<div
  class="sftp-filelist-container {isDragOver ? 'is-drag-over' : ''}"
  tabindex="0"
  role="grid"
  aria-label={local ? 'Local File List' : 'Remote File List'}
  data-file-drop-target
  data-sftp-drop={dropId}
  onkeydown={handleKeyDown}
  oncontextmenu={handleEmptyContextMenu}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
>
  <div class="sftp-table-header" role="row">
    <button
      type="button"
      class="header-cell cell-name"
      onclick={() => toggleSort('name')}
    >
      <span>Name</span>
      {#if sortCol === 'name'}
        <span class="sort-indicator">{sortAsc ? '▲' : '▼'}</span>
      {/if}
    </button>

    <button
      type="button"
      class="header-cell cell-size"
      onclick={() => toggleSort('size')}
    >
      <span>Size</span>
      {#if sortCol === 'size'}
        <span class="sort-indicator">{sortAsc ? '▲' : '▼'}</span>
      {/if}
    </button>

    <button
      type="button"
      class="header-cell cell-mod"
      onclick={() => toggleSort('modTime')}
    >
      <span>Modified</span>
      {#if sortCol === 'modTime'}
        <span class="sort-indicator">{sortAsc ? '▲' : '▼'}</span>
      {/if}
    </button>

    <button
      type="button"
      class="header-cell cell-perm"
      onclick={() => toggleSort('permissions')}
    >
      <span>Perm.</span>
      {#if sortCol === 'permissions'}
        <span class="sort-indicator">{sortAsc ? '▲' : '▼'}</span>
      {/if}
    </button>
  </div>

  <div class="sftp-table-body" role="rowgroup">
    {#if !atRoot}
      <!-- Parent directory row -->
      <div
        class="sftp-row parent-row"
        role="row"
        tabindex="-1"
        ondblclick={onnavigateUp}
      >
        <span class="cell cell-name">
          <img src={getIcon('gtk-directory')} alt="Dir" width="16" height="16" class="file-icon" />
          <span class="name-text">..</span>
        </span>
        <span class="cell cell-size"></span>
        <span class="cell cell-mod"></span>
        <span class="cell cell-perm"></span>
      </div>
    {/if}

    {#each sortedEntries as entry (entry.path)}
      {@const isSelected = selectedPaths.includes(entry.path)}
      <!-- Keyboard input is handled by the list container (handleKeyDown). -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        class="sftp-row {isSelected ? 'is-selected' : ''} {entry.linkBroken ? 'is-broken' : ''}"
        role="row"
        tabindex="-1"
        aria-selected={isSelected}
        onclick={(e) => handleRowClick(e, entry)}
        ondblclick={() => handleRowDoubleClick(entry)}
        oncontextmenu={(e) => handleItemContextMenu(e, entry)}
      >
        <span class="cell cell-name" title={entry.linkTarget ? `${entry.name} -> ${entry.linkTarget}` : entry.name}>
          <div class="icon-wrap">
            <img
              src={getIcon(entry.isDir ? 'gtk-directory' : 'gtk-file')}
              alt={entry.isDir ? 'Dir' : 'File'}
              width="16"
              height="16"
              class="file-icon"
            />
            {#if entry.isLink}
              <span class="symlink-badge">↪</span>
            {/if}
          </div>
          <span class="name-text">{entry.name}</span>
        </span>

        <span class="cell cell-size" title={`${entry.size || 0} bytes`}>
          {entry.isDir ? '' : formatSize(entry.size || 0)}
        </span>

        <span class="cell cell-mod">
          {formatDate(entry.modTime)}
        </span>

        <span class="cell cell-perm">
          {entry.permissions || ''}
        </span>
      </div>
    {/each}

    {#if entries.length === 0 && !loading}
      <div class="empty-hint">Directory is empty</div>
    {/if}

    {#if loading}
      <div class="loading-overlay">
        <span>Loading...</span>
      </div>
    {/if}
  </div>

  <!-- Context Menu -->
  {#if contextMenuOpen}
    <!-- The click handler only keeps the window handler from closing the menu. -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      class="context-menu"
      style="left: {contextMenuX}px; top: {contextMenuY}px;"
      role="menu"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
    >
      {#if contextMenuItem}
        {#if contextMenuItem.isDir}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              if (contextMenuItem) onopen(contextMenuItem.path);
              closeContextMenu();
            }}
          >
            Open
          </button>
        {:else}
          {#if onopenInEditor}
            <button
              type="button"
              class="context-item"
              onclick={() => {
                if (contextMenuItem) onopenInEditor(contextMenuItem.path);
                closeContextMenu();
              }}
            >
              Open in Editor
            </button>
          {/if}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              if (contextMenuItem && onopenWithDefaultApp) onopenWithDefaultApp(contextMenuItem.path);
              closeContextMenu();
            }}
          >
            Open with default application
          </button>
        {/if}

        {#if local}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              oncopyTo?.(selectedPaths);
              closeContextMenu();
            }}
          >
            Copy to…
          </button>
          <button
            type="button"
            class="context-item"
            onclick={() => {
              onmoveTo?.(selectedPaths);
              closeContextMenu();
            }}
          >
            Move to…
          </button>
        {:else}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              ondownload(selectedPaths);
              closeContextMenu();
            }}
          >
            Download
          </button>
        {/if}

        <div class="context-divider"></div>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            if (contextMenuItem) onrename(contextMenuItem.path);
            closeContextMenu();
          }}
        >
          Rename (F2)
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            ondelete(selectedPaths);
            closeContextMenu();
          }}
        >
          Delete (Del)
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            onnewfolder();
            closeContextMenu();
          }}
        >
          New folder (F7)
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            onnewfile();
            closeContextMenu();
          }}
        >
          New empty file
        </button>

        {#if !win}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              onpermissions(selectedPaths);
              closeContextMenu();
            }}
          >
            Permissions...
          </button>
        {/if}

        <div class="context-divider"></div>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            if (contextMenuItem) oncopyPath(contextMenuItem.path);
            closeContextMenu();
          }}
        >
          Copy path
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            if (contextMenuItem) oncopyPathToTerminal(contextMenuItem.path);
            closeContextMenu();
          }}
        >
          Copy path to terminal
        </button>

        {#if contextMenuItem.isDir}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              if (contextMenuItem) oncdInTerminal(contextMenuItem.path);
              closeContextMenu();
            }}
          >
            cd to this folder in terminal
          </button>
        {/if}

        <button
          type="button"
          class="context-item"
          onclick={() => {
            onrefresh();
            closeContextMenu();
          }}
        >
          Refresh (F5)
        </button>
      {:else}
        <!-- Empty area context menu -->
        <button
          type="button"
          class="context-item"
          onclick={() => {
            onnewfolder();
            closeContextMenu();
          }}
        >
          New folder
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            onnewfile();
            closeContextMenu();
          }}
        >
          New empty file
        </button>

        {#if !local}
          <button
            type="button"
            class="context-item"
            onclick={() => {
              onuploadDialog();
              closeContextMenu();
            }}
          >
            Upload files...
          </button>
        {/if}

        <div class="context-divider"></div>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            onrefresh();
            closeContextMenu();
          }}
        >
          Refresh
        </button>

        <button
          type="button"
          class="context-item"
          onclick={() => {
            ontoggleshowhidden();
            closeContextMenu();
          }}
        >
          Show hidden files
        </button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .sftp-filelist-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: var(--gtk-base, #ffffff);
    overflow: hidden;
    position: relative;
    user-select: none;
    outline: none;
  }

  .sftp-filelist-container.is-drag-over {
    border: 2px dashed var(--gtk-selected-bg, #4a90d9);
    background: rgba(74, 144, 217, 0.05);
  }

  .sftp-table-header {
    display: flex;
    background: var(--gtk-header-bg, #f0f0f0);
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    font-size: 11px;
    font-weight: 600;
  }

  .header-cell {
    background: transparent;
    border: none;
    border-right: 1px solid var(--gtk-border, #e0e0e0);
    padding: 4px 6px;
    text-align: left;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--gtk-fg, #2e3436);
  }

  .header-cell:hover {
    background: var(--gtk-header-bg);
  }

  .sort-indicator {
    font-size: 8px;
  }

  .sftp-table-body {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    position: relative;
  }

  .sftp-row {
    display: flex;
    align-items: center;
    height: 22px;
    cursor: pointer;
    font-size: 11px;
    border-bottom: 1px solid var(--gtk-border-color);
    color: var(--gtk-fg, #2e3436);
  }

  .sftp-row:hover:not(.is-selected) {
    background: var(--gtk-hover-bg, #f2f5f9);
  }

  .sftp-row.is-selected {
    background: var(--gtk-selected-bg, #4a90d9);
    color: var(--gtk-selected-fg, #ffffff);
  }

  .sftp-row.is-broken {
    opacity: 0.5;
    font-style: italic;
  }

  .cell {
    padding: 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cell-name {
    flex: 2;
    min-width: 120px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .cell-size {
    width: 60px;
    text-align: right;
  }

  .cell-mod {
    width: 105px;
    text-align: left;
  }

  .cell-perm {
    width: 80px;
    font-family: monospace;
    font-size: 10px;
  }

  .icon-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .file-icon {
    flex-shrink: 0;
  }

  .symlink-badge {
    position: absolute;
    bottom: -3px;
    right: -3px;
    font-size: 10px;
    line-height: 1;
    color: var(--gtk-fg-color);
  }

  .name-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty-hint {
    padding: 24px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
    font-size: 12px;
  }

  .loading-overlay {
    position: absolute;
    inset: 0;
    background: color-mix(in srgb, var(--gtk-base-color) 70%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    color: var(--gtk-muted-fg);
  }

  .context-menu {
    position: fixed;
    background: var(--gtk-menu-bg, #ffffff);
    border: 1px solid var(--gtk-border, #c0c0c0);
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.2);
    border-radius: 3px;
    padding: 4px 0;
    min-width: 180px;
    z-index: 2000;
  }

  .context-item {
    display: block;
    width: 100%;
    padding: 5px 12px;
    font-size: 11px;
    text-align: left;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--gtk-fg, #2e3436);
  }

  .context-item:hover {
    background: var(--gtk-selected-bg, #4a90d9);
    color: var(--gtk-selected-fg, #ffffff);
  }

  .context-divider {
    height: 1px;
    background: var(--gtk-border, #e0e0e0);
    margin: 4px 0;
  }
</style>
