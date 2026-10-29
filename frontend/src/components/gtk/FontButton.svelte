<script lang="ts">
  // GtkFontButton look-alike. The value has the same shape as the Go FontConfig
  // ({ Family, Size, Style, Features } with Size in points); clicking opens the
  // KDE-style FontChooserDialog.
  import FontChooserDialog from './FontChooserDialog.svelte';
  import { fontFaceCss, fontLabel, type FontSetting } from '../../lib/fonts';

  interface Props {
    font?: FontSetting | null;
    disabled?: boolean;
    tooltip?: string;
    /** Start the chooser with "Show only monospaced fonts" checked (terminal fonts). */
    monospace?: boolean;
    class?: string;
    onchange?: (font: FontSetting) => void;
  }

  let {
    font = $bindable({ Family: 'Monospace', Size: 11 }),
    disabled = false,
    tooltip = '',
    monospace = false,
    class: className = '',
    onchange,
  }: Props = $props();

  let showDialog = $state(false);

  const displayLabel = $derived(fontLabel(font));

  function previewCss(f: FontSetting | null | undefined): string {
    const family = !f?.Family || /^mono(space)?$/i.test(f.Family) ? 'monospace' : `"${f.Family.replace(/"/g, '')}", monospace`;
    return `font-family: ${family}; ${fontFaceCss(f)}`;
  }

  function applyFont(next: Required<FontSetting>) {
    // Keep other keys of the bound object (it is the Go FontConfig model).
    font = { ...(font ?? {}), ...next };
    showDialog = false;
    onchange?.(font);
  }
</script>

<button
  type="button"
  class={`gtk-fontbutton ${className}`}
  title={tooltip || displayLabel}
  {disabled}
  onclick={() => !disabled && (showDialog = true)}
>
  <span class="font-label" style={previewCss(font)}>{displayLabel}</span>
</button>

{#if showDialog}
  <FontChooserDialog {font} {monospace} onselect={applyFont} oncancel={() => (showDialog = false)} />
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
</style>
