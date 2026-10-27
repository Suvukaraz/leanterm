<script lang="ts">
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import Button from '../gtk/Button.svelte';
  import FileChooserButton from '../gtk/FileChooserButton.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';

  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  const net = $derived(settings.network);

  const proxyModeOptions = [
    { value: 'direct', label: 'Do not use proxy (Direct connection)' },
    { value: 'socks', label: 'SOCKS proxy' },
    { value: 'jump', label: 'Use Jump Server' },
  ];

  let showSocksPassword = $state(false);
  let showJumpPassword = $state(false);
</script>

<div class="network-page">
  <div class="top-row">
    <Frame label="Global Network Settings">
      <div class="col-flex">
        <RadioGroup
          name="global_network_mode"
          options={proxyModeOptions}
          bind:value={net.Mode}
          direction="vertical"
        />

      </div>
    </Frame>
  </div>

  {#if net.Mode === 'socks'}
    <Frame label="Global SOCKS Proxy">
      <div class="col-flex">
        <div class="row-flex">
          <span class="field-label">IP / Host:</span>
          <Entry
            bind:value={net.Socks.Host}
            placeholder="Proxy host"
          />
          <span class="field-label" style="margin-left: 12px;">Port:</span>
          <SpinButton
            bind:value={net.Socks.Port}
            min={1}
            max={65535}
          />
        </div>
        <div class="row-flex">
          <span class="field-label">User:</span>
          <Entry
            bind:value={net.Socks.User}
            placeholder="Proxy username"
          />
        </div>
      </div>
    </Frame>
  {:else if net.Mode === 'jump'}
    <Frame label="Global Jump Server">
      <div class="col-flex">
        <div class="row-flex">
          <span class="field-label">IP / Host:</span>
          <Entry
            bind:value={net.Jump.Host}
            placeholder="Jump server host"
          />
          <span class="field-label" style="margin-left: 12px;">Port:</span>
          <SpinButton
            bind:value={net.Jump.Port}
            min={1}
            max={65535}
          />
        </div>
        <div class="row-flex">
          <span class="field-label">User:</span>
          <Entry
            bind:value={net.Jump.User}
            placeholder="Jump server username"
          />
        </div>
        <div class="row-flex">
          <span class="field-label">Private key file:</span>
          <FileChooserButton
            bind:path={net.Jump.KeyFile}
            title="Select Jump Server Private Key"
          />
          <Button
            label="Clear"
            onclick={() => (net.Jump.KeyFile = '')}
          />
        </div>
      </div>
    </Frame>
  {/if}
</div>

<style>
  .network-page {
    padding: 10px;
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-sizing: border-box;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .field-label {
    min-width: 120px;
    font-size: 0.9em;
  }

</style>
