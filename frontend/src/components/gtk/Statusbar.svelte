<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    class?: string;
    children?: Snippet;
  }

  let { class: className = '', children }: Props = $props();
</script>

<div class={`gtk-statusbar ${className}`} role="status">
  {@render children?.()}
</div>

<style>
  .gtk-statusbar {
    display: flex;
    align-items: center;
    padding: 2px 8px;
    background-color: var(--gtk-header-bg);
    border-top: 1px solid var(--gtk-border-color);
    font-size: 0.85em;
    min-height: 22px;
    user-select: none;
    box-sizing: border-box;
    gap: 8px;
  }
</style>
