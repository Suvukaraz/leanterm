<script lang="ts">
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    canBack: boolean;
    canForward: boolean;
    canUp: boolean;
    hasSelection: boolean;
    showHidden: boolean;
    followTerminal: boolean;
    /** Local file browser: no upload/download buttons. */
    local?: boolean;
    onback: () => void;
    onforward: () => void;
    onup: () => void;
    onhome: () => void;
    onrefresh: () => void;
    onnewfolder: () => void;
    onupload: () => void;
    ondownload: () => void;
    ontoggleshowhidden: () => void;
    ontogglefollowterminal: () => void;
    ongototerminalfolder: () => void;
    onopenstartfolder: () => void;
    oncopycurrentpath: () => void;
    onhidebrowser: () => void;
  }

  let {
    canBack,
    canForward,
    canUp,
    hasSelection,
    showHidden,
    followTerminal,
    local = false,
    onback,
    onforward,
    onup,
    onhome,
    onrefresh,
    onnewfolder,
    onupload,
    ondownload,
    ontoggleshowhidden,
    ontogglefollowterminal,
    ongototerminalfolder,
    onopenstartfolder,
    oncopycurrentpath,
    onhidebrowser,
  }: Props = $props();

  let menuOpen = $state(false);

  function toggleMenu(e: MouseEvent) {
    e.stopPropagation();
    menuOpen = !menuOpen;
  }

  function closeMenu() {
    menuOpen = false;
  }
</script>

<svelte:window onclick={closeMenu} />

<div class="sftp-toolbar" role="toolbar" aria-label="SFTP Operations Toolbar">
  <button
    type="button"
    class="button-cp"
    title="Back"
    disabled={!canBack}
    onclick={onback}
  >
    <img src={getIcon('gtk-go-back')} alt="Back" width="16" height="16" />
  </button>

  <button
    type="button"
    class="button-cp"
    title="Forward"
    disabled={!canForward}
    onclick={onforward}
  >
    <img src={getIcon('gtk-go-forward')} alt="Forward" width="16" height="16" />
  </button>

  <button
    type="button"
    class="button-cp"
    title="Parent folder"
    disabled={!canUp}
    onclick={onup}
  >
    <img src={getIcon('gtk-go-up')} alt="Up" width="16" height="16" />
  </button>

  <button
    type="button"
    class="button-cp"
    title="Home folder"
    onclick={onhome}
  >
    <img src={getIcon('gtk-home')} alt="Home" width="16" height="16" />
  </button>

  <button
    type="button"
    class="button-cp"
    title="Refresh"
    onclick={onrefresh}
  >
    <img src={getIcon('gtk-refresh')} alt="Refresh" width="16" height="16" />
  </button>

  <span class="toolbar-sep"></span>

  <button
    type="button"
    class="button-cp"
    title="New folder"
    onclick={onnewfolder}
  >
    <img src={getIcon('gtk-directory')} alt="New folder" width="16" height="16" />
  </button>

  {#if !local}
    <button
      type="button"
      class="button-cp"
      title="Upload files..."
      onclick={onupload}
    >
      <img src={getIcon('gtk-open')} alt="Upload" width="16" height="16" />
    </button>

    <button
      type="button"
      class="button-cp"
      title="Download selected"
      disabled={!hasSelection}
      onclick={ondownload}
    >
      <img src={getIcon('gtk-save')} alt="Download" width="16" height="16" />
    </button>
  {/if}

  <span class="toolbar-sep"></span>

  <div class="menu-container">
    <button
      type="button"
      class="button-cp {menuOpen ? 'is-active' : ''}"
      title="More actions"
      onclick={toggleMenu}
    >
      <span class="hamburger-icon">≡</span>
    </button>

    {#if menuOpen}
      <div class="dropdown-menu" role="menu">
        <button
          type="button"
          class="dropdown-item"
          role="menuitemcheckbox"
          aria-checked={showHidden}
          onclick={() => {
            ontoggleshowhidden();
            closeMenu();
          }}
        >
          <span class="check-box">{showHidden ? '✓' : ''}</span>
          <span>Show hidden files</span>
        </button>

        <button
          type="button"
          class="dropdown-item"
          role="menuitemcheckbox"
          aria-checked={followTerminal}
          onclick={() => {
            ontogglefollowterminal();
            closeMenu();
          }}
        >
          <span class="check-box">{followTerminal ? '✓' : ''}</span>
          <span>Follow terminal folder</span>
        </button>

        <button
          type="button"
          class="dropdown-item"
          role="menuitem"
          onclick={() => {
            ongototerminalfolder();
            closeMenu();
          }}
        >
          <span class="item-spacer"></span>
          <span>Go to terminal folder</span>
        </button>

        <button
          type="button"
          class="dropdown-item"
          role="menuitem"
          onclick={() => {
            onopenstartfolder();
            closeMenu();
          }}
        >
          <span class="item-spacer"></span>
          <span>{local ? 'Open start folder' : 'Open SFTP start folder'}</span>
        </button>

        <button
          type="button"
          class="dropdown-item"
          role="menuitem"
          onclick={() => {
            oncopycurrentpath();
            closeMenu();
          }}
        >
          <span class="item-spacer"></span>
          <span>Copy current path</span>
        </button>

        <div class="dropdown-divider"></div>

        <button
          type="button"
          class="dropdown-item"
          role="menuitem"
          onclick={() => {
            onhidebrowser();
            closeMenu();
          }}
        >
          <span class="item-spacer"></span>
          <span>Hide {local ? 'file' : 'SFTP'} browser</span>
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  .sftp-toolbar {
    display: flex;
    align-items: center;
    background: var(--gtk-header-bg, #f6f6f6);
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    padding: 3px 6px;
    gap: 3px;
    user-select: none;
  }

  .button-cp {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 3px 5px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--gtk-fg, #2e3436);
  }

  .button-cp:hover:not(:disabled) {
    background-color: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }

  .button-cp:active:not(:disabled),
  .button-cp.is-active {
    background-color: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }

  .button-cp:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .toolbar-sep {
    width: 1px;
    height: 16px;
    background: var(--gtk-border, #d3d7cf);
    margin: 0 3px;
  }

  .hamburger-icon {
    font-size: 16px;
    font-weight: bold;
    line-height: 1;
    width: 16px;
    height: 16px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .menu-container {
    position: relative;
    display: inline-block;
    margin-left: auto;
  }

  .dropdown-menu {
    position: absolute;
    right: 0;
    top: 100%;
    margin-top: 4px;
    background: var(--gtk-menu-bg, #ffffff);
    border: 1px solid var(--gtk-border, #c0c0c0);
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.15);
    border-radius: 3px;
    padding: 4px 0;
    min-width: 200px;
    z-index: 1000;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 12px;
    font-size: 12px;
    text-align: left;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--gtk-fg, #2e3436);
  }

  .dropdown-item:hover {
    background: var(--gtk-selected-bg, #4a90d9);
    color: var(--gtk-selected-fg, #ffffff);
  }

  .check-box {
    width: 14px;
    font-weight: bold;
    text-align: center;
  }

  .item-spacer {
    width: 14px;
  }

  .dropdown-divider {
    height: 1px;
    background: var(--gtk-border, #e0e0e0);
    margin: 4px 0;
  }
</style>
