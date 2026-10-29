<script lang="ts" module>
  // Stack of open dialogs so that Escape only closes the topmost one.
  let nextId = 1;
  const openStack: number[] = [];
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import { get } from 'svelte/store';
  import { dialogGeometries } from '../../lib/stores/ui';
  import {
    RESIZE_EDGES,
    centerRect,
    clampRect,
    moveRect,
    resizeRect,
    type Rect,
    type ResizeEdge,
  } from '../../lib/dialogGeometry';

  interface Props {
    title?: string;
    bannerText?: string;
    width?: number;
    height?: number;
    open?: boolean;
    class?: string;
    /** Whether Escape / the × button may close the dialog. */
    closable?: boolean;
    /** Drag the title bar to move the dialog inside the main window. */
    movable?: boolean;
    /** Drag the edges/corners to resize the dialog. */
    resizable?: boolean;
    minWidth?: number;
    minHeight?: number;
    /** Remembers position and size under this name (state/ui.toml). */
    geometryKey?: string;
    children?: Snippet;
    actions?: Snippet;
    onclose?: () => void;
  }

  let {
    title = '',
    bannerText = '',
    width = 500,
    height = 0,
    open = $bindable(true),
    class: className = '',
    closable = true,
    movable = false,
    resizable = false,
    minWidth = 320,
    minHeight = 200,
    geometryKey = '',
    children,
    actions,
    onclose,
  }: Props = $props();

  const id = nextId++;
  let windowEl = $state<HTMLElement>();
  let backdropEl = $state<HTMLElement>();

  // Explicit geometry once the dialog is movable/resizable; null = CSS-centred.
  let rect = $state<Rect | null>(null);
  let free = $derived(movable || resizable);
  let minSize = $derived({ w: minWidth, h: minHeight });

  function viewport() {
    return { w: backdropEl?.clientWidth || window.innerWidth, h: backdropEl?.clientHeight || window.innerHeight };
  }

  /** Takes the saved geometry, otherwise the size the dialog has when centred by CSS. */
  function initGeometry() {
    const saved = geometryKey ? get(dialogGeometries)[geometryKey] : undefined;
    if (saved && saved.width > 0 && saved.height > 0) {
      rect = clampRect({ x: saved.x, y: saved.y, w: saved.width, h: saved.height }, viewport(), minSize);
      return;
    }
    const box = windowEl?.getBoundingClientRect();
    if (!box) return;
    rect = centerRect({ w: box.width, h: box.height }, viewport(), minSize);
  }

  function saveGeometry() {
    if (!geometryKey || !rect) return;
    const r = rect;
    dialogGeometries.update((m) => ({ ...m, [geometryKey]: { x: r.x, y: r.y, width: r.w, height: r.h } }));
  }

  /** Double-click on the title bar: back to the default size, centred. */
  function resetGeometry() {
    if (!free) return;
    if (geometryKey) {
      dialogGeometries.update((m) => {
        const { [geometryKey]: _removed, ...rest } = m;
        return rest;
      });
    }
    rect = null;
    requestAnimationFrame(() => initGeometry());
  }

  $effect(() => {
    if (!open || !free) return;
    // After the first layout, so an auto-height dialog can be measured.
    const frame = requestAnimationFrame(() => initGeometry());
    return () => {
      cancelAnimationFrame(frame);
      rect = null;
    };
  });

  // ---- Pointer drag (move and resize) ------------------------------------
  let drag: { kind: 'move' | ResizeEdge; startX: number; startY: number; start: Rect } | null = null;

  function startDrag(e: PointerEvent, kind: 'move' | ResizeEdge) {
    if (e.button !== 0 || !rect) return;
    if (kind === 'move' && (e.target as HTMLElement).closest('button')) return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    drag = { kind, startX: e.clientX, startY: e.clientY, start: rect };
  }

  function onDrag(e: PointerEvent) {
    if (!drag) return;
    const dx = e.clientX - drag.startX;
    const dy = e.clientY - drag.startY;
    rect =
      drag.kind === 'move'
        ? moveRect(drag.start, dx, dy, viewport(), minSize)
        : resizeRect(drag.start, drag.kind, dx, dy, viewport(), minSize);
  }

  function endDrag() {
    if (!drag) return;
    drag = null;
    saveGeometry();
  }

  function handleWindowResize() {
    if (rect) rect = clampRect(rect, viewport(), minSize);
  }

  $effect(() => {
    if (!open) return;
    openStack.push(id);
    // Move focus into the dialog (first input, otherwise the window) for keyboard use.
    queueMicrotask(() => {
      const target = windowEl?.querySelector<HTMLElement>(
        'input:not([type=hidden]):not([disabled]), select, textarea'
      );
      (target ?? windowEl)?.focus({ preventScroll: true });
    });
    return () => {
      const idx = openStack.lastIndexOf(id);
      if (idx >= 0) openStack.splice(idx, 1);
    };
  });

  function handleKeyDown(e: KeyboardEvent) {
    if (!open || e.key !== 'Escape' || !closable) return;
    if (openStack[openStack.length - 1] !== id) return;
    e.preventDefault();
    e.stopPropagation();
    closeDialog();
  }

  function closeDialog() {
    open = false;
    onclose?.();
  }
