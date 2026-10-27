<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';
  import Button from '../gtk/Button.svelte';

  interface Props {
    title?: string;
    tooltip?: string;
    items: T[];
    createItem: () => T;
    addLabel?: string;
    canReorder?: boolean;
    disabled?: boolean;
    itemSnippet: Snippet<[T, number, (idx: number) => void, (idx: number) => void, (idx: number) => void]>;
  }

  let {
    title = '',
    tooltip = '',
    items = $bindable([]),
    createItem,
    addLabel = 'Add',
    canReorder = false,
    disabled = false,
    itemSnippet
  }: Props = $props();

  // Undo stack
  let history: string[] = [];
  let isUndoing = false;

  function pushHistory() {
    if (isUndoing) return;
    history.push(JSON.stringify(items));
    if (history.length > 50) history.shift();
  }

  function handleUndo(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
      if (history.length > 0) {
        e.preventDefault();
        e.stopPropagation();
        isUndoing = true;
        const prev = history.pop();
        if (prev) {
          items = JSON.parse(prev);
        }
        isUndoing = false;
      }
    }
  }

  function addItem() {
    pushHistory();
    items = [...items, createItem()];
  }

  function removeItem(index: number) {
    pushHistory();
    items = items.filter((_, i) => i !== index);
  }

  function moveUp(index: number) {
    if (index <= 0) return;
    pushHistory();
    const updated = [...items];
    const temp = updated[index];
    updated[index] = updated[index - 1];
    updated[index - 1] = temp;
    items = updated;
  }

  function moveDown(index: number) {
    if (index >= items.length - 1) return;
    pushHistory();
    const updated = [...items];
    const temp = updated[index];
    updated[index] = updated[index + 1];
    updated[index + 1] = temp;
    items = updated;
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="editable-list" title={tooltip} onkeydown={handleUndo}>
  {#if title}
    <div class="list-header">
      <span class="list-title"><b>{title}</b></span>
      <span class="undo-hint">(Ctrl+Z to undo)</span>
    </div>
  {/if}

  <div class="items-container">
    {#if items.length === 0}
      <div class="empty-list-hint">No entries defined. Click "{addLabel}" below to add one.</div>
    {:else}
      {#each items as item, i}
        <div class="item-row">
          {@render itemSnippet(item, i, removeItem, moveUp, moveDown)}
        </div>
      {/each}
    {/if}
  </div>

  <div class="list-actions">
    <Button
      label={addLabel}
      icon="gtk-add"
      {disabled}
      onclick={addItem}
    />
  </div>
</div>

<style>
  .editable-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 100%;
  }

  .list-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 2px;
  }

  .list-title {
    font-size: 0.95em;
    color: var(--gtk-fg-color);
  }

  .undo-hint {
    font-size: 0.8em;
    color: var(--gtk-muted-fg);
  }

  .items-container {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 320px;
    overflow-y: auto;
    padding: 2px;
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    background: var(--gtk-base-color, #ffffff);
  }

  .empty-list-hint {
    padding: 16px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
    font-size: 0.9em;
  }

  .item-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .item-row:last-child {
    border-bottom: none;
  }

  .list-actions {
    display: flex;
    justify-content: flex-start;
    padding-top: 4px;
  }
</style>
