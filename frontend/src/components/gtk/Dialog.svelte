<script lang="ts" module>
  // Stack of open dialogs so that Escape only closes the topmost one.
  let nextId = 1;
  const openStack: number[] = [];
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title?: string;
    bannerText?: string;
    width?: number;
    height?: number;
    open?: boolean;
    class?: string;
    /** Whether Escape / the × button may close the dialog. */
    closable?: boolean;
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
    children,
    actions,
    onclose,
  }: Props = $props();

  const id = nextId++;
  let windowEl = $state<HTMLElement>();

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

<svelte:window onkeydown={handleKeyDown} />

{#if open}
  <!-- Modal like a GTK dialog: clicks outside do nothing (they used to discard edits). -->
  <div class="gtk-dialog-backdrop">
    <div
      bind:this={windowEl}
      class={`gtk-dialog-window ${className}`}
      style={`width: min(${width}px, 96vw); ${height ? `height: min(${height}px, 94vh);` : ''}`}
      role="dialog"
      aria-modal="true"
      aria-label={bannerText || title}
      tabindex="-1"
    >
      {#if bannerText}
        <div class="banner-fill">
          <span class="banner-text">{bannerText}</span>
          {#if closable}
            <button type="button" class="banner-close button-cp" title="Close" onclick={closeDialog}>×</button>
          {/if}
        </div>
      {:else if title}
        <div class="gtk-dialog-titlebar">
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
