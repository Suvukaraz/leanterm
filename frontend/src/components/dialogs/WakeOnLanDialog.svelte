<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  interface Props {
    open: boolean;
    initialHost?: string;
    initialMAC?: string;
    onsend?: (mac: string) => void;
    onclose: () => void;
  }

  let {
    open = false,
    initialHost = '',
    initialMAC = '',
    onsend,
    onclose,
  }: Props = $props();

  let mac = $state<string>('');
  let host = $state<string>('');
  let port = $state<number>(9);
  let broadcast = $state<boolean>(true);
  let statusText = $state<string>('');
  let isSending = $state<boolean>(false);

  $effect(() => {
    if (open) {
      host = initialHost;
      mac = initialMAC;
      port = 9;
      broadcast = true;
      statusText = '';
    }
  });

  async function checkArp() {
    if (!host) {
      statusText = 'Please enter a host or IP first';
      return;
    }
    statusText = `Checking MAC for '${host}' ...`;
    try {
      const found = await SystemService.LookupArpMAC(host);
      if (found) {
        mac = found;
        statusText = `Found MAC: ${found}`;
      } else {
        statusText = 'MAC not found in ARP table';
      }
    } catch (err: any) {
      statusText = `Failed: ${err?.message || err}`;
    }
  }

  async function handleSend() {
    if (!mac) {
      statusText = 'Please enter a valid MAC address';
      return;
    }
    isSending = true;
    statusText = 'Sending Wake-on-LAN magic packet...';
    try {
      await SystemService.WakeOnLan(mac, host, port, broadcast);
      statusText = 'Magic packet sent successfully!';
      if (onsend) onsend(mac);
      setTimeout(() => {
        onclose();
      }, 1000);
    } catch (err: any) {
      statusText = `Error sending packet: ${err?.message || err}`;
    } finally {
      isSending = false;
    }
  }
</script>

{#if open}
  <Dialog title="Wake On LAN" width={480} {onclose}>
    <div class="wol-body">
      <div class="form-grid">
        <label for="wol-mac">MAC Address:</label>
        <div class="mac-row">
          <Entry
            bind:value={mac}
            placeholder="00:11:22:33:44:55"
          />
          <Button
            label="Check ARP"
            icon="gtk-find"
            tooltip="Query local ARP cache for MAC"
            onclick={checkArp}
          />
        </div>

        <label for="wol-host">Host / IP:</label>
        <Entry bind:value={host} placeholder="192.168.1.100" />

        <label for="wol-port">Port Number:</label>
        <SpinButton
          bind:value={port}
          min={1}
          max={65535}
          disabled={broadcast}
        />

        <div></div>
        <CheckButton
          label="Send to broadcast (255.255.255.255:9)"
          bind:checked={broadcast}
        />
      </div>

      {#if statusText}
        <div class="status-box">{statusText}</div>
      {/if}
    </div>

    {#snippet actions()}
      <Button label="Close" onclick={onclose} />
      <Button
        label="Wake up"
        icon="gtk-network"
        disabled={!mac || isSending}
        onclick={handleSend}
      />
    {/snippet}
  </Dialog>
{/if}

<style>
  .wol-body {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: 120px 1fr;
    row-gap: 10px;
    column-gap: 8px;
    align-items: center;
  }

  .form-grid label {
    font-size: 12px;
    color: var(--text-color, #eee);
  }

  .mac-row {
    display: flex;
    gap: 6px;
  }

  .status-box {
    font-size: 12px;
    color: var(--accent-color, #3584e4);
    background: var(--bg-tertiary, #222);
    padding: 6px 10px;
    border-radius: 4px;
    border-left: 3px solid var(--accent-color, #3584e4);
  }
</style>
