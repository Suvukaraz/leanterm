<script lang="ts">
  import type { TransferItem } from '../../lib/stores/sftp';

  interface Props {
    transfers: TransferItem[];
    oncancel: (id: string) => void;
    ondismiss: (id: string) => void;
  }

  let { transfers, oncancel, ondismiss }: Props = $props();

  let expanded = $state(false);

  // Auto-expand when active transfers exist
  let activeTransfers = $derived(
    transfers.filter((t) => t.status === 'running')
  );

  $effect(() => {
    if (activeTransfers.length > 0) {
      expanded = true;
    }
  });

  function toggleExpand() {
    expanded = !expanded;
  }
</script>

{#if transfers.length > 0}
  <div class="transfer-queue-container">
    <button type="button" class="queue-header" onclick={toggleExpand}>
      <span class="expand-arrow">{expanded ? '▼' : '▶'}</span>
      <span class="queue-title">
        Transfers ({activeTransfers.length} running, {transfers.length} total)
      </span>
    </button>

    {#if expanded}
      <div class="queue-list">
        {#each transfers as t (t.id)}
          <div class="transfer-item">
            <div class="transfer-meta">
              <span class="transfer-direction" title={t.direction}>
                {t.direction === 'upload' ? '⬆' : '⬇'}
              </span>
              <span class="transfer-name" title={t.name}>{t.name}</span>
              <span class="transfer-status status-{t.status}">
                {#if t.status === 'running'}
                  {t.speed}
                {:else}
                  {t.status}
                {/if}
              </span>
              <button
                type="button"
                class="cancel-btn"
                title={t.status === 'running' ? 'Cancel' : 'Dismiss'}
                onclick={() => (t.status === 'running' ? oncancel(t.id) : ondismiss(t.id))}
              >
                ✕
              </button>
            </div>

            {#if t.status === 'running'}
              <div class="progress-bar-bg">
                <div class="progress-bar-fill" style="width: {t.progress}%"></div>
              </div>
            {/if}

            {#if t.error}
              <div class="transfer-error" title={t.error}>{t.error}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .transfer-queue-container {
    background: var(--gtk-base, #ffffff);
    border-top: 1px solid var(--gtk-border, #d3d7cf);
    font-size: 11px;
    color: var(--gtk-fg, #2e3436);
  }

  .queue-header {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 3px 6px;
    background: var(--gtk-header-bg, #f0f0f0);
    border: none;
    border-bottom: 1px solid var(--gtk-border, #d3d7cf);
    cursor: pointer;
    text-align: left;
    font-size: 11px;
    color: var(--gtk-fg, #2e3436);
    user-select: none;
  }

  .queue-header:hover {
    background: var(--gtk-header-bg);
  }

  .expand-arrow {
    font-size: 9px;
  }

  .queue-title {
    font-weight: 600;
  }

  .queue-list {
    max-height: 120px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding: 4px 6px;
    gap: 6px;
  }

  .transfer-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 2px 0;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .transfer-meta {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .transfer-direction {
    font-weight: bold;
    width: 14px;
    text-align: center;
  }

  .transfer-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: monospace;
  }

  .transfer-status {
    font-size: 10px;
    color: var(--gtk-muted-fg);
  }

  .status-completed {
    color: var(--gtk-success-fg);
    font-weight: bold;
  }

  .status-failed {
    color: var(--gtk-error-fg);
    font-weight: bold;
  }

  .status-cancelled {
    color: var(--gtk-muted-fg);
  }

  .cancel-btn {
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--gtk-muted-fg);
    padding: 0 4px;
    font-size: 10px;
  }

  .cancel-btn:hover {
    color: var(--gtk-error-fg);
  }

  .progress-bar-bg {
    width: 100%;
    height: 4px;
    background: var(--gtk-header-bg);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: var(--gtk-selected-bg, #4a90d9);
    transition: width 0.2s ease;
  }

  .transfer-error {
    color: var(--gtk-error-fg);
    font-size: 10px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
