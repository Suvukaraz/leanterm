<script lang="ts">
  // Question dialog with free button labels (Save / Discard / Cancel, ...).
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import { getIcon } from '../../lib/theme/icons';

  export interface Choice {
    id: string;
    label: string;
    icon?: string;
    isDefault?: boolean;
  }

  interface Props {
    title: string;
    message: string;
    detail?: string;
    type?: 'question' | 'warning' | 'error' | 'info';
    choices: Choice[];
    /** Answer for Escape / the × button. */
    cancelId?: string;
    onanswer: (id: string) => void;
  }

  let { title, message, detail = '', type = 'question', choices, cancelId = 'cancel', onanswer }: Props = $props();

  const icon = $derived(
    type === 'warning' || type === 'error' ? 'gtk-dialog-warning' : type === 'info' ? 'gtk-info' : 'gtk-dialog-question'
  );
</script>

<Dialog {title} width={480} onclose={() => onanswer(cancelId)}>
  <div class="choice-body">
    <img src={getIcon(icon)} alt="" width="40" height="40" class="choice-icon" />
    <div class="choice-text">
      <p class="choice-message">{message}</p>
      {#if detail}
        <p class="choice-detail">{detail}</p>
      {/if}
    </div>
  </div>

  {#snippet actions()}
    {#each choices as c (c.id)}
      <Button label={c.label} icon={c.icon} isDefault={c.isDefault} onclick={() => onanswer(c.id)} />
    {/each}
  {/snippet}
</Dialog>

<style>
  .choice-body {
    display: flex;
    gap: 14px;
    align-items: flex-start;
  }
  .choice-icon {
    flex-shrink: 0;
  }
  .choice-message {
    margin: 0 0 6px;
    font-weight: bold;
    white-space: pre-wrap;
  }
  .choice-detail {
    margin: 0;
    color: var(--gtk-muted-fg);
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>
