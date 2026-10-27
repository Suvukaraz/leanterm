<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    orientation?: 'horizontal' | 'vertical'; // horizontal = side-by-side (col-resize), vertical = stacked (row-resize)
    /** Split position in percent of the first pane (used when no fixed pixel size is given). */
    splitPercent?: number;
    /** Fixed pixel size of the `fixed` pane (like GtkPaned with a non-resizing child). */
    initialSplit?: number;
    /** Which pane keeps its pixel size when the window is resized. */
    fixed?: 'first' | 'second';
    minSizeFirst?: number;
    minFirst?: number;
    minSizeSecond?: number;
    minSecond?: number;
    force50?: boolean;
    class?: string;
    first?: Snippet;
    second?: Snippet;
    onresize?: (percent: number) => void;
    /** Called once when a drag ends, with the pixel size of the fixed pane. */
    onchange?: (pos: number) => void;
  }

  let {
    orientation = 'horizontal',
    splitPercent = $bindable(30),
    initialSplit,
    fixed = 'first',
    minSizeFirst = 150,
    minFirst,
    minSizeSecond = 200,
    minSecond,
    force50 = false,
    class: className = '',
    first,
    second,
    onresize,
    onchange,
  }: Props = $props();

  const effectiveMinFirst = $derived(minFirst ?? minSizeFirst);
  const effectiveMinSecond = $derived(minSecond ?? minSizeSecond);
  const pixelMode = $derived(initialSplit !== undefined && !force50);

  let containerEl: HTMLElement;
  let isDragging = $state(false);
  // Pixel size of the fixed pane while dragging/after a drag (pixel mode only).
  let fixedPx = $state<number | null>(null);
  let rafId = 0;
  let pendingPos: number | null = null;

  const currentPx = $derived(fixedPx ?? initialSplit ?? 250);
  const effectivePercent = $derived(force50 ? 50 : splitPercent);
  const horizontal = $derived(orientation === 'horizontal');
  const sizeProp = $derived(horizontal ? 'width' : 'height');

  function firstStyle(): string {
    if (pixelMode) {
      return fixed === 'first' ? `flex: 0 0 ${currentPx}px;` : 'flex: 1 1 0;';
    }
    return `flex: 0 0 calc(${effectivePercent}% - 3px);`;
  }

  function secondStyle(): string {
    if (pixelMode) {
      return fixed === 'second' ? `flex: 0 0 ${currentPx}px;` : 'flex: 1 1 0;';
    }
    return 'flex: 1 1 0;';
  }

  function startDrag(e: PointerEvent) {
    if (force50 || e.button !== 0) return;
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    isDragging = true;
    document.body.classList.add(horizontal ? 'paned-dragging-col' : 'paned-dragging-row');
  }

  function onPointerMove(e: PointerEvent) {
    if (!isDragging) return;
    pendingPos = horizontal ? e.clientX : e.clientY;
    if (!rafId) rafId = requestAnimationFrame(applyPending);
  }

  function applyPending() {
    rafId = 0;
    if (pendingPos === null || !containerEl) return;
    const rect = containerEl.getBoundingClientRect();
    const total = horizontal ? rect.width : rect.height;
    let pos = pendingPos - (horizontal ? rect.left : rect.top);
    pos = Math.max(effectiveMinFirst, Math.min(total - effectiveMinSecond, pos));
    pendingPos = null;

    if (pixelMode) {
      fixedPx = Math.round(fixed === 'first' ? pos : total - pos);
    } else {
      splitPercent = Math.max(5, Math.min(95, (pos / total) * 100));
      onresize?.(splitPercent);
    }
  }

  function stopDrag(e: PointerEvent) {
    if (!isDragging) return;
    isDragging = false;
    (e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId);
    document.body.classList.remove('paned-dragging-col', 'paned-dragging-row');
    if (pendingPos !== null) applyPending();
    if (pixelMode && fixedPx !== null) {
      onchange?.(fixedPx);
    } else if (containerEl) {
      const rect = containerEl.getBoundingClientRect();
      onchange?.(Math.round(((horizontal ? rect.width : rect.height) * splitPercent) / 100));
    }
  }

  function handleDblClick() {
    if (pixelMode) return;
    splitPercent = 50;
    onresize?.(50);
  }
</script>

<div bind:this={containerEl} class={`gtk-paned orient-${orientation} ${className}`}>
  <div class="paned-first" style={firstStyle()} data-size-prop={sizeProp}>
    {@render first?.()}
  </div>

  <div
    class={`paned-handle ${horizontal ? '' : 'paned-handle-horizontal'} ${isDragging ? 'dragging' : ''}`}
    role="separator"
    aria-orientation={horizontal ? 'vertical' : 'horizontal'}
    onpointerdown={startDrag}
    onpointermove={onPointerMove}
    onpointerup={stopDrag}
    onpointercancel={stopDrag}
    ondblclick={handleDblClick}
  ></div>

  <div class="paned-second" style={secondStyle()}>
    {@render second?.()}
  </div>
</div>

<style>
  .gtk-paned {
    display: flex;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    position: relative;
  }
  .orient-horizontal {
    flex-direction: row;
  }
  .orient-vertical {
    flex-direction: column;
  }
  .paned-first,
  .paned-second {
    overflow: hidden;
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .paned-handle {
    touch-action: none;
  }
  /* While dragging, keep the resize cursor everywhere and stop xterm/iframes from eating events. */
  :global(body.paned-dragging-col),
  :global(body.paned-dragging-col *) {
    cursor: col-resize !important;
    user-select: none !important;
  }
  :global(body.paned-dragging-row),
  :global(body.paned-dragging-row *) {
    cursor: row-resize !important;
    user-select: none !important;
  }
</style>
