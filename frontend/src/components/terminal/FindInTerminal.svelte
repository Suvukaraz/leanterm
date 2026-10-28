<script lang="ts">
  import type { TerminalSearch } from '../../lib/terminal/engine';
  import Entry from '../gtk/Entry.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    open: boolean;
    search?: TerminalSearch;
    onclose: () => void;
  }

  let { open = $bindable(false), search, onclose }: Props = $props();

  let searchText = $state('');
  let caseSensitive = $state(false);
  let wholeWord = $state(false);
  let regex = $state(false);

  function findNext() {
    if (!search || !searchText) return;
    search.findNext(searchText, { caseSensitive, wholeWord, regex });
  }

  function findPrevious() {
    if (!search || !searchText) return;
    search.findPrevious(searchText, { caseSensitive, wholeWord, regex });
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (e.shiftKey) findPrevious();
      else findNext();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onclose();
    }
  }
</script>

{#if open}
  <!-- Enter/Escape are handled for all controls in the panel (entry, checkboxes, buttons). -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="find-terminal-panel" role="search" aria-label="Find in terminal" onkeydown={handleKeyDown}>
    <div class="find-row-input">
      <Entry
        value={searchText}
        placeholder="Find in terminal..."
        onchange={(val) => {
          searchText = val;
          findNext();
        }}
      />
      <Button icon="gtk-media-previous" tooltip="Previous match (Shift+Enter)" onclick={findPrevious} />
      <Button icon="gtk-media-next" tooltip="Next match (Enter)" onclick={findNext} />
      <Button icon="gtk-close" tooltip="Close find bar" onclick={onclose} />
    </div>
    <div class="find-options-row">
      <CheckButton label="Match case" checked={caseSensitive} onchange={(v) => (caseSensitive = v)} />
      <CheckButton label="Whole word" checked={wholeWord} onchange={(v) => (wholeWord = v)} />
      <CheckButton label="Regular expression" checked={regex} onchange={(v) => (regex = v)} />
    </div>
  </div>
{/if}

<style>
  .find-terminal-panel {
    position: absolute;
    top: 4px;
    right: 20px;
    z-index: 50;
    background: var(--gtk-bg-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 4px;
    padding: 6px 8px;
    box-shadow: 0 3px 8px rgba(0, 0, 0, 0.25);
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 11px;
    user-select: none;
  }
  .find-row-input {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .find-options-row {
    display: flex;
    gap: 12px;
  }
</style>
