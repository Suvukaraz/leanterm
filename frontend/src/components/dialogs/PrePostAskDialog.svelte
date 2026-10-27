<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';

  interface PrePostItem {
    command: string;
    ask: boolean;
    default: boolean;
  }

  interface Props {
    open: boolean;
    phase?: string; // "PRE_EXEC" or "POST_EXEC"
    entries?: (string | PrePostItem)[];
    onexecute?: (selected: any[]) => void;
    onskip?: () => void;
    onclose?: () => void;
  }

  let {
    open = false,
    phase = 'PRE_EXEC',
    entries = [],
    onexecute,
    onskip,
    onclose,
  }: Props = $props();

  let normalizedEntries = $derived.by<PrePostItem[]>(() => {
    return entries.map((e) => {
      if (typeof e === 'string') {
        return { command: e, ask: true, default: true };
      }
      return e;
    });
  });

  let selections = $state<boolean[]>([]);

  $effect(() => {
    if (open) {
      selections = normalizedEntries.map((e) => e.default);
    }
  });

  function handleExecute() {
    const selected = normalizedEntries.filter((_, idx) => selections[idx]);
    if (onexecute) onexecute(selected);
    if (onclose) onclose();
  }

  function handleSkip() {
    if (onskip) onskip();
    if (onclose) onclose();
  }
</script>

{#if open}
  <Dialog
    title={`Confirm ${phase === 'PRE_EXEC' ? 'Pre' : 'Post'} Execution Commands`}
    width={500}
    onclose={handleSkip}
  >
    <div class="prepost-body">
      <p class="description">
        The following {phase === 'PRE_EXEC' ? 'pre-connection' : 'post-disconnection'}
        commands are configured to execute. Select the commands you wish to run:
      </p>

      <div class="command-list">
        {#each normalizedEntries as entry, i}
          <div class="command-item">
            <CheckButton
              label={entry.command}
              bind:checked={selections[i]}
            />
          </div>
        {/each}
      </div>
    </div>

    {#snippet actions()}
      <Button label="Skip All" onclick={handleSkip} />
      <Button
        label="Execute Selected"
        icon="gtk-execute"
        onclick={handleExecute}
      />
    {/snippet}
  </Dialog>
{/if}

<style>
  .prepost-body {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .description {
    font-size: 13px;
    color: var(--text-color, #eee);
    margin: 0;
  }

  .command-list {
    max-height: 250px;
    overflow-y: auto;
    border: 1px solid var(--border-color, #444);
    background: var(--bg-tertiary, #222);
    border-radius: 4px;
    padding: 6px 10px;
  }

  .command-item {
    padding: 4px 0;
    font-family: monospace;
    font-size: 12px;
  }
</style>
