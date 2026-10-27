<script lang="ts">
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Button from '../gtk/Button.svelte';
  import LinkButton from '../gtk/LinkButton.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import { settings } from '../../lib/stores/config';

  interface Props {
    conn: any;
    disabled?: boolean;
  }

  let { conn = $bindable(), disabled = false }: Props = $props();

  const ipProtocolOptions = [
    { value: 'any', label: 'any' },
    { value: '4', label: '4' },
    { value: '6', label: '6' },
  ];

  const lineEndingOptions = [
    { value: 'crlf', label: 'CR LF' },
    { value: 'crnul', label: 'CR NUL' },
    { value: 'lf', label: 'LF' },
  ];

  const supportedSSHOptions = [
    'AddressFamily',
    'BindAddress',
    'Ciphers',
    'ConnectTimeout',
    'ExitOnForwardFailure',
    'ForwardAgent',
    'ForwardX11',
    'GatewayPorts',
    'HostKeyAlgorithms',
    'HostName',
    'IdentitiesOnly',
    'IdentityFile',
    'KexAlgorithms',
    'MACs',
    'Port',
    'PreferredAuthentications',
    'ProxyJump',
    'PubkeyAcceptedAlgorithms',
    'RequestTTY',
    'SendEnv',
    'ServerAliveCountMax',
    'ServerAliveInterval',
    'SetEnv',
    'StrictHostKeyChecking',
    'User',
    'UserKnownHostsFile',
  ];

  let activeForwardTab = $state('local');

  let localForwards = $state<any[]>([]);
  let remoteForwards = $state<any[]>([]);
  let dynamicForwards = $state<any[]>([]);
  let advOptions = $state<{ key: string; val: string }[]>([]);

  // Sync state with conn
  $effect(() => {
    if (conn.SSH) {
      localForwards = conn.SSH.LocalForwards || [];
      remoteForwards = conn.SSH.RemoteForwards || [];
      dynamicForwards = conn.SSH.DynamicForwards || [];
      const opts = conn.SSH.Options || {};
      advOptions = Object.entries(opts).map(([k, v]) => ({ key: k, val: String(v) }));
    }
  });

  function updateSSHForwards() {
    if (conn.SSH) {
      conn.SSH.LocalForwards = localForwards;
      conn.SSH.RemoteForwards = remoteForwards;
      conn.SSH.DynamicForwards = dynamicForwards;
      const opts: Record<string, string> = {};
      for (const o of advOptions) {
        if (o.key.trim()) opts[o.key.trim()] = o.val;
      }
      conn.SSH.Options = opts;
    }
  }

  const forwardTabs = $derived<TabItem[]>([
    { id: 'local', label: `Local Port Forwarding (${localForwards.length})` },
    { id: 'remote', label: `Remote Port Forwarding (${remoteForwards.length})` },
    { id: 'dynamic', label: `Dynamic Socks Proxy (${dynamicForwards.length})` },
    { id: 'advanced', label: `Advanced Options (${advOptions.length})` },
  ]);

  function addLocalForward() {
    localForwards = [...localForwards, { BindAddress: '127.0.0.1', Port: 8080, Host: '127.0.0.1', HostPort: 80 }];
    updateSSHForwards();
  }

  function removeLocalForward(index: number) {
    localForwards = localForwards.filter((_, i) => i !== index);
    updateSSHForwards();
  }

  function addRemoteForward() {
    remoteForwards = [...remoteForwards, { BindAddress: '127.0.0.1', Port: 9090, Host: '127.0.0.1', HostPort: 80 }];
    updateSSHForwards();
  }

  function removeRemoteForward(index: number) {
    remoteForwards = remoteForwards.filter((_, i) => i !== index);
    updateSSHForwards();
  }

  function addDynamicForward() {
    dynamicForwards = [...dynamicForwards, { BindAddress: '127.0.0.1', Port: 1080 }];
    updateSSHForwards();
  }

  function removeDynamicForward(index: number) {
    dynamicForwards = dynamicForwards.filter((_, i) => i !== index);
    updateSSHForwards();
  }

  function addAdvOption() {
    advOptions = [...advOptions, { key: 'ServerAliveInterval', val: '30' }];
    updateSSHForwards();
  }

  function removeAdvOption(index: number) {
    advOptions = advOptions.filter((_, i) => i !== index);
    updateSSHForwards();
  }

  function openHelp() {
    const url = $settings?.general?.HelpURL || 'https://www.asbru-cm.net/';
    SystemService.OpenURL(url);
  }
