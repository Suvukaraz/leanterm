<script lang="ts">
  import Dialog from './Dialog.svelte';
  import { getIcon } from '../../lib/theme/icons';

  export type DialogType = 'info' | 'warning' | 'error' | 'question';

  interface Props {
    open?: boolean;
    type?: DialogType;
    title?: string;
    primaryText?: string;
    secondaryText?: string;
    buttons?: 'ok' | 'ok_cancel' | 'yes_no' | 'yes_no_cancel';
    onresponse?: (response: 'ok' | 'cancel' | 'yes' | 'no') => void;
  }

  let {
    open = $bindable(true),
    type = 'info',
    title = '',
    primaryText = '',
    secondaryText = '',
    buttons = 'ok',
    onresponse
  }: Props = $props();

  const iconName = $derived.by(() => {
    switch (type) {
      case 'warning':
        return 'gtk-dialog-warning';
      case 'error':
        return 'gtk-dialog-error';
      case 'question':
        return 'gtk-dialog-question';
      case 'info':
      default:
        return 'gtk-info';
    }
  });

  const dialogClass = $derived.by(() => {
    switch (type) {
      case 'warning':
        return 'w-warning';
      case 'error':
        return 'w-error';
      case 'question':
        return 'w-confirm';
      default:
        return 'w-info';
    }
  });

  function respond(ans: 'ok' | 'cancel' | 'yes' | 'no') {
    open = false;
    if (onresponse) onresponse(ans);
  }
</script>

<Dialog
  bind:open={open}
  title={title || type.toUpperCase()}
  width={440}
  class={dialogClass}
  onclose={() => respond('cancel')}
>
  <div class="message-body">
    <img src={getIcon(iconName)} alt="" width="32" height="32" class="dialog-icon" />
    <div class="text-block">
      {#if primaryText}
        <p class="primary-text"><b>{primaryText}</b></p>
      {/if}
      {#if secondaryText}
        <p class="secondary-text">{secondaryText}</p>
      {/if}
    </div>
  </div>

  {#snippet actions()}
    {#if buttons === 'ok'}
      <button type="button" class="btn-primary" onclick={() => respond('ok')}>OK</button>
    {:else if buttons === 'ok_cancel'}
      <button type="button" onclick={() => respond('cancel')}>Cancel</button>
      <button type="button" class="btn-primary" onclick={() => respond('ok')}>OK</button>
    {:else if buttons === 'yes_no'}
      <button type="button" onclick={() => respond('no')}>No</button>
      <button type="button" class="btn-primary" onclick={() => respond('yes')}>Yes</button>
    {:else if buttons === 'yes_no_cancel'}
      <button type="button" onclick={() => respond('cancel')}>Cancel</button>
      <button type="button" onclick={() => respond('no')}>No</button>
      <button type="button" class="btn-primary" onclick={() => respond('yes')}>Yes</button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .message-body {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 6px 0;
  }
  .dialog-icon {
    flex-shrink: 0;
    margin-top: 2px;
  }
  .text-block {
    flex: 1;
    font-size: 0.95em;
  }
  .primary-text {
    margin: 0 0 6px 0;
    font-size: 1.05em;
  }
  .secondary-text {
    margin: 0;
    color: var(--gtk-fg-color);
    white-space: pre-wrap;
    word-break: break-word;
  }
  .btn-primary {
    background-color: var(--gtk-selected-bg);
    color: var(--gtk-selected-fg);
  }
</style>
