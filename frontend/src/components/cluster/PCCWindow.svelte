<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import { clusters } from '../../lib/stores/clusters';
  import * as ClusterService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/clusterservice.js';

  interface Props {
    open: boolean;
    onclose: () => void;
    onopenClusterAdmin?: () => void;
  }

  let { open = false, onclose, onopenClusterAdmin }: Props = $props();

  let selectedCluster = $state<string>('');
  let sendToAll = $state<boolean>(false);
  let preventSingleBroadcast = $state<boolean>(false);
  let multiLine = $state<boolean>(false);
  let sendOnEnter = $state<boolean>(true);
  let useVariables = $state<boolean>(true);

  let singleLineCmd = $state<string>('');
  let multiLineCmd = $state<string>('');
  let statusMsg = $state<string>('');

  $effect(() => {
    if (open && $clusters.length > 0 && !selectedCluster) {
      selectedCluster = $clusters[0].name;
    }
  });

  $effect(() => {
    ClusterService.SetPreventSingleBroadcast(preventSingleBroadcast);
  });

  let clusterOptions = $derived.by(() => {
    return $clusters.map((c) => ({
      value: c.name,
      label: `${c.name} (${c.sessions?.length || 0} terminals)`,
    }));
  });

  async function sendCommand(text: string) {
    if (!text.trim()) return;
    try {
      await ClusterService.PCCSend(selectedCluster, text, {
        sendToAll,
        preventSingleBroadcast,
        useVariables,
        appendReturn: true,
      } as any);
      statusMsg = `Sent: ${text.length > 40 ? text.slice(0, 40) + '...' : text}`;
    } catch (err: any) {
      statusMsg = `Error: ${err?.message || err}`;
    }
  }

  function handleSingleLineSubmit() {
    if (singleLineCmd) {
      sendCommand(singleLineCmd);
      singleLineCmd = '';
    }
  }

  function handleExecuteAll() {
    sendCommand(multiLineCmd);
  }

  async function handleRestartAll() {
    if (!selectedCluster && !sendToAll) return;
    try {
      await ClusterService.RestartAll(selectedCluster);
      statusMsg = 'Restart command sent to cluster';
    } catch (err: any) {
      statusMsg = `Error: ${err?.message || err}`;
    }
  }

  async function handleCloseAll() {
    if (!selectedCluster && !sendToAll) return;
    try {
      await ClusterService.CloseAll(selectedCluster);
      statusMsg = 'Close command sent to cluster';
    } catch (err: any) {
      statusMsg = `Error: ${err?.message || err}`;
    }
  }
</script>

{#if open}
  <Dialog title="LeanTerm: Power Cluster Controller" width={700} {onclose}>
    <div class="pcc-body">
      <!-- Top cluster selection & global controls -->
      <div class="top-row">
        <div class="cluster-pick">
          <span>Send to Cluster:</span>
          <ComboBoxText
            options={clusterOptions}
            bind:value={selectedCluster}
            disabled={sendToAll}
          />
        </div>
        <CheckButton label="Send to ALL terminals" bind:checked={sendToAll} />
      </div>

      <div class="button-row">
        <Button label="Restart all" icon="gtk-refresh" onclick={handleRestartAll} />
        <Button label="Close ALL" icon="gtk-close" onclick={handleCloseAll} />
        <CheckButton
          label="Prevent single window keys broadcast"
          bind:checked={preventSingleBroadcast}
        />
        <CheckButton label="Use Multi-Line Text Entry" bind:checked={multiLine} />
      </div>

      <!-- Command entry section -->
      {#if !multiLine}
        <div class="single-line-box">
          <label for="pcc-single-entry">Type commands here:</label>
          <div class="input-actions">
            <Entry
              bind:value={singleLineCmd}
              placeholder="Enter command to broadcast..."
              onkeydown={(e) => {
                if (e.key === 'Enter' && sendOnEnter) {
                  handleSingleLineSubmit();
                }
              }}
            />
            <Button
              label="Send"
              icon="gtk-execute"
              disabled={!singleLineCmd}
              onclick={handleSingleLineSubmit}
            />
          </div>
          <div class="single-options">
            <CheckButton label="Send on <ENTER>" bind:checked={sendOnEnter} />
            <CheckButton label="Use LeanTerm variables" bind:checked={useVariables} />
          </div>
        </div>
      {:else}
        <div class="multi-line-box">
          <textarea
            bind:value={multiLineCmd}
            placeholder="Type multi-line script or commands here (Ctrl+Enter to execute all)..."
            rows="10"
            onkeydown={(e) => {
              if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                handleExecuteAll();
              }
            }}
          ></textarea>
          <div class="multi-actions">
            <div class="left-actions">
              <Button label="Clear" icon="gtk-clear" onclick={() => (multiLineCmd = '')} />
              <CheckButton label="Use LeanTerm variables" bind:checked={useVariables} />
            </div>
            <div class="right-actions">
              <Button
                label="Execute All (Ctrl+Enter)"
                icon="gtk-execute"
                disabled={!multiLineCmd}
                onclick={handleExecuteAll}
              />
            </div>
          </div>
        </div>
      {/if}

      {#if statusMsg}
        <div class="status-box">{statusMsg}</div>
      {/if}
    </div>

    {#snippet actions()}
      <Button
        label="Clusters"
        icon="asbru-cluster-manager2"
        onclick={() => {
          onclose();
          if (onopenClusterAdmin) onopenClusterAdmin();
        }}
      />
      <Button label="Close" onclick={onclose} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .pcc-body {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .top-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .cluster-pick {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    font-size: 12px;
  }

  .button-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding: 6px 0;
    border-top: 1px solid var(--border-color, #444);
    border-bottom: 1px solid var(--border-color, #444);
    font-size: 12px;
  }

  .single-line-box {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 12px;
  }

  .input-actions {
    display: flex;
    gap: 6px;
  }

  .single-options {
    display: flex;
    gap: 16px;
  }

  .multi-line-box {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .multi-line-box textarea {
    width: 100%;
    background: var(--bg-tertiary, #222);
    color: var(--text-color, #eee);
    border: 1px solid var(--border-color, #444);
    border-radius: 4px;
    padding: 8px;
    font-family: monospace;
    font-size: 12px;
    resize: vertical;
    box-sizing: border-box;
  }

  .multi-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .left-actions,
  .right-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-box {
    font-size: 12px;
    color: var(--accent-color, #3584e4);
    background: var(--bg-tertiary, #222);
    padding: 4px 8px;
    border-radius: 4px;
    border-left: 3px solid var(--accent-color, #3584e4);
  }
</style>
