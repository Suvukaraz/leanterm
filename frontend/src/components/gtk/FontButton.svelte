<script lang="ts">
  // GtkFontButton look-alike. The value has the same shape as the Go FontConfig
  // ({ Family, Size } with Size in points).
  import Dialog from './Dialog.svelte';
  import Button from './Button.svelte';

  interface FontConfig {
    Family: string;
    Size: number;
  }

  interface Props {
    font?: FontConfig | null;
    disabled?: boolean;
    tooltip?: string;
    /** Restrict suggestions to monospace fonts (terminal fonts). */
    monospace?: boolean;
    class?: string;
    onchange?: (font: FontConfig) => void;
  }

  let {
    font = $bindable({ Family: 'Monospace', Size: 11 }),
    disabled = false,
    tooltip = '',
    monospace = false,
    class: className = '',
    onchange,
  }: Props = $props();

  const MONO = ['Monospace', 'DejaVu Sans Mono', 'Ubuntu Mono', 'Noto Sans Mono', 'Liberation Mono', 'Fira Code', 'JetBrains Mono', 'Hack', 'Source Code Pro', 'Cascadia Code', 'Courier New'];
  const SANS = ['Cantarell', 'Ubuntu', 'Noto Sans', 'DejaVu Sans', 'Liberation Sans', 'Inter', 'Roboto'];
  const suggestions = $derived(monospace ? MONO : [...MONO, ...SANS]);
  const listId = `font-list-${Math.random().toString(36).slice(2)}`;

  let showDialog = $state(false);
  let editFamily = $state('');
  let editSize = $state(11);

  const displayLabel = $derived(
    font?.Family ? `${font.Family} ${font.Size || ''}`.trim() : '(Default system font)'
  );

  function previewFamily(f: string): string {
    return !f || /^mono(space)?$/i.test(f) ? 'monospace' : `"${f}", monospace`;
  }

  function openDialog() {
    if (disabled) return;
    editFamily = font?.Family || 'Monospace';
    editSize = font?.Size || 11;
    showDialog = true;
  }

  function applyFont() {
    const size = Math.max(6, Math.min(72, Math.round(Number(editSize) || 11)));
    font = { Family: editFamily.trim(), Size: size };
    showDialog = false;
    onchange?.(font);
  }
</script>

<button
  type="button"
  class={`gtk-fontbutton ${className}`}
  title={tooltip || displayLabel}
  {disabled}
  onclick={openDialog}
>
  <span class="font-label" style={`font-family: ${previewFamily(font?.Family ?? '')};`}>{displayLabel}</span>
</button>

{#if showDialog}
  <Dialog title="Select Font" width={420} open={true} onclose={() => (showDialog = false)}>
    <div class="font-dialog-body">
      <label class="font-row">
        <span>Family</span>
        <input type="text" list={listId} bind:value={editFamily} />
        <datalist id={listId}>
          {#each suggestions as f}
            <option value={f}></option>
          {/each}
        </datalist>
      </label>
      <label class="font-row">
        <span>Size (pt)</span>
        <input type="number" min="6" max="72" bind:value={editSize} class="font-size-input" />
      </label>
      <div class="font-preview" style={`font-family: ${previewFamily(editFamily)}; font-size: ${editSize}pt;`}>
        user@host:~$ ls -la /etc 0123456789
      </div>
    </div>
    {#snippet actions()}
      <Button label="Cancel" onclick={() => (showDialog = false)} />
      <Button label="Select" isDefault={true} onclick={applyFont} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .gtk-fontbutton {
    display: inline-flex;
    align-items: center;
    min-width: 180px;
    justify-content: flex-start;
  }
  .font-label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .font-dialog-body {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .font-row {
    display: grid;
    grid-template-columns: 90px 1fr;
    align-items: center;
    gap: 8px;
  }
  .font-size-input {
    width: 80px;
  }
  .font-preview {
    border: 1px solid var(--gtk-border-color);
    background: var(--gtk-terminal-bg);
    color: #e6e6e6;
    padding: 10px;
    border-radius: 3px;
    min-height: 48px;
    max-height: 120px;
    overflow: auto;
    user-select: text;
  }
</style>
