<script lang="ts">
  // KWrite's Quick Open (Ctrl+Alt+O): a filter field over the open documents.
  // Up/Down choose, Enter opens, Escape closes.
  import { onMount } from 'svelte';

  export interface QuickOpenItem {
    id: number;
    title: string;
    detail: string;
  }

  interface Props {
    items: QuickOpenItem[];
    onchoose: (id: number) => void;
    onclose: () => void;
  }

  let { items, onchoose, onclose }: Props = $props();

  let query = $state('');
  let index = $state(0);
  let inputEl = $state<HTMLInputElement>();
  let listEl = $state<HTMLElement>();

  /** Fuzzy match: all query characters in order; earlier and denser matches rank higher. */
  function score(text: string, q: string): number {
    if (!q) return 0;
    const t = text.toLowerCase();
    const direct = t.indexOf(q);
    if (direct >= 0) return direct;
    let pos = -1;
    let gaps = 0;
    for (const ch of q) {
      const next = t.indexOf(ch, pos + 1);
      if (next < 0) return -1;
      gaps += next - pos - 1;
      pos = next;
    }
    return 1000 + gaps;
  }

  let shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return items
      .map((it) => {
        const s = score(it.title, q);
        const d = s < 0 ? score(it.detail, q) : s;
        return { it, s: s >= 0 ? s : d >= 0 ? 2000 + d : -1 };
      })
      .filter((x) => x.s >= 0)
      .sort((a, b) => a.s - b.s)
      .map((x) => x.it);
  });

  $effect(() => {
    void query;
    index = 0;
  });

  onMount(() => inputEl?.focus());

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
    else if (e.key === 'ArrowDown') index = Math.min(shown.length - 1, index + 1);
    else if (e.key === 'ArrowUp') index = Math.max(0, index - 1);
    else if (e.key === 'Enter') {
      const it = shown[index];
      if (it) onchoose(it.id);
    } else return;
    e.preventDefault();
    e.stopPropagation();
    queueMicrotask(() => listEl?.querySelector('.active')?.scrollIntoView({ block: 'nearest' }));
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="quick-open-backdrop" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="quick-open" onclick={(e) => e.stopPropagation()}>
    <input
      bind:this={inputEl}
      bind:value={query}
      type="text"
      placeholder="Quick Open: type to filter open documents"
      spellcheck="false"
      onkeydown={onKeyDown}
      onblur={() => setTimeout(() => document.activeElement !== inputEl && onclose(), 0)}
    />
    <div class="qo-list" bind:this={listEl} role="listbox" aria-label="Open documents">
      {#each shown as it, i (it.id)}
        <div
          class="qo-item"
          class:active={i === index}
          role="option"
          tabindex="-1"
          aria-selected={i === index}
          onmousedown={(e) => {
            e.preventDefault();
            onchoose(it.id);
          }}
        >
          <span class="qo-title">{it.title}</span>
          <span class="qo-detail">{it.detail}</span>
        </div>
      {:else}
        <div class="qo-empty">No matching document</div>
      {/each}
    </div>
  </div>
</div>

<style>
  .quick-open-backdrop {
    position: fixed;
    inset: 0;
    z-index: 900;
  }

  .quick-open {
    position: absolute;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    width: min(620px, 90vw);
    background: var(--gtk-menu-bg, var(--gtk-bg-color));
    color: var(--gtk-fg-color);
    border: 1px solid var(--gtk-border-color);
    border-radius: 6px;
    box-shadow: 0 10px 32px var(--gtk-shadow);
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  input {
    width: 100%;
    box-sizing: border-box;
  }

  .qo-list {
    max-height: 320px;
    overflow-y: auto;
  }

  .qo-item {
    display: flex;
    gap: 10px;
    align-items: baseline;
    padding: 4px 8px;
    border-radius: 3px;
    cursor: default;
  }

  .qo-item.active {
    background: var(--gtk-selected-bg);
    color: var(--gtk-selected-fg);
  }

  .qo-title {
    white-space: nowrap;
  }

  .qo-detail {
    font-size: 0.85em;
    opacity: 0.7;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    direction: rtl;
    text-align: left;
  }

  .qo-empty {
    padding: 6px 8px;
    color: var(--gtk-muted-fg);
  }
</style>
