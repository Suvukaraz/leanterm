<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    label?: string;
    class?: string;
    children?: Snippet;
  }

  let { label = '', class: className = '', children }: Props = $props();
</script>

<fieldset class={`gtk-frame ${className}`}>
  {#if label}
    <legend class="gtk-frame-label"><b>{label}</b></legend>
  {/if}
  <div class="gtk-frame-content">
    {@render children?.()}
  </div>
</fieldset>

<style>
  .gtk-frame {
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    padding: 8px 10px;
    margin: 4px 0;
    box-sizing: border-box;
  }
  .gtk-frame-label {
    padding: 0 4px;
    color: var(--gtk-fg-color);
    font-size: 0.95em;
  }
  .gtk-frame-content {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
</style>