</script>

<div class="options-tab-container">
  {#if conn.Method === 'ssh' || conn.Method === 'sftp'}
    {#if conn.SSH}
      <div class="top-row">
        <div class="row-flex">
          <span class="field-label">IP Protocol:</span>
          <ComboBoxText
            options={ipProtocolOptions}
            bind:value={conn.SSH.IPVersion}
            tooltip="-(4|6) : Uses IPv4, IPv6 or no specification (ip based)"
            {disabled}
          />
          <LinkButton label="Help" tooltip="SSH options help" url={$settings?.general?.HelpURL || 'https://www.asbru-cm.net/'} />
        </div>
      </div>

      {#if conn.Method === 'ssh'}
        <Frame label="Other Options">
          <div class="other-options-grid">
            <CheckButton
              bind:checked={conn.SSH.ForwardX11}
              label="Forward X"
              tooltip="-(X|x) : Forwards or not the X"
              {disabled}
            />
            <CheckButton
              bind:checked={conn.SSH.GatewayPorts}
              label="Allow Remote Port Connect"
              tooltip="[-g] : Allow or not Remote Port Connections"
              {disabled}
            />
            <CheckButton
              bind:checked={conn.SSH.ForwardAgent}
              label="Forward Agent"
              tooltip="[-A] : Forward or not the SSH authentication agent"
              {disabled}
            />
            <CheckButton
              bind:checked={conn.SSH.NoRemoteCommand}
              label="Do NOT execute remote command"
              tooltip="[-N] : Do not execute a remote command. This is useful for just forwarding ports."
              {disabled}
            />
            <CheckButton
              bind:checked={conn.SSH.RandomSOCKSTunnel}
              label="Create random SOCKS tunnel"
              tooltip="Creates a SOCKS tunnel on a random port at connection startup. Variable SOCKS5_PORT will contain the random port."
              {disabled}
            />
          </div>
        </Frame>
      {/if}

      <div class="forwards-section">
        <Notebook
          tabs={forwardTabs}
          bind:activeTab={activeForwardTab}
          position="top"
        >
          {#snippet children(tabId: string)}
            <div class="forwards-body">
              {#if tabId === 'local'}
                <div class="table-list">
                  <div class="table-header">
                    <span class="col-ip">Local IP</span>
                    <span class="col-port">Local Port</span>
                    <span class="col-ip">Remote IP / Host</span>
                    <span class="col-port">Remote Port</span>
                    <span class="col-action"></span>
                  </div>
                  {#each localForwards as fwd, i}
                    <div class="table-row">
                      <Entry bind:value={fwd.BindAddress} placeholder="127.0.0.1" {disabled} onchange={updateSSHForwards} />
                      <SpinButton bind:value={fwd.Port} min={1} max={65535} {disabled} onchange={updateSSHForwards} />
                      <Entry bind:value={fwd.Host} placeholder="Remote host" {disabled} onchange={updateSSHForwards} />
                      <SpinButton bind:value={fwd.HostPort} min={1} max={65535} {disabled} onchange={updateSSHForwards} />
                      <Button icon="gtk-delete" tooltip="Remove forward" {disabled} onclick={() => removeLocalForward(i)} />
                    </div>
                  {/each}
                  <div class="add-row">
                    <Button label="Add" icon="gtk-add" {disabled} onclick={addLocalForward} />
                  </div>
                </div>

              {:else if tabId === 'remote'}
                <div class="table-list">
                  <div class="table-header">
                    <span class="col-ip">Remote IP</span>
                    <span class="col-port">Remote Port</span>
                    <span class="col-ip">Target IP / Host</span>
                    <span class="col-port">Target Port</span>
                    <span class="col-action"></span>
                  </div>
                  {#each remoteForwards as fwd, i}
                    <div class="table-row">
                      <Entry bind:value={fwd.BindAddress} placeholder="127.0.0.1" {disabled} onchange={updateSSHForwards} />
                      <SpinButton bind:value={fwd.Port} min={1} max={65535} {disabled} onchange={updateSSHForwards} />
                      <Entry bind:value={fwd.Host} placeholder="Target host" {disabled} onchange={updateSSHForwards} />
                      <SpinButton bind:value={fwd.HostPort} min={1} max={65535} {disabled} onchange={updateSSHForwards} />
                      <Button icon="gtk-delete" tooltip="Remove forward" {disabled} onclick={() => removeRemoteForward(i)} />
                    </div>
                  {/each}
                  <div class="add-row">
                    <Button label="Add" icon="gtk-add" {disabled} onclick={addRemoteForward} />
                  </div>
                </div>

              {:else if tabId === 'dynamic'}
                <div class="table-list">
                  <div class="table-header">
                    <span class="col-ip">Bind Address</span>
                    <span class="col-port">Port</span>
                    <span class="col-action"></span>
                  </div>
                  {#each dynamicForwards as fwd, i}
                    <div class="table-row">
                      <Entry bind:value={fwd.BindAddress} placeholder="127.0.0.1" {disabled} onchange={updateSSHForwards} />
                      <SpinButton bind:value={fwd.Port} min={1} max={65535} {disabled} onchange={updateSSHForwards} />
                      <Button icon="gtk-delete" tooltip="Remove forward" {disabled} onclick={() => removeDynamicForward(i)} />
                    </div>
                  {/each}
                  <div class="add-row">
                    <Button label="Add" icon="gtk-add" {disabled} onclick={addDynamicForward} />
                  </div>
                </div>

              {:else if tabId === 'advanced'}
                <div class="table-list">
                  <div class="table-header">
                    <span style="flex: 1;">ssh_config Option</span>
                    <span style="flex: 1;">Value</span>
                    <span class="col-action"></span>
                  </div>
                  {#each advOptions as opt, i}
                    <div class="table-row">
                      <ComboBoxText
                        options={supportedSSHOptions}
                        bind:value={opt.key}
                        {disabled}
                        onchange={updateSSHForwards}
                      />
                      <Entry bind:value={opt.val} placeholder="Value" {disabled} onchange={updateSSHForwards} />
                      <Button icon="gtk-delete" tooltip="Remove option" {disabled} onclick={() => removeAdvOption(i)} />
                    </div>
                  {/each}
                  <div class="add-row">
                    <Button label="Add Option" icon="gtk-add" {disabled} onclick={addAdvOption} />
                  </div>
                </div>
              {/if}
            </div>
          {/snippet}
        </Notebook>
      </div>
    {/if}

  {:else if conn.Method === 'telnet'}
    {#if conn.Telnet}
      <div class="telnet-section">
        <Frame label="Telnet Options">
          <div class="col-flex">
            <div class="row-flex">
              <span class="field-label">IP Protocol:</span>
              <ComboBoxText
                options={ipProtocolOptions}
                bind:value={conn.Telnet.IPVersion}
                {disabled}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Bind Address:</span>
              <Entry
                bind:value={conn.Telnet.BindAddress}
                placeholder="Outgoing IP (leave blank for any)"
                tooltip="[-b ip] : Bind outgoing connection to given ip"
                {disabled}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Escape Character:</span>
              <Entry
                bind:value={conn.Telnet.EscapeChar}
                placeholder="^]"
                tooltip="[-e [escape_character]] : Use given string as Escape Character"
                {disabled}
              />
            </div>

            <CheckButton
              bind:checked={conn.Telnet.NAWS}
              label="Send window size (NAWS)"
              {disabled}
            />

            <div class="row-flex">
              <span class="field-label">Terminal type:</span>
              <Entry
                bind:value={conn.Telnet.TerminalType}
                placeholder="XTERM-256COLOR"
                {disabled}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Line ending:</span>
              <ComboBoxText
                options={lineEndingOptions}
                bind:value={conn.Telnet.LineEnding}
                {disabled}
              />
            </div>
          </div>
        </Frame>
      </div>
    {/if}
  {/if}
</div>

<style>
  .options-tab-container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
    box-sizing: border-box;
    overflow-y: auto;
  }

  .top-row {
    margin-bottom: 4px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .field-label {
    min-width: 120px;
    font-size: 0.9em;
    color: var(--gtk-fg-color);
  }

  .other-options-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }

  .forwards-section {
    flex: 1;
    min-height: 220px;
  }

  .forwards-body {
    padding: 6px;
    height: 100%;
    overflow-y: auto;
  }

  .table-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .table-header {
    display: flex;
    gap: 6px;
    padding: 2px 4px;
    font-weight: bold;
    font-size: 0.85em;
    color: var(--gtk-fg-color);
  }

  .table-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 0;
  }

  .col-ip {
    flex: 2;
  }

  .col-port {
    flex: 1;
  }

  .col-action {
    width: 32px;
  }

  .add-row {
    margin-top: 6px;
  }

  .telnet-section {
    max-width: 500px;
  }
</style>
