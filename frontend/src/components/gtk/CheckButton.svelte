<script lang="ts">
  interface Props {
    label?: string;
    checked?: boolean;
    disabled?: boolean;
    tooltip?: string;
    class?: string;
    onchange?: (checked: boolean) => void;
  }

  let {
    label = '',
    checked = $bindable(false),
    disabled = false,
    tooltip = '',
    class: className = '',
    onchange
  }: Props = $props();

  function handleChange(e: Event) {
    const target = e.target as HTMLInputElement;
    checked = target.checked;
    if (onchange) onchange(checked);
  }
</script>

<label class={`gtk-checkbutton ${className}`} title={tooltip}>
  <input
    type="checkbox"
    bind:checked={checked}
    {disabled}
    onchange={handleChange}
  />
  {#if label}
    <span class="label-text">{label}</span>
  {/if}
</label>

<style>
  .gtk-checkbutton {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: inherit;
    cursor: pointer;
    user-select: none;
    margin: 2px 0;
  }
  .gtk-checkbutton input[type="checkbox"] {
    margin: 0;
  }
  .label-text {
    line-height: 1.2;
  }
</style>
