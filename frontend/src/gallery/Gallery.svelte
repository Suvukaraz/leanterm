<script lang="ts">
  // The GTK components in fixed states, laid out like the native gallery
  // (cmd/gtk-gallery) so screenshots can be compared side by side.
  import Button from '../components/gtk/Button.svelte';
  import ToggleButton from '../components/gtk/ToggleButton.svelte';
  import CheckButton from '../components/gtk/CheckButton.svelte';
  import RadioGroup from '../components/gtk/RadioGroup.svelte';
  import Entry from '../components/gtk/Entry.svelte';
  import SpinButton from '../components/gtk/SpinButton.svelte';
  import ComboBoxText from '../components/gtk/ComboBoxText.svelte';
  import Frame from '../components/gtk/Frame.svelte';
  import ProgressBar from '../components/gtk/ProgressBar.svelte';
  import Scale from '../components/gtk/Scale.svelte';
  import Statusbar from '../components/gtk/Statusbar.svelte';
  import Notebook from '../components/gtk/Notebook.svelte';
  import ColorButton from '../components/gtk/ColorButton.svelte';
  import TreeView, { type TreeItem } from '../components/gtk/TreeView.svelte';
  import ContextMenu from '../components/gtk/ContextMenu.svelte';
  import MessageDialog from '../components/gtk/MessageDialog.svelte';

  const section = new URLSearchParams(location.search).get('section') ?? 'basics';

  const tree: TreeItem = {
    id: '__root__', name: 'My Connections', isGroup: true, expanded: true,
    children: [
      { id: 'g1', name: 'Customers', isGroup: true, expanded: true, children: [
        { id: 'c1', name: 'acme-backup', isGroup: false, method: 'ssh', host: '10.0.0.1' },
        { id: 'c2', name: 'acme-erp', isGroup: false, method: 'ssh', host: '10.0.0.2', protected: true },
      ] },
      { id: 'g2', name: 'Home Lab', isGroup: true, expanded: false, children: [] },
      { id: 'c3', name: 'web-prod-01', isGroup: false, method: 'ssh', favourite: true },
      { id: 'c4', name: 'switch-01', isGroup: false, method: 'telnet' },
    ],
  };
</script>

{#if section === 'basics'}
<div class="gallery">
  <div class="row">
    <Button label="Normal" />
    <Button label="Default" isDefault={true} />
    <Button label="Disabled" disabled={true} />
    <Button label="With icon" icon="gtk-connect" />
    <ToggleButton label="Toggle off" />
    <ToggleButton label="Toggle on" active={true} />
  </div>
  <div class="row">
    <CheckButton label="Checked" checked={true} />
    <CheckButton label="Unchecked" />
    <CheckButton label="Disabled" disabled={true} />
    <RadioGroup name="r" value="b" options={[{ value: 'a', label: 'Alpha' }, { value: 'b', label: 'Beta' }, { value: 'c', label: 'Gamma' }]} />
  </div>
  <div class="row">
    <Entry value="Entry text" />
    <Entry placeholder="Placeholder" />
    <Entry value="secret" type="password" />
    <SpinButton value={22} min={1} max={65535} />
    <ComboBoxText value="ssh" options={[{ value: 'ssh', label: 'SSH' }, { value: 'telnet', label: 'Telnet' }]} />
    <ColorButton value="#4a90d9" />
  </div>
  <div class="row">
    <Frame label="Frame label">
      <CheckButton label="Inside a frame" checked={true} />
      <Entry value="Second row" />
    </Frame>
    <div class="col">
      <ProgressBar value={40} />
      <Scale value={0.3} min={0} max={1} step={0.01} />
    </div>
  </div>
  <div class="nb">
    <Notebook tabs={[{ id: 't1', label: 'Info', icon: 'gtk-info' }, { id: 't2', label: 'web-prod-01', icon: 'asbru-protected', color: '#73d216', closable: true }, { id: 't3', label: 'db-prod-01', closable: true }]} activeTab="t2">
      {#snippet children(id: string)}<div class="nb-body">Tab {id}</div>{/snippet}
    </Notebook>
  </div>
  <Statusbar>Status: CONNECTED</Statusbar>
</div>
{:else if section === 'tree'}
<div class="tree-box">
  <TreeView root={tree} selectedIds={["c3"]} expandedIds={["__root__", "g1"]} />
</div>
{:else if section === 'menu'}
<ContextMenu x={20} y={20} onclose={() => {}} items={[
  { id: 'a', label: 'Connect', icon: 'gtk-connect' },
  { id: 'b', label: 'Edit', shortcut: 'F2' },
  { separator: true },
  { id: 'c', label: 'Favourite', checked: true },
  { id: 'd', label: 'Disabled', disabled: true },
  { id: 'e', label: 'More', submenu: [{ id: 'f', label: 'Sub item' }] },
]} />
{:else if section === 'message'}
<MessageDialog open={true} type="question" title="Quit" primaryText="Close all connections?" secondaryText="3 connections are still open." buttons="yes_no" />
{/if}

<style>
  .gallery { display: flex; flex-direction: column; gap: 12px; padding: 12px; width: 980px; }
  .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .col { display: flex; flex-direction: column; gap: 10px; width: 260px; }
  .nb { height: 120px; display: flex; flex-direction: column; }
  .nb-body { padding: 8px; }
  .tree-box { width: 300px; height: 260px; display: flex; flex-direction: column; }
</style>
