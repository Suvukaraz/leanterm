<script lang="ts">
  import { baseName } from '../../lib/fspath';
  import { getIcon } from '../../lib/theme/icons';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  interface Props {
    path?: string;
    mode?: 'file' | 'folder';
    title?: string;
    disabled?: boolean;
    tooltip?: string;
    showClear?: boolean;
    class?: string;
    onselect?: (path: string) => void;
    onclear?: () => void;
  }

  let {
    path = $bindable(''),
    mode = 'file',
    title = 'Choose File',
    disabled = false,
    tooltip = '',
    showClear = false,
    class: className = '',
    onselect,
    onclear
  }: Props = $props();

  const iconName = $derived(mode === 'folder' ? 'gtk-directory' : 'gtk-file');
  const displayPath = $derived(path ? baseName(path) || path : '(None)');

  let busy = false;

  // Native GTK file chooser via the backend (a browser <input type=file> would not
  // reveal the full path).
  async function chooseFile() {
    if (disabled || busy) return;
    busy = true;
    try {
      const selected = await SystemService.ChooseFile(title, path || '', mode === 'folder');
      if (selected) {
        path = selected;
        onselect?.(selected);
      }
    } catch (err) {
      console.warn('File chooser failed:', err);
    } finally {
      busy = false;
    }
  }

  function clear() {
    path = '';
    if (onclear) onclear();
  }
</script>

<div class={`gtk-filechooser-row ${className}`} title={tooltip || path}>
  <button
    type="button"
    class="gtk-filechooser"
    {disabled}
    onclick={chooseFile}
  >
    <img src={getIcon(iconName)} alt="" width="16" height="16" />
    <span class="file-path-text">{displayPath}</span>
  </button>

  {#if showClear && path}
    <button
      type="button"
      class="clear-btn"
      title="Clear selection"
      {disabled}
      onclick={clear}
    >
      Clear
    </button>
  {/if}
</div>

<style>
  .gtk-filechooser-row {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    max-width: 100%;
  }
  .gtk-filechooser {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 6px;
    cursor: pointer;
    max-width: 280px;
    overflow: hidden;
  }
  .file-path-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.9em;
  }
  .clear-btn {
    padding: 2px 5px;
    font-size: 0.85em;
  }
</style>
