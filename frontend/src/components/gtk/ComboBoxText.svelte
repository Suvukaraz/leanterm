<script lang="ts">
  interface OptionItem {
    value: string;
    label: string;
  }

  interface Props {
    options: (string | OptionItem)[];
    value?: string;
    disabled?: boolean;
    tooltip?: string;
    class?: string;
    onchange?: (value: string) => void;
  }

  let {
    options = [],
    value = $bindable(''),
    disabled = false,
    tooltip = '',
    class: className = '',
    onchange
  }: Props = $props();

  const normalizedOptions = $derived(
    options.map((opt) => (typeof opt === 'string' ? { value: opt, label: opt } : opt))
  );

  function handleChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    value = target.value;
    if (onchange) onchange(value);
  }
</script>

<select
  class={`gtk-combo ${className}`}
  {value}
  {disabled}
  title={tooltip}
  onchange={handleChange}
>
  {#each normalizedOptions as opt}
    <option value={opt.value}>{opt.label}</option>
  {/each}
</select>

<style>
  .gtk-combo {
    font-family: inherit;
    font-size: inherit;
    padding: 3px 6px;
    border: 1px solid var(--gtk-border-color);
    background-color: var(--gtk-base-color);
    color: var(--gtk-text-color);
    border-radius: 2px;
    outline: none;
    cursor: pointer;
    box-sizing: border-box;
  }
  .gtk-combo:focus {
    background-color: var(--gtk-btn-focus-bg);
  }
</style>
