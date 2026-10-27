<script lang="ts">
  import type { Settings } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/models.js';
  import Frame from '../gtk/Frame.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';

  interface Props {
    settings: Settings;
  }

  let { settings = $bindable() }: Props = $props();

  if (!settings.keepass) {
    settings.keepass = {
      Enabled: false,
      Database: '',
      KeyFile: '',
    } as any;
  }
</script>

<div class="keepass-page">
  <Frame label="KeePassXC Integration">
    <div class="form-grid">
      <div class="form-row full-width">
        <CheckButton
          label="Enable KeePassXC integration"
          bind:checked={settings.keepass.Enabled}
        />
      </div>

      <div class="form-row">
        <label for="kp-db" class="form-label">Database (.kdbx):</label>
        <div class="form-input">
          <Entry
            bind:value={settings.keepass.Database}
            placeholder="/path/to/database.kdbx"
          />
        </div>
      </div>

      <div class="form-row">
        <label for="kp-key" class="form-label">Key File (optional):</label>
        <div class="form-input">
          <Entry
            bind:value={settings.keepass.KeyFile}
            placeholder="/path/to/database.key"
          />
        </div>
      </div>

      <div class="info-note">
        <b>Usage in Connections & Macros:</b>
        <p>Use the mask <code>&lt;field|entry&gt;</code> in host, user, password, macros, or expect strings.</p>
        <p>Example: <code>&lt;Password|Root/Servers/web1&gt;</code> or <code>&lt;UserName|Root/Servers/web1&gt;</code>.</p>
        <p>Requires <code>keepassxc-cli</code> installed in your system PATH.</p>
      </div>
    </div>
  </Frame>
</div>

<style>
  .keepass-page {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 6px;
  }

  .form-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .form-row.full-width {
    width: 100%;
  }

  .form-label {
    width: 140px;
    font-size: 12px;
    color: var(--gtk-fg, #2e3436);
  }

  .form-input {
    flex: 1;
  }

  .info-note {
    margin-top: 10px;
    padding: 10px;
    background: var(--gtk-header-bg, #f0f0f0);
    border: 1px solid var(--gtk-border, #d3d7cf);
    border-radius: 4px;
    font-size: 11px;
    color: var(--gtk-fg, #333);
    line-height: 1.4;
  }

  .info-note code {
    background: var(--gtk-header-bg);
    padding: 2px 4px;
    border-radius: 3px;
    font-family: monospace;
    font-size: 11px;
  }
</style>
