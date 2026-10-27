<script lang="ts">
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Button from '../gtk/Button.svelte';
  import FileChooserButton from '../gtk/FileChooserButton.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  interface Props {
    conn: any;
    disabled?: boolean;
    onShowCmdLine?: () => void;
  }

  let {
    conn = $bindable(),
    disabled = false,
    onShowCmdLine
  }: Props = $props();

  let activeDetailTab = $state('host');

  const detailTabs: TabItem[] = [
    { id: 'host', label: 'Host' },
    { id: 'auth', label: 'Authentication' },
    { id: 'details', label: 'Connection Details' },
    { id: 'logs', label: 'Session Logs' },
    { id: 'advanced', label: 'Advanced Parameters' },
    { id: 'network', label: 'Network Settings' },
  ];

  let showPassword = $state(false);
  let showPassphrase = $state(false);
  let showSocksPassword = $state(false);
  let showJumpPassword = $state(false);

  const methodOptions = [
    { value: 'ssh', label: 'SSH' },
    { value: 'telnet', label: 'Telnet' },
    { value: 'sftp', label: 'SFTP (file browser only)' },
  ];

  const authOptions = [
    { value: 'password', label: 'User / Password ' },
    { value: 'publickey', label: 'Private Key ' },
    { value: 'manual', label: 'Manual' },
  ];

  const networkModes = [
    { value: 'global', label: 'Use global proxy options' },
    { value: 'direct', label: 'Use a direct connection' },
    { value: 'socks', label: 'Use a SOCKS proxy' },
    { value: 'jump', label: 'Use a Jump Host' },
  ];

  const sftpBrowserOptions = [
    { value: 'global', label: 'Use global setting' },
    { value: 'open', label: 'Open automatically' },
    { value: 'closed', label: 'Do not open automatically' },
  ];

  function handleMethodChange(newMethod: string) {
    const oldMethod = conn.Method || 'ssh';
    if (newMethod !== oldMethod) {
      if (newMethod === 'telnet') {
        if (conn.Port === 22) conn.Port = 23;
        if (conn.Auth && conn.Auth.Type === 'publickey') {
          conn.Auth.Type = 'password';
        }
      } else {
        if (conn.Port === 23) conn.Port = 22;
      }
      conn.Method = newMethod;
    }
  }


  function openLogFolder() {
    const folder = conn.SessionLogs?.Folder || '';
    if (folder) {
      SystemService.OpenPath(folder);
    }
  }
</script>

