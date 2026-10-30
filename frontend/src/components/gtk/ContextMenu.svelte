<script lang="ts" module>
  // Keyboard handlers of the open menu levels (deepest last). The root menu owns a
  // single window listener and forwards keys to the deepest level, so menus never
  // take focus away from the terminal (which would make it repaint).
  const keyHandlers: ((e: KeyboardEvent) => void)[] = [];
</script>

<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { getIcon } from '../../lib/theme/icons';
  import ContextMenu from './ContextMenu.svelte';

  export interface MenuItem {
    id?: string;
    label?: string;
    icon?: string;
    theme?: string;
    shortcut?: string;
    disabled?: boolean;
    separator?: boolean;
    submenu?: MenuItem[];
    checked?: boolean;
    tooltip?: string;
    action?: () => void;
  }

  interface Props {
    items: MenuItem[];
    x: number;
    y: number;
    onclose: () => void;
    /** Set for submenus: the parent row's rectangle, used for placement. */
    anchor?: DOMRect | null;
    /** Submenus only: ArrowLeft returns to the parent menu. */
    onback?: () => void;
  }

  let { items, x, y, onclose, anchor = null, onback }: Props = $props();

  // Position props are read once: a menu does not move while it is open.
  const isRoot = untrack(() => anchor === null);

  let activeSubmenu = $state<number | null>(null);
  let submenuAnchor = $state<DOMRect | null>(null);
  let focusIndex = $state(-1);
  let menuEl: HTMLElement;
  let hoverTimer: ReturnType<typeof setTimeout> | undefined;

  // Initial position; corrected synchronously in onMount (before the first paint)
  // so the menu never appears at a wrong place or jumps.
  const initialLeft = untrack(() => (anchor ? anchor.right - 2 : x));
  const initialTop = untrack(() => (anchor ? anchor.top - 4 : y));

  function place() {
    if (!menuEl) return;
    const w = menuEl.offsetWidth;
    const h = menuEl.offsetHeight;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    let left = initialLeft;
    let top = initialTop;
    const a = untrack(() => anchor);
    if (a) {
      if (left + w > vw - 4) left = Math.max(4, a.left - w + 2);
    } else if (left + w > vw - 4) {
      left = Math.max(4, vw - w - 4);
    }
    if (top + h > vh - 4) top = Math.max(4, vh - h - 4);
    if (left !== initialLeft) menuEl.style.left = `${left}px`;
    if (top !== initialTop) menuEl.style.top = `${top}px`;
  }

  onMount(() => {
    place();
    keyHandlers.push(handleMenuKey);
    const removeKeyHandler = () => {
      const idx = keyHandlers.lastIndexOf(handleMenuKey);
      if (idx >= 0) keyHandlers.splice(idx, 1);
    };
    if (!isRoot) return removeKeyHandler;

    // Only the root menu listens globally; clicks anywhere inside the menu tree
    // (root or any submenu) must not close it.
    function handlePointerDown(e: PointerEvent) {
      const target = e.target as HTMLElement | null;
      if (target?.closest?.('.gtk-menu')) return;
      onclose();
    }
    function handleKeyDown(e: KeyboardEvent) {
      // While a menu is open it owns the keyboard (like a GTK menu grab).
      e.preventDefault();
      e.stopPropagation();
      if (e.key === 'Escape') {
        onclose();
        return;
      }
      keyHandlers[keyHandlers.length - 1]?.(e);
    }
    function handleBlur() {
      onclose();
    }
    window.addEventListener('pointerdown', handlePointerDown, true);
    window.addEventListener('keydown', handleKeyDown, true);
    window.addEventListener('blur', handleBlur);
    window.addEventListener('resize', handleBlur);
    return () => {
      removeKeyHandler();
      window.removeEventListener('pointerdown', handlePointerDown, true);
      window.removeEventListener('keydown', handleKeyDown, true);
      window.removeEventListener('blur', handleBlur);
      window.removeEventListener('resize', handleBlur);
      clearTimeout(hoverTimer);
    };
  });

  function openSubmenu(i: number, row: HTMLElement) {
    clearTimeout(hoverTimer);
    submenuAnchor = row.getBoundingClientRect();
    activeSubmenu = i;
  }

  function handleEnter(i: number, item: MenuItem, e: MouseEvent) {
    focusIndex = i;
    clearTimeout(hoverTimer);
    const row = e.currentTarget as HTMLElement;
    // Small delay so moving diagonally towards an open submenu does not close it.
    hoverTimer = setTimeout(() => {
      if (item.submenu && !item.disabled) openSubmenu(i, row);
      else activeSubmenu = null;
    }, item.submenu ? 80 : 150);
  }

  function clickItem(item: MenuItem, i: number, e: MouseEvent) {
    if (item.disabled || item.separator) return;
    if (item.submenu) {
      openSubmenu(i, e.currentTarget as HTMLElement);
      return;
    }
    onclose();
    item.action?.();
  }

  function handleMenuKey(e: KeyboardEvent) {
    const selectable = items.map((it, i) => ({ it, i })).filter(({ it }) => !it.separator && !it.disabled);
    if (selectable.length === 0) return;
    const pos = selectable.findIndex(({ i }) => i === focusIndex);
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const delta = e.key === 'ArrowDown' ? 1 : -1;
      const next =
        pos < 0 ? (delta > 0 ? 0 : selectable.length - 1) : (pos + delta + selectable.length) % selectable.length;
      focusIndex = selectable[next].i;
      activeSubmenu = null;
    } else if ((e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowRight') && focusIndex >= 0) {
      const item = items[focusIndex];
      const row = menuEl?.querySelector<HTMLElement>(`[data-index="${focusIndex}"]`);
      if (!row) return;
      if (item.submenu) openSubmenu(focusIndex, row);
      else if (e.key !== 'ArrowRight') {
        onclose();
        item.action?.();
      }
    } else if (e.key === 'ArrowLeft' && !isRoot) {
      onback?.();
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions, a11y_click_events_have_key_events -->
<div
  bind:this={menuEl}
  class="gtk-menu"
  style={`left: ${initialLeft}px; top: ${initialTop}px;`}
  role="menu"
  tabindex="-1"
  onmousedown={(e) => e.preventDefault()}
  oncontextmenu={(e) => e.preventDefault()}
>
  {#each items as item, i}
    {#if item.separator}
      <div class="menu-separator" role="separator"></div>
    {:else}
      <div
        class="menu-item"
        class:disabled={item.disabled}
        class:focused={focusIndex === i}
        class:has-active-sub={activeSubmenu === i}
        data-index={i}
        title={item.tooltip || ''}
        role="menuitem"
        tabindex="-1"
        aria-disabled={item.disabled}
        aria-haspopup={item.submenu ? 'menu' : undefined}
        onclick={(e) => clickItem(item, i, e)}
        onmouseenter={(e) => handleEnter(i, item, e)}
      >
        <span class="icon-col">
          {#if item.icon}
            <img src={getIcon(item.icon, item.theme)} alt="" width="16" height="16" />
          {:else if item.checked !== undefined}
            <span class="check-mark">{item.checked ? '✓' : ''}</span>
          {/if}
        </span>

        <span class="label-col">{item.label}</span>

        <span class="meta-col">
          {#if item.shortcut}
            <span class="shortcut-text">{item.shortcut}</span>
          {/if}
          {#if item.submenu}
            <span class="arrow-right">▸</span>
          {/if}
        </span>
      </div>
    {/if}
  {/each}
</div>

{#if activeSubmenu !== null && items[activeSubmenu]?.submenu && submenuAnchor}
  {#key activeSubmenu}
    <ContextMenu
      items={items[activeSubmenu].submenu ?? []}
      x={0}
      y={0}
      anchor={submenuAnchor}
      {onclose}
      onback={() => {
        activeSubmenu = null;
      }}
    />
  {/key}
{/if}

<style>
  .gtk-menu {
    position: fixed;
    z-index: 99999;
    background-color: var(--gtk-menu-bg);
    border: 1px solid var(--gtk-border-color);
    box-shadow: 0 6px 18px var(--gtk-shadow);
    border-radius: 5px;
    padding: 4px 0;
    min-width: 200px;
    max-width: 420px;
    max-height: calc(100vh - 8px);
    overflow-y: auto;
    font-size: 0.95em;
    user-select: none;
    outline: none;
  }

  .menu-separator {
    height: 1px;
    background-color: var(--gtk-border-color);
    margin: 4px 6px;
  }

  .menu-item {
    display: flex;
    align-items: center;
    padding: 4px 10px;
    cursor: default;
    color: var(--gtk-fg-color);
    gap: 8px;
    outline: none;
  }

  .menu-item:hover:not(.disabled),
  .menu-item.focused:not(.disabled),
  .menu-item.has-active-sub {
    background-color: var(--gtk-selected-bg);
    color: var(--gtk-selected-fg);
  }

  .menu-item.disabled {
    opacity: 0.45;
  }

  .icon-col {
    width: 16px;
    height: 16px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .check-mark {
    font-weight: bold;
  }

  .label-col {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .meta-col {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-left: 12px;
  }

  .shortcut-text {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
  }

  .menu-item:hover:not(.disabled) .shortcut-text,
  .menu-item.focused .shortcut-text {
    color: inherit;
    opacity: 0.8;
  }

  .arrow-right {
    font-size: 0.9em;
  }
</style>
