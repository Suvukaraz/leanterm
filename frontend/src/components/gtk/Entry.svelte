<script lang="ts">
  interface Props {
    value?: string;
    type?: 'text' | 'password';
    placeholder?: string;
    disabled?: boolean;
    readonly?: boolean;
    tooltip?: string;
    class?: string;
    oninput?: (val: string) => void;
    onchange?: (val: string) => void;
    onkeydown?: (e: KeyboardEvent) => void;
  }

  let {
    value = $bindable(''),
    type = 'text',
    placeholder = '',
    disabled = false,
    readonly = false,
    tooltip = '',
    class: className = '',
    oninput,
    onchange,
    onkeydown
  }: Props = $props();

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    if (oninput) oninput(target.value);
  }

  function handleChange(e: Event) {
    const target = e.target as HTMLInputElement;
    if (onchange) onchange(target.value);
  }
</script>

<input
  {type}
  class={`gtk-entry ${className}`}
  bind:value={value}
  {placeholder}
  {disabled}
  {readonly}
  title={tooltip}
  oninput={handleInput}
  onchange={handleChange}
  {onkeydown}
/>

<style>
  .gtk-entry {
    font-family: inherit;
    font-size: inherit;
    padding: 3px 6px;
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    background-color: var(--gtk-base-color);
    color: var(--gtk-text-color);
    outline: none;
    box-sizing: border-box;
  }

  .gtk-entry:focus {
    border-color: var(--gtk-selected-bg);
    box-shadow: 0 0 0 1px var(--gtk-selected-bg);
  }

  .gtk-entry:disabled {
    opacity: 0.5;
    background-color: #f5f5f5;
    cursor: not-allowed;
  }
</style>
