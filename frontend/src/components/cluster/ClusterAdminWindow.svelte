<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Notebook from '../gtk/Notebook.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import { sessions } from '../../lib/stores/sessions';
  import { clusters, loadClusters } from '../../lib/stores/clusters';
  import { settings, loadSettings } from '../../lib/stores/config';
  import * as ClusterService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/clusterservice.js';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';

  interface Props {
    open: boolean;
    onclose: () => void;
    onopenPCC?: () => void;
  }

  let { open = false, onclose, onopenPCC }: Props = $props();

  let activeTab = $state<string>('running');
  let tabs = [
    { id: 'running', label: 'Running clusters' },
    { id: 'saved', label: 'Saved clusters' },
    { id: 'auto', label: 'Auto Cluster' },
  ];

  // Running Clusters State
  let selectedCluster = $state<string>('');
  let newClusterName = $state<string>('');
  let selectedUnclusteredTmpID = $state<string>('');
  let selectedClusteredTmpID = $state<string>('');

  // Auto Cluster State
  let autoRuleName = $state<string>('');
  let autoMatchName = $state<string>('');
  let autoMatchTitle = $state<string>('');
  let autoMatchHost = $state<string>('');
  let autoMatchDesc = $state<string>('');
  let autoMatches = $state<string[]>([]);

  $effect(() => {
    if (open) {
      loadClusters();
      if ($clusters.length > 0 && !selectedCluster) {
        selectedCluster = $clusters[0].name;
      }
    }
  });

  // Calculate unclustered sessions
  let unclusteredSessions = $derived.by(() => {
    const clusteredSet = new Set<string>();
    for (const c of $clusters) {
      for (const s of c.sessions || []) {
        clusteredSet.add(s);
      }
    }
    return $sessions.filter((s) => !clusteredSet.has(s.tmpID));
  });

  // Calculate sessions in selected cluster
  let currentClusterSessions = $derived.by(() => {
    const found = $clusters.find((c) => c.name === selectedCluster);
    if (!found) return [];
    const set = new Set(found.sessions || []);
    return $sessions.filter((s) => set.has(s.tmpID));
  });

  let clusterOptions = $derived.by(() => {
    return $clusters.map((c) => ({
      value: c.name,
      label: `${c.name} (${c.sessions?.length || 0} terminals)`,
    }));
  });

  async function handleAddCluster() {
    if (!newClusterName.trim()) return;
    try {
      await ClusterService.Create(newClusterName.trim());
      selectedCluster = newClusterName.trim();
      newClusterName = '';
      await loadClusters();
    } catch (err) {
      console.error('Failed to create cluster:', err);
    }
  }

  async function handleDeleteCluster() {
    if (!selectedCluster) return;
    try {
      await ClusterService.Delete(selectedCluster);
      selectedCluster = '';
      await loadClusters();
    } catch (err) {
      console.error('Failed to delete cluster:', err);
    }
  }

  async function handleAddToCluster() {
    if (!selectedUnclusteredTmpID || !selectedCluster) return;
    try {
      await ClusterService.Add(selectedUnclusteredTmpID, selectedCluster);
      selectedUnclusteredTmpID = '';
      await loadClusters();
    } catch (err) {
      console.error('Failed to add to cluster:', err);
    }
  }

  async function handleRemoveFromCluster() {
    if (!selectedClusteredTmpID) return;
    try {
      await ClusterService.Remove(selectedClusteredTmpID);
      selectedClusteredTmpID = '';
      await loadClusters();
    } catch (err) {
      console.error('Failed to remove from cluster:', err);
    }
  }

  async function previewAutoCluster() {
    try {
      const matches = await ClusterService.AutoClusterMatches({
        name: autoRuleName,
        match_name: autoMatchName,
        match_title: autoMatchTitle,
        match_host: autoMatchHost,
        match_description: autoMatchDesc,
      } as any);
      autoMatches = matches || [];
    } catch (err) {
      console.error('Failed to match auto clusters:', err);
    }
  }

  async function saveAutoClusterRule() {
    if (!autoRuleName) return;
    try {
      const cfg = $settings as any;
      if (!cfg) return;
      if (!cfg.auto_clusters) cfg.auto_clusters = [];
      cfg.auto_clusters.push({
        name: autoRuleName,
        match_name: autoMatchName,
        match_title: autoMatchTitle,
        match_host: autoMatchHost,
        match_description: autoMatchDesc,
      });
      await ConfigService.SaveSettings(cfg);
      await loadSettings();
      autoRuleName = '';
      autoMatchName = '';
      autoMatchTitle = '';
      autoMatchHost = '';
      autoMatchDesc = '';
      autoMatches = [];
    } catch (err) {
      console.error('Failed to save auto cluster:', err);
    }
  }
