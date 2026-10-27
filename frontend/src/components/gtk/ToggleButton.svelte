<script lang="ts">
  import { getIcon } from '../../lib/theme/icons';

  interface Props {
    label?: string;
    icon?: string;
    theme?: string;
    tooltip?: string;
    shortcut?: string;
    disabled?: boolean;
    active?: boolean;
    checked?: boolean;
    variant?: 'normal' | 'cp';
    class?: string;
    onchange?: (active: boolean) => void;
  }

  let {
    label = '',
    icon = '',
    theme = 'default',
    tooltip = '',
    shortcut = '',
    disabled = false,
    active = $bindable(false),
    checked,
    variant = 'normal',
    class: className = '',
    onchange
  }: Props = $props();

  $effect(() => {
    if (checked !== undefined) {
      active = checked;
    }
  });

  const iconSrc = $derived(icon ? getIcon(icon, theme) : '');
  const fullTooltip = $derived(tooltip ? (shortcut ? `${tooltip} (${shortcut})` : tooltip) : (shortcut ? `(${shortcut})` : ''));

  function toggle() {
    if (disabled) return;
    active = !active;
    if (onchange) onchange(active);
  }
</script>

<button
  type="button"
  class={`${variant === 'cp' ? 'button-cp' : ''} ${active ? 'is-active' : ''} ${className}`}
  title={fullTooltip}
  {disabled}
  onclick={toggle}
>
  {#if iconSrc}
    <img src={iconSrc} alt="" width="16" height="16" class="btn-icon" />
  {/if}
  {#if label}
    <span class="btn-label">{label}</span>
  {/if}
</button>

<style>
  .is-active {
    background-color: var(--gtk-btn-focus-bg) !important;
    border-color: var(--gtk-border-color) !important;
  }
  .btn-icon {
    flex-shrink: 0;
    pointer-events: none;
    object-fit: contain;
  }
  .btn-label {
    white-space: nowrap;
  }
</style>