</script>

<svelte:window onkeydown={handleKeyDown} onresize={handleWindowResize} />

{#if open}
  <!-- Modal like a GTK dialog: clicks outside do nothing (they used to discard edits). -->
  <div class="gtk-dialog-backdrop" bind:this={backdropEl}>
    <div
      bind:this={windowEl}
      class={`gtk-dialog-window ${className}`}
      class:is-free={rect !== null}
      style={rect
        ? `left: ${rect.x}px; top: ${rect.y}px; width: ${rect.w}px; height: ${rect.h}px;`
        : `width: min(${width}px, 96vw); ${height ? `height: min(${height}px, 94vh);` : ''}`}
      role="dialog"
      aria-modal="true"
      aria-label={bannerText || title}
      tabindex="-1"
    >
      {#if bannerText}
        <!-- Mouse-only drag handle (moving has no keyboard equivalent, like a window title bar). -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="banner-fill"
          class:drag-handle={movable}
          onpointerdown={movable ? (e) => startDrag(e, 'move') : undefined}
          onpointermove={onDrag}
          onpointerup={endDrag}
          onpointercancel={endDrag}
          ondblclick={movable ? resetGeometry : undefined}
        >
          <span class="banner-text">{bannerText}</span>
          {#if closable}
            <button type="button" class="banner-close button-cp" title="Close" onclick={closeDialog}>×</button>
          {/if}
        </div>
      {:else if title}
        <!-- Mouse-only drag handle (moving has no keyboard equivalent, like a window title bar). -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="gtk-dialog-titlebar"
          class:drag-handle={movable}
          onpointerdown={movable ? (e) => startDrag(e, 'move') : undefined}
          onpointermove={onDrag}
          onpointerup={endDrag}
          onpointercancel={endDrag}
          ondblclick={movable ? resetGeometry : undefined}
        >
          <span class="dialog-title"><b>{title}</b></span>
          {#if closable}
            <button type="button" class="close-btn button-cp" title="Close" onclick={closeDialog}>×</button>
          {/if}
        </div>
      {/if}

      <div class="gtk-dialog-body">
        {@render children?.()}
      </div>

      {#if actions}
        <div class="gtk-dialog-actions">
          {@render actions()}
        </div>
      {/if}

      {#if resizable && rect}
        {#each RESIZE_EDGES as edge (edge)}
          <div
            class={`resize-handle edge-${edge}`}
            aria-hidden="true"
            onpointerdown={(e) => startDrag(e, edge)}
            onpointermove={onDrag}
            onpointerup={endDrag}
            onpointercancel={endDrag}
          ></div>
        {/each}
      {/if}
    </div>
  </div>
{/if}

<style>
  .gtk-dialog-backdrop {
    position: fixed;
    inset: 0;
    background: var(--gtk-backdrop);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100000;
  }
  .gtk-dialog-window {
    background-color: var(--gtk-bg-color);
    color: var(--gtk-fg-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 6px;
    box-shadow: 0 10px 32px var(--gtk-shadow);
    display: flex;
    flex-direction: column;
    max-height: 94vh;
    overflow: hidden;
    outline: none;
    position: relative;
  }
  .gtk-dialog-window.is-free {
    position: absolute;
    max-height: none;
  }
  .drag-handle {
    user-select: none;
    touch-action: none;
  }
  .resize-handle {
    position: absolute;
    z-index: 2;
    touch-action: none;
  }
  .edge-n,
  .edge-s {
    left: 8px;
    right: 8px;
    height: 5px;
    cursor: ns-resize;
  }
  .edge-n {
    top: 0;
  }
  .edge-s {
    bottom: 0;
  }
  .edge-e,
  .edge-w {
    top: 8px;
    bottom: 8px;
    width: 5px;
    cursor: ew-resize;
  }
  .edge-e {
    right: 0;
  }
  .edge-w {
    left: 0;
  }
  .edge-ne,
  .edge-nw,
  .edge-se,
  .edge-sw {
    width: 10px;
    height: 10px;
  }
  .edge-ne {
    top: 0;
    right: 0;
    cursor: nesw-resize;
  }
  .edge-sw {
    bottom: 0;
    left: 0;
    cursor: nesw-resize;
  }
  .edge-nw {
    top: 0;
    left: 0;
    cursor: nwse-resize;
  }
  .edge-se {
    bottom: 0;
    right: 0;
    cursor: nwse-resize;
  }
  .banner-fill {
    position: relative;
  }
  .banner-close {
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%);
    color: #eee !important;
    font-size: 1.4em;
    line-height: 1;
    padding: 0 6px !important;
  }
  .gtk-dialog-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    background-color: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color);
    font-size: 0.95em;
    flex-shrink: 0;
  }
  .close-btn {
    font-size: 1.2em;
    padding: 0 6px !important;
    line-height: 1;
  }
  .gtk-dialog-body {
    padding: 12px 16px;
    overflow-y: auto;
    flex: 1;
    min-height: 0;
  }
  .gtk-dialog-actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    border-top: 1px solid var(--gtk-border-color);
    background-color: var(--gtk-header-bg);
    flex-shrink: 0;
  }
</style>
