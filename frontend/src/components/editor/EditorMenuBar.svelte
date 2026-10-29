<script lang="ts">
  // KWrite-like menu bar. While one menu is open, hovering another title switches to it.
  import ContextMenu, { type MenuItem } from '../gtk/ContextMenu.svelte';

  export interface MenuDef {
    label: string;
    items: MenuItem[];
  }

  interface Props {
    menus: MenuDef[];
  }

  let { menus }: Props = $props();

  let openIndex = $state<number | null>(null);
  let anchorX = $state(0);
  let anchorY = $state(0);
  // The menu closes on pointerdown outside it; a click on the same title right
  // after that must not open it again.
  let closedAt = 0;
  let closedIndex: number | null = null;

  function openAt(index: number, target: HTMLElement) {
    const r = target.getBoundingClientRect();
    anchorX = r.left;
    anchorY = r.bottom;
    openIndex = index;
  }

  function handleClick(index: number, e: MouseEvent) {
    if (closedIndex === index && performance.now() - closedAt < 300) return;
    if (openIndex === index) {
      openIndex = null;
      return;
    }
    openAt(index, e.currentTarget as HTMLElement);
  }

  function handleEnter(index: number, e: MouseEvent) {
    if (openIndex !== null && openIndex !== index) openAt(index, e.currentTarget as HTMLElement);
  }

  function close() {
    closedIndex = openIndex;
    closedAt = performance.now();
    openIndex = null;
  }
</script>

<!-- Like a GTK menu, an open menu closes when the window loses the focus. -->
<svelte:window onblur={() => openIndex !== null && close()} />

<div class="editor-menubar" role="menubar">
  {#each menus as menu, i (menu.label)}
    <button
      type="button"
      class="menubar-item"
      class:open={openIndex === i}
      role="menuitem"
      aria-haspopup="true"
      aria-expanded={openIndex === i}
      onclick={(e) => handleClick(i, e)}
      onmouseenter={(e) => handleEnter(i, e)}
    >
      {menu.label}
    </button>
  {/each}
</div>

{#if openIndex !== null}
  {#key openIndex}
    <ContextMenu items={menus[openIndex].items} x={anchorX} y={anchorY} onclose={close} />
  {/key}
{/if}

<style>
  .editor-menubar {
    display: flex;
    align-items: center;
    gap: 1px;
    padding: 2px 4px;
    background: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color);
    flex-shrink: 0;
    user-select: none;
  }
  .menubar-item {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 3px 9px;
    color: var(--gtk-fg-color);
    font-size: inherit;
  }
  .menubar-item:hover:not(:disabled),
  .menubar-item.open {
    background: var(--gtk-row-hover);
    border-color: var(--gtk-border-color);
  }
</style>
