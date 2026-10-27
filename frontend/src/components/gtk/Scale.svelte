<script lang="ts">
  interface Props {
    value?: number;
    min?: number;
    max?: number;
    step?: number;
    disabled?: boolean;
    tooltip?: string;
    class?: string;
    onchange?: (value: number) => void;
  }

  let {
    value = $bindable(0),
    min = 0,
    max = 1,
    step = 0.05,
    disabled = false,
    tooltip = '',
    class: className = '',
    onchange
  }: Props = $props();

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = parseFloat(target.value);
    if (onchange) onchange(value);
  }
</script>

<div class={`gtk-scale-row ${className}`} title={tooltip || `${value}`}>
  <input
    type="range"
    class="gtk-scale"
    {min}
    {max}
    {step}
    bind:value={value}
    {disabled}
    oninput={handleInput}
  />
  <span class="scale-val-label">{Math.round(value * 100)}%</span>
</div>

<style>
  .gtk-scale-row {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }
  .gtk-scale {
    cursor: pointer;
  }
  .scale-val-label {
    font-size: 0.85em;
    min-width: 35px;
  }
</style>
