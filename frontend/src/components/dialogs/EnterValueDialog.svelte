<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';

  interface Props {
    open: boolean;
    prompt?: string;
    options?: string[];
    masked?: boolean;
    onsubmit?: (val: string) => void;
    oncancel?: () => void;
  }

  let {
    open = false,
    prompt = 'Please enter a value:',
    options = [],
    masked = false,
    onsubmit,
    oncancel,
  }: Props = $props();

  let enteredValue = $state<string>('');
  let selectedOption = $state<string>('');

  $effect(() => {
    if (open) {
      enteredValue = '';
      selectedOption = options.length > 0 ? options[0] : '';
    }
  });

  function handleSubmit() {
    const val = options.length > 0 ? selectedOption : enteredValue;
    if (onsubmit) onsubmit(val);
  }

  function handleCancel() {
    if (oncancel) oncancel();
  }

  let comboItems = $derived.by(() => {
    return options.map((opt) => ({ value: opt, label: opt }));
  });
</script>

{#if open}
  <Dialog title="Variable Substitution" width={420} onclose={handleCancel}>
    <div class="ask-body">
      <div class="prompt-text">{prompt}</div>

      {#if options.length > 0}
        <ComboBoxText options={comboItems} bind:value={selectedOption} />
      {:else}
        <Entry
          bind:value={enteredValue}
          type={masked ? 'password' : 'text'}
          placeholder="Value..."
          onkeydown={(e) => {
            if (e.key === 'Enter') handleSubmit();
          }}
        />
      {/if}
    </div>

    {#snippet actions()}
      <Button label="Cancel" onclick={handleCancel} />
      <Button label="OK" icon="gtk-ok" onclick={handleSubmit} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .ask-body {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .prompt-text {
    font-size: 13px;
    color: var(--text-color, #eee);
  }
</style>
