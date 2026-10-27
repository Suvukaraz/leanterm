<script lang="ts">
  interface Props {
    currentPath: string;
    history: string[];
    followTerminal: boolean;
    /** Local browsers follow the shell directly, SSH tabs need OSC 7. */
    local?: boolean;
    onnavigate: (path: string) => void;
    ontogglefollow: () => void;
  }

  let { currentPath, history, followTerminal, local = false, onnavigate, ontogglefollow }: Props = $props();

  let inputVal = $state(currentPath);
  let dropdownOpen = $state(false);

  $effect(() => {
    inputVal = currentPath;
  });

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      onnavigate(inputVal);
      dropdownOpen = false;
    } else if (e.key === 'Escape') {
      e.preventDefault();
      inputVal = currentPath;
      dropdownOpen = false;
    }
  }

  function toggleDropdown(e: MouseEvent) {
    e.stopPropagation();
    dropdownOpen = !dropdownOpen;
  }

  function closeDropdown() {
    dropdownOpen = false;
  }

  function handleSelectHistory(p: string) {
    inputVal = p;
    onnavigate(p);
    dropdownOpen = false;
  }

  // Deduplicated unique recent paths (max 20)
  let uniqueHistory = $derived(
    Array.from(new Set(history.slice().reverse())).slice(0, 20)
  );
</script>

<svelte:window onclick={closeDropdown} />

<div class="sftp-pathbar-container">
  <div class="path-input-group">
    <input
      type="text"
      class="path-input"
      bind:value={inputVal}
      onkeydown={handleKeyDown}
      title="Path (press Enter to navigate, Esc to revert)"
    />
    <button
      type="button"
      class="path-dropdown-btn {dropdownOpen ? 'is-active' : ''}"
      title="Recent paths"
      onclick={toggleDropdown}
    >
      ▼
    </button>

    {#if dropdownOpen}
      <div class="history-dropdown" role="menu">
        {#if uniqueHistory.length === 0}
          <div class="history-empty">No history</div>
        {:else}
          {#each uniqueHistory as p}
            <button
              type="button"
              class="history-item {p === currentPath ? 'is-current' : ''}"
              role="menuitem"
              onclick={() => handleSelectHistory(p)}
            >
              <span class="folder-icon">📁</span>
              <span class="path-text">{p}</span>
            </button>
          {/each}
        {/if}
      </div>
    {/if}
  </div>

  <div class="follow-row">
    <label class="follow-checkbox-label">
      <input
        type="checkbox"
        checked={followTerminal}
        onchange={ontogglefollow}
      />
      <span>Follow terminal folder{local ? '' : ' (OSC 7)'}</span>
    </label>
  </div>
</div>

<style>
  .sftp-pathbar-container {
    display: flex;
    flex-direction: column;
    padding: 4px 6px;
    background: var(--gtk-bg, #f6f6f6);
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    gap: 4px;
    font-size: 12px;
  }

  .path-input-group {
    position: relative;
    display: flex;
    align-items: center;
  }

  .path-input {
    flex: 1;
    font-family: monospace;
    font-size: 12px;
    padding: 3px 6px;
    border: 1px solid var(--gtk-border, #c0c0c0);
    border-right: none;
    border-radius: 3px 0 0 3px;
    background: var(--gtk-base, #ffffff);
    color: var(--gtk-fg, #2e3436);
    outline: none;
  }

  .path-input:focus {
    border-color: var(--gtk-selected-bg, #4a90d9);
  }

  .path-dropdown-btn {
    border: 1px solid var(--gtk-border, #c0c0c0);
    border-radius: 0 3px 3px 0;
    background: var(--gtk-button-bg, #e8e8e8);
    color: var(--gtk-fg, #555753);
    padding: 3px 6px;
    font-size: 9px;
    cursor: pointer;
    line-height: 1.2;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .path-dropdown-btn:hover {
    background: var(--gtk-btn-hover-bg);
  }

  .path-dropdown-btn.is-active {
    background: var(--gtk-btn-active-bg);
  }

  .history-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    margin-top: 2px;
    background: var(--gtk-menu-bg, #ffffff);
    border: 1px solid var(--gtk-border, #c0c0c0);
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.15);
    border-radius: 3px;
    max-height: 220px;
    overflow-y: auto;
    z-index: 1001;
    padding: 2px 0;
  }

  .history-empty {
    padding: 6px 12px;
    color: var(--gtk-muted-fg);
    font-style: italic;
  }

  .history-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 4px 8px;
    font-size: 11px;
    text-align: left;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--gtk-fg, #2e3436);
  }

  .history-item:hover {
    background: var(--gtk-selected-bg, #4a90d9);
    color: #ffffff;
  }

  .history-item.is-current {
    font-weight: bold;
  }

  .folder-icon {
    font-size: 11px;
  }

  .path-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: monospace;
  }

  .follow-row {
    display: flex;
    align-items: center;
  }

  .follow-checkbox-label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--gtk-fg, #555753);
    cursor: pointer;
    user-select: none;
  }
</style>
