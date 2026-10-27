<script lang="ts">
  interface Option {
    value: string;
    label: string;
    tooltip?: string;
    disabled?: boolean;
  }

  interface Props {
    name: string;
    options: Option[];
    value?: string;
    disabled?: boolean;
    class?: string;
    direction?: 'horizontal' | 'vertical';
    onchange?: (value: string) => void;
  }

  let {
    name,
    options = [],
    value = $bindable(''),
    disabled = false,
    class: className = '',
    direction = 'horizontal',
    onchange
  }: Props = $props();

  function selectOption(val: string) {
    value = val;
    if (onchange) onchange(val);
  }
</script>

<div class={`gtk-radiogroup direction-${direction} ${className}`}>
  {#each options as opt}
    <label class="gtk-radio-item" title={opt.tooltip || ''}>
      <input
        type="radio"
        {name}
        value={opt.value}
        checked={value === opt.value}
        disabled={disabled || opt.disabled}
        onchange={() => selectOption(opt.value)}
      />
      <span class="label-text">{opt.label}</span>
    </label>
  {/each}
</div>

<style>
  .gtk-radiogroup {
    display: inline-flex;
    gap: 8px;
    user-select: none;
  }
  .direction-vertical {
    flex-direction: column;
    gap: 4px;
  }
  .direction-horizontal {
    flex-direction: row;
    align-items: center;
  }
  .gtk-radio-item {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
  }
  .label-text {
    line-height: 1.2;
  }
</style>
