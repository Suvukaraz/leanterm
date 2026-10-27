<script lang="ts">
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import { editConnDialog } from '../../lib/stores/ui';
  import { openConnections } from '../../lib/stores/sessions';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
  import * as models from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/models.js';

  let open = $derived($editConnDialog.open && $editConnDialog.isQuick);

  let method = $state<'ssh' | 'telnet' | 'sftp'>('ssh');
  let title = $state('Quick Connect');
  let host = $state('');
  let port = $state(22);
  let user = $state('');
  let password = $state('');
  let connecting = $state(false);
  let errorMsg = $state('');

  $effect(() => {
    if (open) {
      method = 'ssh';
      title = 'Quick Connect';
      host = '';
      port = 22;
      user = '';
      password = '';
      connecting = false;
      errorMsg = '';
    }
  });

  function handleMethodChange(e: Event) {
    const val = (e.target as HTMLSelectElement).value as 'ssh' | 'telnet' | 'sftp';
    method = val;
    if (val === 'telnet') {
      if (port === 22) port = 23;
    } else {
      if (port === 23) port = 22;
    }
  }

  function handleClose() {
    editConnDialog.set({ open: false, connId: '', isQuick: false });
  }

  async function handleConnect() {
    if (!host.trim()) {
      errorMsg = 'Please enter a target host.';
      return;
    }

    connecting = true;
    errorMsg = '';

    try {
      const quickID = `quick-${Date.now()}`;
      // In-memory quick connection request
      const req = new models.OpenRequest({
        connIDs: [quickID],
        where: 'tab',
        instances: 1,
      });

      // Construct connection parameters for open
      await SessionService.OpenQuick(
        quickID,
        title.trim() || host.trim(),
        method,
        host.trim(),
        port,
        user.trim(),
        password
      );

      handleClose();
    } catch (err: any) {
      errorMsg = err?.message || String(err);
      connecting = false;
    }
  }
</script>

<Dialog
  {open}
  title="Quick Connect"
  bannerText="Connect to a remote server without saving to configuration"
  width={420}
  onclose={handleClose}
>
  <div class="quick-connect-form">
    {#if errorMsg}
      <div class="error-banner">{errorMsg}</div>
    {/if}

    <div class="form-row">
      <label for="qc-method"><b>Method:</b></label>
      <select id="qc-method" class="form-select" value={method} onchange={handleMethodChange}>
        <option value="ssh">SSH</option>
        <option value="telnet">Telnet</option>
        <option value="sftp">SFTP Browser</option>
      </select>
    </div>

    <div class="form-row">
      <label for="qc-host"><b>Host:</b></label>
      <input
        id="qc-host"
        type="text"
        class="form-input"
        placeholder="e.g. 192.168.1.100 or server.example.com"
        bind:value={host}
        onkeydown={(e) => e.key === 'Enter' && handleConnect()}
      />
    </div>

    <div class="form-row">
      <label for="qc-port"><b>Port:</b></label>
      <input
        id="qc-port"
        type="number"
        class="form-input port-input"
        min="1"
        max="65535"
        bind:value={port}
        onkeydown={(e) => e.key === 'Enter' && handleConnect()}
      />
    </div>

    <div class="form-row">
      <label for="qc-user"><b>User:</b></label>
      <input
        id="qc-user"
        type="text"
        class="form-input"
        placeholder="Username"
        bind:value={user}
        onkeydown={(e) => e.key === 'Enter' && handleConnect()}
      />
    </div>

    <div class="form-row">
      <label for="qc-password"><b>Password:</b></label>
      <input
        id="qc-password"
        type="password"
        class="form-input"
        placeholder="Password (kept in memory only)"
        bind:value={password}
        onkeydown={(e) => e.key === 'Enter' && handleConnect()}
      />
    </div>

    <div class="form-row">
      <label for="qc-title">Title (optional):</label>
      <input
        id="qc-title"
        type="text"
        class="form-input"
        placeholder="Display title for tab"
        bind:value={title}
        onkeydown={(e) => e.key === 'Enter' && handleConnect()}
      />
    </div>
  </div>

  {#snippet actions()}
    <Button label="Close" onclick={handleClose} />
    <Button
      label={connecting ? 'Connecting...' : 'Connect'}
      isDefault
      disabled={connecting}
      onclick={handleConnect}
    />
  {/snippet}
</Dialog>

<style>
  .quick-connect-form {
    display: flex;
    flex-direction: column;
    gap: 10px;
    font-size: 12px;
    color: var(--gtk-fg, #2e3436);
  }

  .error-banner {
    background: var(--gtk-error-bg);
    border: 1px solid var(--gtk-error-border);
    color: var(--gtk-error-fg);
    padding: 6px 10px;
    border-radius: 3px;
    font-size: 11px;
  }

  .form-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .form-row label {
    width: 100px;
    flex-shrink: 0;
  }

  .form-input,
  .form-select {
    flex: 1;
    font-size: 12px;
    padding: 4px 8px;
    background: var(--gtk-base, #ffffff);
    border: 1px solid var(--gtk-border, #c0c0c0);
    border-radius: 3px;
    color: var(--gtk-fg, #2e3436);
    outline: none;
  }

  .form-input:focus,
  .form-select:focus {
    border-color: var(--gtk-selected-bg, #4a90d9);
  }

  .port-input {
    max-width: 100px;
  }
</style>
