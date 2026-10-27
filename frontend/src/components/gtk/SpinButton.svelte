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
    max = 65535,
    step = 1,
    disabled = false,
    tooltip = '',
    class: className = '',
    onchange
  }: Props = $props();

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    let num = parseInt(target.value, 10);
    if (isNaN(num)) num = min;
    if (num < min) num = min;
    if (num > max) num = max;
    value = num;
    if (onchange) onchange(num);
  }
</script>

<input
  type="number"
  class={`spinbutton ${className}`}
  bind:value={value}
  {min}
  {max}
  {step}
  {disabled}
  title={tooltip}
  oninput={handleInput}
/>

<style>
  .spinbutton {
    color: var(--gtk-fg-color);
    font-size: 0.9em;
    border: 1px solid var(--gtk-border-color);
    background-color: var(--gtk-header-bg);
    border-radius: 0px;
    padding: 2px 4px;
    margin: 0px;
    width: 70px;
    box-sizing: border-box;
    outline: none;
  }
  .spinbutton:hover:not(:disabled) {
    border: 1px solid var(--gtk-border-color);
    background-color: var(--gtk-base-color);
  }
  .spinbutton:focus {
    background-color: var(--gtk-btn-focus-bg);
  }
</style>