</script>

{#if open}
  <Dialog title="Cluster Administration : LeanTerm" width={750} {onclose}>
    <div class="admin-body">
      <Notebook {tabs} bind:activeTab>
        <!-- Tab 0: Running Clusters -->
        {#if activeTab === 'running'}
          <div class="tab-content running-tab">
            <div class="column left-col">
              <span class="col-title">Unclustered Terminals</span>
              <div class="list-box">
                {#each unclusteredSessions as s}
                  <!-- svelte-ignore a11y_click_events_have_key_events -->
                  <!-- svelte-ignore a11y_no_static_element_interactions -->
                  <div
                    class="list-item"
                    class:selected={selectedUnclusteredTmpID === s.tmpID}
                    onclick={() => (selectedUnclusteredTmpID = s.tmpID)}
                  >
                    {s.title || 'Terminal'} ({s.kind})
                  </div>
                {/each}
                {#if unclusteredSessions.length === 0}
                  <div class="empty-hint">No unclustered terminals</div>
                {/if}
              </div>
            </div>

            <div class="column mid-buttons">
              <Button
                label="Add to Cluster ›"
                disabled={!selectedUnclusteredTmpID || !selectedCluster}
                onclick={handleAddToCluster}
              />
              <Button
                label="‹ Del from Cluster"
                disabled={!selectedClusteredTmpID}
                onclick={handleRemoveFromCluster}
              />
            </div>

            <div class="column right-col">
              <span class="col-title">Active Clusters</span>
              <div class="cluster-controls">
                <ComboBoxText
                  options={clusterOptions}
                  bind:value={selectedCluster}
                />
                <Button label="Delete" disabled={!selectedCluster} onclick={handleDeleteCluster} />
              </div>

              <div class="new-cluster-row">
                <Entry bind:value={newClusterName} placeholder="New cluster name..." />
                <Button label="Add" disabled={!newClusterName.trim()} onclick={handleAddCluster} />
              </div>

              <span class="col-title" style="margin-top: 8px;">Terminals in Cluster</span>
              <div class="list-box">
                {#each currentClusterSessions as s}
                  <!-- svelte-ignore a11y_click_events_have_key_events -->
                  <!-- svelte-ignore a11y_no_static_element_interactions -->
                  <div
                    class="list-item"
                    class:selected={selectedClusteredTmpID === s.tmpID}
                    onclick={() => (selectedClusteredTmpID = s.tmpID)}
                  >
                    {s.title || 'Terminal'} ({s.kind})
                  </div>
                {/each}
                {#if currentClusterSessions.length === 0}
                  <div class="empty-hint">No terminals in this cluster</div>
                {/if}
              </div>
            </div>
          </div>
        {/if}

        <!-- Tab 1: Saved Clusters -->
        {#if activeTab === 'saved'}
          <div class="tab-content saved-tab">
            <p class="hint">
              Configure static clusters assigned to connections. When launched, connections are automatically grouped into these clusters.
            </p>
            <div class="saved-list">
              {#each ($settings as any)?.connections || [] as conn}
                <div class="saved-item">
                  <span class="conn-title">{conn.title || conn.name}</span>
                  <span class="clusters-tag">
                    {(conn.clusters || []).join(', ') || '(no clusters)'}
                  </span>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Tab 2: Auto Cluster -->
        {#if activeTab === 'auto'}
          <div class="tab-content auto-tab">
            <p class="hint">
              Automatically cluster connections based on regular expressions matching Name, Title, Host or Description.
            </p>
            <div class="auto-grid">
              <label for="auto-name">Cluster Name:</label>
              <Entry bind:value={autoRuleName} placeholder="Production" />

              <label for="auto-title">Match Title (regex):</label>
              <Entry bind:value={autoMatchTitle} placeholder="^prod-.*" onchange={previewAutoCluster} />

              <label for="auto-host">Match Host (regex):</label>
              <Entry bind:value={autoMatchHost} placeholder="^10\.0\..*" onchange={previewAutoCluster} />

              <label for="auto-desc">Match Desc (regex):</label>
              <Entry bind:value={autoMatchDesc} placeholder=".*database.*" onchange={previewAutoCluster} />
            </div>

            <div class="auto-actions">
              <Button label="Preview Matches" icon="gtk-find" onclick={previewAutoCluster} />
              <Button label="Save Rule" icon="gtk-save" disabled={!autoRuleName} onclick={saveAutoClusterRule} />
            </div>

            {#if autoMatches.length > 0}
              <div class="preview-box">
                <span class="preview-title">Matches ({autoMatches.length}):</span>
                <ul>
                  {#each autoMatches as m}
                    <li>{m}</li>
                  {/each}
                </ul>
              </div>
            {/if}
          </div>
        {/if}
      </Notebook>
    </div>

    {#snippet actions()}
      <Button
        label="Power Cluster Controller"
        icon="gtk-justify-fill"
        onclick={() => {
          onclose();
          if (onopenPCC) onopenPCC();
        }}
      />
      <Button label="Close" onclick={onclose} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .admin-body {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .tab-content {
    min-height: 340px;
    padding: 10px 0;
  }

  .running-tab {
    display: flex;
    gap: 12px;
  }

  .column {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .left-col,
  .right-col {
    flex: 1;
  }

  .mid-buttons {
    width: 140px;
    justify-content: center;
    gap: 12px;
  }

  .col-title {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted, #aaa);
    text-transform: uppercase;
  }

  .list-box {
    flex: 1;
    min-height: 180px;
    max-height: 220px;
    overflow-y: auto;
    border: 1px solid var(--border-color, #444);
    background: var(--bg-tertiary, #222);
    border-radius: 4px;
    padding: 4px;
  }

  .list-item {
    padding: 4px 8px;
    font-size: 12px;
    cursor: pointer;
    border-radius: 3px;
  }

  .list-item:hover {
    background: var(--bg-hover, #383838);
  }

  .list-item.selected {
    background: var(--accent-color, #3584e4);
    color: #fff;
  }

  .empty-hint {
    padding: 12px;
    font-size: 11px;
    color: var(--text-muted, #777);
    text-align: center;
  }

  .cluster-controls,
  .new-cluster-row {
    display: flex;
    gap: 6px;
  }

  .hint {
    font-size: 12px;
    color: var(--text-muted, #aaa);
    margin: 0 0 10px 0;
  }

  .saved-list {
    max-height: 280px;
    overflow-y: auto;
    border: 1px solid var(--border-color, #444);
    background: var(--bg-tertiary, #222);
    border-radius: 4px;
  }

  .saved-item {
    display: flex;
    justify-content: space-between;
    padding: 6px 12px;
    font-size: 12px;
    border-bottom: 1px solid var(--border-color, #333);
  }

  .auto-grid {
    display: grid;
    grid-template-columns: 140px 1fr;
    row-gap: 8px;
    column-gap: 8px;
    align-items: center;
    font-size: 12px;
  }

  .auto-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 10px;
  }

  .preview-box {
    margin-top: 10px;
    background: var(--bg-tertiary, #222);
    border: 1px solid var(--border-color, #444);
    border-radius: 4px;
    padding: 8px 12px;
    font-size: 12px;
  }
</style>