<div class="connection-tab-container">
  <Notebook
    tabs={detailTabs}
    bind:activeTab={activeDetailTab}
    position="top"
  >
    {#snippet children(tabId: string)}
      <div class="inner-tab-body">
        {#if tabId === 'host'}
          <div class="section-group">
            <div class="row-flex">
              <span class="field-label">Method:</span>
              <ComboBoxText
                options={methodOptions}
                value={conn.Method || 'ssh'}
                {disabled}
                onchange={handleMethodChange}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Host:</span>
              <Entry
                bind:value={conn.Host}
                placeholder="IP or Hostname"
                tooltip="IP or Hostname of the machine to connect to"
                {disabled}
              />
              <span class="field-label" style="margin-left: 12px;">Port:</span>
              <SpinButton
                bind:value={conn.Port}
                min={1}
                max={65535}
                tooltip="Port number"
                {disabled}
              />
            </div>

            <div class="row-flex">
              <span class="field-label">TAB / Window Title:</span>
              <Entry
                bind:value={conn.Title}
                placeholder="TAB/Window title"
                tooltip="TAB/Window title to use with this connection"
                {disabled}
              />
            </div>

            <div class="row-flex wrap-gap">
              <CheckButton
                bind:checked={conn.LaunchAtStartup}
                label="Launch at startup"
                tooltip="If checked, this session will be executed upon startup"
                {disabled}
              />
              <CheckButton
                bind:checked={conn.AutoReconnect}
                label="Reconnect on disconnection"
                tooltip="If checked, reconnect this session when disconnection happens with exit status <> 0"
                {disabled}
              />
              <div class="sub-row">
                <span>Wait</span>
                <SpinButton
                  bind:value={conn.SendDelayMs}
                  min={0}
                  max={5000}
                  step={5}
                  {disabled}
                />
                <span>millisecs for automated char sending</span>
              </div>
            </div>
          </div>

        {:else if tabId === 'auth'}
          <div class="section-group">
            <RadioGroup
              name="auth_type"
              options={conn.Method === 'telnet' ? authOptions.filter(o => o.value !== 'publickey') : authOptions}
              value={conn.Auth?.Type || 'password'}
              direction="vertical"
              {disabled}
              onchange={(val) => {
                if (conn.Auth) conn.Auth.Type = val;
              }}
            />

            {#if conn.Auth?.Type === 'password'}
              <Frame label="User / Password">
                <div class="row-flex">
                  <span class="field-label">User:</span>
                  <Entry
                    bind:value={conn.Auth.User}
                    placeholder="Username"
                    tooltip="Login username"
                    {disabled}
                  />
                  <span class="field-label" style="margin-left: 12px;">Password:</span>
                  <Entry
                    type={showPassword ? 'text' : 'password'}
                    bind:value={conn.Password}
                    placeholder="Password"
                    tooltip="Login password"
                    {disabled}
                  />
                  <CheckButton
                    bind:checked={showPassword}
                    label="Show"
                    tooltip="Show/hide password"
                  />
                </div>
              </Frame>
            {:else if conn.Auth?.Type === 'publickey'}
              <Frame label="Private Key">
                <div class="col-flex">
                  <div class="row-flex">
                    <span class="field-label">Private key file:</span>
                    <FileChooserButton
                      bind:path={conn.Auth.KeyFile}
                      title="Select Private Key File"
                      {disabled}
                    />
                    <Button
                      label="Clear"
                      tooltip="Clear selected private key"
                      {disabled}
                      onclick={() => {
                        if (conn.Auth) conn.Auth.KeyFile = '';
                      }}
                    />
                  </div>
                  <div class="row-flex">
                    <span class="field-label">User:</span>
                    <Entry
                      bind:value={conn.Auth.KeyUser}
                      placeholder="Username for key"
                      tooltip="Username to use for connecting"
                      {disabled}
                    />
                    <span class="field-label" style="margin-left: 12px;">Passphrase:</span>
                    <Entry
                      type={showPassphrase ? 'text' : 'password'}
                      bind:value={conn.KeyPassphrase}
                      placeholder="Passphrase"
                      tooltip="Enter here the passphrase (if needed) for the given private key"
                      {disabled}
                    />
                    <CheckButton
                      bind:checked={showPassphrase}
                      label="Show"
                      tooltip="Show/hide passphrase"
                    />
                  </div>
                </div>
              </Frame>
            {:else if conn.Auth?.Type === 'manual'}
              <div class="manual-auth-hint">
                <i>Username and password will be requested interactively upon connecting.</i>
              </div>
            {/if}

            {#if conn.Method !== 'telnet'}
              <div style="margin-top: 10px;">
                <CheckButton
                  bind:checked={conn.Auth.Exclusive}
                  label='Use selected auth method exclusively (no fallback nor "Advanced Options" auth bypass)'
                  tooltip="Checking this may improve login speed on some systems, but may break compatibility with SSH agent, Seahorse, ..."
                  {disabled}
                />
              </div>
            {/if}
          </div>

        {:else if tabId === 'details'}
          <div class="section-group">
            <Frame label="Programmed String Sending">
              <div class="col-flex">
                <div class="row-flex">
                  <CheckButton
                    bind:checked={conn.SendString.Enabled}
                    label="Programatically send string"
                    {disabled}
                  />
                  <Entry
                    bind:value={conn.SendString.Text}
                    placeholder="String to send"
                    disabled={disabled || !conn.SendString.Enabled}
                  />
                </div>
                <div class="row-flex">
                  <CheckButton
                    bind:checked={conn.SendString.AppendReturn}
                    label="With <RETURN>"
                    disabled={disabled || !conn.SendString.Enabled}
                  />
                  <span style="margin-left: 12px;">Every (secs):</span>
                  <SpinButton
                    bind:value={conn.SendString.Interval}
                    min={1}
                    max={3600}
                    disabled={disabled || !conn.SendString.Enabled}
                  />
                  <CheckButton
                    bind:checked={conn.SendString.OnlyWhenIdle}
                    label="Idle Only"
                    tooltip="Restart the timer each time the terminal is updated or focused, effectively preventing the string from being sent unless the terminal has been idle for the configured period."
                    disabled={disabled || !conn.SendString.Enabled}
                  />
                </div>
              </div>
            </Frame>
          </div>

        {:else if tabId === 'logs'}
          <div class="section-group">
            <Frame label="Auto save Session logs">
              <div class="col-flex">
                <CheckButton
                  bind:checked={conn.SessionLogs.Enabled}
                  label="Enable session logs"
                  {disabled}
                />
                <div class="row-flex">
                  <CheckButton
                    bind:checked={conn.SessionLogs.StripControlChars}
                    label="Remove CONTROL characters"
                    disabled={disabled || !conn.SessionLogs.Enabled}
                  />
                  <CheckButton
                    bind:checked={conn.SessionLogs.Timestamps}
                    label="Add timestamp"
                    disabled={disabled || !conn.SessionLogs.Enabled}
                  />
                </div>
                <div class="row-flex">
                  <span>Files to save (0=infinite):</span>
                  <SpinButton
                    bind:value={conn.SessionLogs.Keep}
                    min={0}
                    max={999}
                    disabled={disabled || !conn.SessionLogs.Enabled}
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Save in:</span>
                  <Entry
                    bind:value={conn.SessionLogs.Folder}
                    placeholder="Log folder path"
                    disabled={disabled || !conn.SessionLogs.Enabled}
                  />
                  <Button
                    label="Open folder"
                    tooltip="Open folder in system file manager"
                    onclick={openLogFolder}
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Logfile name pattern:</span>
                  <Entry
                    bind:value={conn.SessionLogs.Pattern}
                    placeholder="<DATE_Y><DATE_M><DATE_D>_<NAME>.log"
                    disabled={disabled || !conn.SessionLogs.Enabled}
                  />
                </div>
              </div>
            </Frame>
          </div>

        {:else if tabId === 'advanced'}
          <div class="section-group">
            <div class="row-flex">
              <span class="field-label">Connection UUID:</span>
              <Entry
                value={conn.ID}
                readonly={true}
                tooltip="Unique connection identifier"
              />
              <Button
                label="Get Command line"
                tooltip="Show equivalent command line"
                onclick={onShowCmdLine}
              />
            </div>

            {#if conn.Method === 'ssh'}
              <div class="row-flex" style="margin-top: 10px;">
                <span class="field-label">SFTP browser:</span>
                <ComboBoxText
                  options={sftpBrowserOptions}
                  value={conn.SFTPBrowser || 'global'}
                  {disabled}
                  onchange={(val) => {
                    conn.SFTPBrowser = val;
                  }}
                />
              </div>
              <div class="row-flex">
                <span class="field-label">SFTP start folder:</span>
                <Entry
                  bind:value={conn.SFTPStartDir}
                  placeholder="(login directory)"
                  tooltip="Initial directory when SFTP panel opens"
                  {disabled}
                />
              </div>
            {/if}
          </div>

        {:else if tabId === 'network'}
          <div class="section-group">
            <div class="row-flex">
              <RadioGroup
                name="network_mode"
                options={networkModes}
                value={conn.Network?.Mode || 'global'}
                direction="vertical"
                {disabled}
                onchange={(val) => {
                  if (conn.Network) conn.Network.Mode = val;
                }}
              />
            </div>

            {#if conn.Network?.Mode === 'socks'}
              <Frame label="SOCKS Proxy Settings">
                <div class="col-flex">
                  <div class="row-flex">
                    <span class="field-label">IP / Host:</span>
                    <Entry
                      bind:value={conn.Network.Socks.Host}
                      placeholder="Proxy host"
                      {disabled}
                    />
                    <span class="field-label" style="margin-left: 12px;">Port:</span>
                    <SpinButton
                      bind:value={conn.Network.Socks.Port}
                      min={1}
                      max={65535}
                      {disabled}
                    />
                  </div>
                  <div class="row-flex">
                    <span class="field-label">User:</span>
                    <Entry
                      bind:value={conn.Network.Socks.User}
                      placeholder="Proxy username"
                      {disabled}
                    />
                    <span class="field-label" style="margin-left: 12px;">Password:</span>
                    <Entry
                      type={showSocksPassword ? 'text' : 'password'}
                      bind:value={conn.SocksPassword}
                      placeholder="Proxy password"
                      {disabled}
                    />
                    <CheckButton
                      bind:checked={showSocksPassword}
                      label="Show"
                    />
                  </div>
                </div>
              </Frame>
            {:else if conn.Network?.Mode === 'jump'}
              <Frame label="Jump Host Settings">
                <div class="col-flex">
                  <div class="row-flex">
                    <span class="field-label">IP / Host:</span>
                    <Entry
                      bind:value={conn.Network.Jump.Host}
                      placeholder="Jump host"
                      {disabled}
                    />
                    <span class="field-label" style="margin-left: 12px;">Port:</span>
                    <SpinButton
                      bind:value={conn.Network.Jump.Port}
                      min={1}
                      max={65535}
                      {disabled}
                    />
                  </div>
                  <div class="row-flex">
                    <span class="field-label">User:</span>
                    <Entry
                      bind:value={conn.Network.Jump.User}
                      placeholder="Jump username"
                      {disabled}
                    />
                    <span class="field-label" style="margin-left: 12px;">Password:</span>
                    <Entry
                      type={showJumpPassword ? 'text' : 'password'}
                      bind:value={conn.JumpPassword}
                      placeholder="Jump password"
                      {disabled}
                    />
                    <CheckButton
                      bind:checked={showJumpPassword}
                      label="Show"
                    />
                  </div>
                  <div class="row-flex">
                    <span class="field-label">Private key file:</span>
                    <FileChooserButton
                      bind:path={conn.Network.Jump.KeyFile}
                      title="Select Jump Host Private Key"
                      {disabled}
                    />
                    <Button
                      label="Clear"
                      tooltip="Remove any selected private key"
                      {disabled}
                      onclick={() => {
                        if (conn.Network) conn.Network.Jump.KeyFile = '';
                      }}
                    />
                  </div>
                  {#if conn.Method === 'ssh'}
                    <CheckButton
                      bind:checked={conn.Network.PseudoJump}
                      label="Pseudo jump server"
                      tooltip="Only open the shell on the jump host"
                      {disabled}
                    />
                  {/if}
                </div>
              </Frame>
            {/if}
          </div>
        {/if}
      </div>
    {/snippet}
  </Notebook>
</div>

<style>
  .connection-tab-container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .inner-tab-body {
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    height: 100%;
    overflow-y: auto;
  }

  .section-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .wrap-gap {
    flex-wrap: wrap;
    gap: 12px;
  }

  .sub-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .field-label {
    min-width: 90px;
    font-size: 0.9em;
    color: var(--gtk-fg-color);
  }

  .manual-auth-hint {
    padding: 12px;
    background: var(--gtk-row-hover);
    border-radius: 4px;
    font-size: 0.9em;
  }
</style>
