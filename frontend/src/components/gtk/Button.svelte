<script lang="ts">
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    label?: string;
    icon?: string;
    theme?: string;
    tooltip?: string;
    shortcut?: string;
    disabled?: boolean;
    isDefault?: boolean;
    class?: string;
    variant?: 'normal' | 'cp' | 'flat';
    onclick?: (e: MouseEvent) => void;
  }

  let {
    label = '',
    icon = '',
    theme = 'default',
    tooltip = '',
    shortcut = '',
    disabled = false,
    isDefault = false,
    class: className = '',
    variant = 'normal',
    onclick
  }: Props = $props();

  const iconSrc = $derived(icon ? getIcon(icon, theme) : '');

  // GTK mnemonics: "_Save and Close" shows "Save and Close" with an underlined S
  // and Alt+S activates the button.
  const mnemonic = $derived.by(() => {
    const idx = label.indexOf('_');
    if (idx < 0 || idx === label.length - 1) return null;
    return { before: label.slice(0, idx), key: label[idx + 1], after: label.slice(idx + 2) };
  });

  let buttonEl = $state<HTMLButtonElement>();

  function handleWindowKey(e: KeyboardEvent) {
    if (!mnemonic || disabled || !e.altKey || e.ctrlKey || e.metaKey) return;
    if (e.key.toLowerCase() !== mnemonic.key.toLowerCase()) return;
    if (!buttonEl || buttonEl.offsetParent === null) return; // not visible
    // Only buttons inside the topmost dialog (or outside dialogs if none is open) react.
    const dialogs = document.querySelectorAll('[role="dialog"]');
    const top = dialogs[dialogs.length - 1];
    if (top && !top.contains(buttonEl)) return;
    e.preventDefault();
    buttonEl.click();
  }
  const fullTooltip = $derived(tooltip ? (shortcut ? `${tooltip} (${shortcut})` : tooltip) : (shortcut ? `(${shortcut})` : ''));
</script>

<svelte:window onkeydown={handleWindowKey} />

<button
  bind:this={buttonEl}
  type="button"
  class={`${variant === 'cp' ? 'button-cp' : ''} ${isDefault ? 'btn-default' : ''} ${className}`}
  title={fullTooltip}
  {disabled}
  {onclick}
>
  {#if iconSrc}
    <img src={iconSrc} alt="" width="16" height="16" class="btn-icon" />
  {/if}
  {#if mnemonic}
    <span class="btn-label">{mnemonic.before}<u>{mnemonic.key}</u>{mnemonic.after}</span>
  {:else if label}
    <span class="btn-label">{label}</span>
  {/if}
</button>

<style>
  .btn-icon {
    flex-shrink: 0;
    pointer-events: none;
    object-fit: contain;
  }
  .btn-label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .btn-default {
    border-color: var(--gtk-accent);
    font-weight: 500;
  }
</style>
