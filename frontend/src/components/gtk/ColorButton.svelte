<script lang="ts">
  interface Props {
    value?: string; // #rrggbb
    disabled?: boolean;
    tooltip?: string;
    class?: string;
    onchange?: (color: string) => void;
  }

  let {
    value = $bindable('#000000'),
    disabled = false,
    tooltip = '',
    class: className = '',
    onchange
  }: Props = $props();

  let colorInputEl: HTMLInputElement;

  function handleClick() {
    if (disabled || !colorInputEl) return;
    colorInputEl.click();
  }

  function handleColorChange(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    if (onchange) onchange(value);
  }
</script>

<div class={`gtk-colorbutton-wrapper ${className}`}>
  <button
    type="button"
    class="gtk-colorbutton"
    title={tooltip || value}
    {disabled}
    onclick={handleClick}
  >
    <span class="color-swatch" style={`background-color: ${value};`}></span>
    <span class="color-hex">{value}</span>
  </button>
  <input
    bind:this={colorInputEl}
    type="color"
    value={value}
    {disabled}
    class="hidden-color-input"
    onchange={handleColorChange}
  />
</div>

<style>
  .gtk-colorbutton-wrapper {
    display: inline-flex;
    align-items: center;
  }
  .gtk-colorbutton {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 6px;
    cursor: pointer;
  }
  .color-swatch {
    width: 18px;
    height: 14px;
    border: 1px solid var(--gtk-border-color);
    border-radius: 1px;
    display: inline-block;
  }
  .color-hex {
    font-size: 0.85em;
    font-family: monospace;
  }
  .hidden-color-input {
    position: absolute;
    width: 0;
    height: 0;
    opacity: 0;
    pointer-events: none;
  }
</style>
