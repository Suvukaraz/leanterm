<script lang="ts">
  import { untrack } from 'svelte';
  import { prefsDialogOpen } from '../../lib/stores/ui';
  import { settings, loadSettings, checkUnsaved } from '../../lib/stores/config';
  import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import { sftpPanelWidth } from '../../lib/stores/ui';
  import Dialog from '../gtk/Dialog.svelte';
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import Button from '../gtk/Button.svelte';
  import MessageDialog from '../gtk/MessageDialog.svelte';
  import Entry from '../gtk/Entry.svelte';

  import MainOptionsPage from './MainOptionsPage.svelte';
  import TerminalOptionsPage from './TerminalOptionsPage.svelte';
  import LocalShellPage from './LocalShellPage.svelte';
  import NetworkPage from './NetworkPage.svelte';
  import GlobalVariablesPage from './GlobalVariablesPage.svelte';
  import GlobalCommandsPage from './GlobalCommandsPage.svelte';
  import KeePassPage from './KeePassPage.svelte';
  import KeybindingsPage from './KeybindingsPage.svelte';

  let open = $derived($prefsDialogOpen);

  let localSettings = $state<any>(null);
  let activePage = $state('main');
  let hintDialog = $state<{ open: boolean; message: string }>({ open: false, message: '' });
  let masterPwdDialog = $state<{ open: boolean; oldPwd: string; newPwd: string; confirmPwd: string; error: string }>({
    open: false,
    oldPwd: '',
    newPwd: '',
    confirmPwd: '',
    error: '',
  });

  const preferencePages: TabItem[] = [
    { id: 'main', label: 'Main Options', icon: 'gtk-preferences' },
    { id: 'terminal', label: 'Terminal Options', icon: 'asbru-shell' },
    { id: 'local_shell', label: 'Local Shell Options', icon: 'gtk-execute' },
    { id: 'network', label: 'Network Settings', icon: 'asbru-wol' },
    { id: 'variables', label: 'Global Variables', icon: 'gtk-edit' },
    { id: 'local_commands', label: 'Local Commands', icon: 'gtk-execute' },
    { id: 'remote_commands', label: 'Remote Commands', icon: 'gtk-network' },
    { id: 'keepass', label: 'KeePass Integration', icon: 'asbru-protected' },
    { id: 'keybindings', label: 'Keybindings', icon: 'gtk-properties' },
  ];

  // Work on a copy taken synchronously when the dialog opens, so the window
  // appears fully built (no "Loading..." frame that makes it flicker).
  let wasOpen = false;
  $effect.pre(() => {
    if (open && !wasOpen) {
      wasOpen = true;
      const current = untrack(() => $settings);
      if (current) {
        localSettings = JSON.parse(JSON.stringify(current));
      } else {
        localSettings = null;
        loadSettings().then((s) => {
          localSettings = s ? JSON.parse(JSON.stringify(s)) : null;
        });
      }
    } else if (!open) {
      wasOpen = false;
    }
  });

  function handleClose() {
    prefsDialogOpen.set(false);
  }

  // Options marked (*) only take effect after a restart.
  function restartRelevantChanged(before: any, after: any): boolean {
    const pick = (x: any) =>
      JSON.stringify([
        x?.general?.Layout,
        x?.general?.Theme,
        x?.appearance?.TreePosition,
        x?.appearance?.TabsInMainWindow,
        x?.appearance?.ProtectedColor,
        x?.appearance?.ProtectedColorTarget,
        x?.appearance?.UnprotectedColor,
        x?.appearance?.UnprotectedColorTarget,
        x?.terminal?.Look?.Transparency,
      ]);
    return pick(before) !== pick(after);
  }

  let saving = $state(false);

  async function handleSave() {
    if (!localSettings || saving) return;
    saving = true;
    const before = $settings;
    try {
      await ConfigService.SaveSettings(localSettings);
      if (before?.sftpBrowser?.Width !== localSettings.sftpBrowser?.Width) {
        sftpPanelWidth.set(0); // the new preference wins over a dragged width
      }
      await loadSettings();
      await checkUnsaved();
      handleClose();
      if (restartRelevantChanged(before, localSettings)) {
        hintDialog = {
          open: true,
          message: 'Settings saved. Options marked with (*) take effect after restarting LeanTerm.',
        };
      }
    } catch (err: any) {
      hintDialog = {
        open: true,
        message: `The settings could not be saved:\n\n${err?.message || err}`,
      };
    } finally {
      saving = false;
    }
  }

  async function handleResetDefaults() {
    try {
      await ConfigService.ResetSettings();
      await loadSettings();
      if ($settings) {
        localSettings = JSON.parse(JSON.stringify($settings));
      }
    } catch (err: any) {
      hintDialog = { open: true, message: `Reset failed: ${err?.message || err}` };
    }
  }

  async function handleExport() {
    try {
      const path = await SystemService.ChooseSaveFile('Export LeanTerm configuration', 'leanterm-export.toml');
      if (!path) return;
      await ConfigService.ExportFull(path, false, false, '');
      hintDialog = { open: true, message: `Configuration exported to\n${path}` };
    } catch (err: any) {
      hintDialog = { open: true, message: `Export failed: ${err?.message || err}` };
    }
  }

  function openMasterPasswordSetup() {
    masterPwdDialog = {
      open: true,
      oldPwd: '',
      newPwd: '',
      confirmPwd: '',
      error: '',
    };
  }

  async function saveMasterPassword() {
    if (masterPwdDialog.newPwd !== masterPwdDialog.confirmPwd) {
      masterPwdDialog.error = 'New passwords do not match!';
      return;
    }
    if (masterPwdDialog.newPwd.length < 4) {
      masterPwdDialog.error = 'Password must be at least 4 characters long.';
      return;
    }
    try {
      await ConfigService.SetMasterPassword(masterPwdDialog.oldPwd, masterPwdDialog.newPwd);
      masterPwdDialog.open = false;
      if (localSettings?.general) {
        localSettings.general.ProtectWithPassword = true;
      }
    } catch (err: any) {
      masterPwdDialog.error = `Failed to set password: ${err?.message || err}`;
    }
  }
</script>

{#if open}
  <Dialog
    title="Preferences"
    bannerText="Preferences"
    width={880}
    height={660}
    {open}
    onclose={handleClose}
  >
    {#snippet children()}
      <div class="preferences-window-body">
        {#if !localSettings}
          <div class="loading-state">Loading settings...</div>
        {:else}
          <div class="prefs-notebook-container">
            <Notebook
              tabs={preferencePages}
              bind:activeTab={activePage}
              position="left"
            >
              {#snippet children(tabId: string)}
                <div class="prefs-page-content">
                  {#if tabId === 'main'}
                    <MainOptionsPage
                      bind:settings={localSettings}
                      onSetMasterPassword={openMasterPasswordSetup}
                    />
                  {:else if tabId === 'terminal'}
                    <TerminalOptionsPage bind:settings={localSettings} />
                  {:else if tabId === 'local_shell'}
                    <LocalShellPage bind:settings={localSettings} />
                  {:else if tabId === 'network'}
                    <NetworkPage bind:settings={localSettings} />
                  {:else if tabId === 'variables'}
                    <GlobalVariablesPage bind:settings={localSettings} />
                  {:else if tabId === 'local_commands'}
                    <GlobalCommandsPage bind:settings={localSettings} isRemote={false} />
                  {:else if tabId === 'remote_commands'}
                    <GlobalCommandsPage bind:settings={localSettings} isRemote={true} />
                  {:else if tabId === 'keepass'}
                    <KeePassPage bind:settings={localSettings} />
                  {:else if tabId === 'keybindings'}
                    <KeybindingsPage bind:settings={localSettings} />
                  {/if}
                </div>
              {/snippet}
            </Notebook>
          </div>
        {/if}
      </div>
    {/snippet}

    {#snippet actions()}
      <div class="prefs-footer">
        <div class="footer-left">
          <Button
            label="Reset to defaults"
            icon="gtk-undo"
            onclick={handleResetDefaults}
          />
          <Button
            label="Export…"
            icon="gtk-save-as"
            onclick={handleExport}
          />
        </div>
        <div class="footer-right">
          <Button
            label="_Close without saving"
            icon="gtk-cancel"
            onclick={handleClose}
          />
          <Button
            label="_Save and Close"
            icon="gtk-save"
            isDefault={true}
            disabled={saving || !localSettings}
            onclick={handleSave}
          />
        </div>
      </div>
    {/snippet}
  </Dialog>
{/if}

{#if hintDialog.open}
  <MessageDialog
    title="Preferences"
    secondaryText={hintDialog.message}
    type="info"
    buttons="ok"
    onresponse={() => (hintDialog.open = false)}
  />
{/if}

{#if masterPwdDialog.open}
  <Dialog
    title="Set Master Password"
    bannerText="Set Master Password"
    width={400}
    open={masterPwdDialog.open}
    onclose={() => (masterPwdDialog.open = false)}
  >
    {#snippet children()}
      <div class="pwd-dialog-body">
        {#if masterPwdDialog.error}
          <div class="pwd-error">{masterPwdDialog.error}</div>
        {/if}
        <div class="col-flex">
          <label>
            Current Master Password (leave empty if not set):
            <Entry
              type="password"
              bind:value={masterPwdDialog.oldPwd}
              placeholder="Old password"
            />
          </label>
          <label>
            New Master Password:
            <Entry
              type="password"
              bind:value={masterPwdDialog.newPwd}
              placeholder="New password"
            />
          </label>
          <label>
            Confirm New Master Password:
            <Entry
              type="password"
              bind:value={masterPwdDialog.confirmPwd}
              placeholder="Confirm new password"
            />
          </label>
        </div>
      </div>
    {/snippet}
    {#snippet actions()}
      <Button label="Cancel" onclick={() => (masterPwdDialog.open = false)} />
      <Button label="Save Password" isDefault={true} onclick={saveMasterPassword} />
    {/snippet}
  </Dialog>
{/if}

<style>
  .preferences-window-body {
    height: 100%;
    min-height: 520px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .loading-state {
    padding: 32px;
    text-align: center;
    color: var(--gtk-muted-fg);
    font-style: italic;
  }

  .prefs-notebook-container {
    flex: 1;
    display: flex;
    min-height: 0;
    overflow: hidden;
  }

  .prefs-page-content {
    height: 100%;
    overflow-y: auto;
  }

  .prefs-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }

  .footer-left {
    display: flex;
    gap: 8px;
  }

  .footer-right {
    display: flex;
    gap: 8px;
  }

  .pwd-dialog-body {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .pwd-error {
    background: var(--gtk-error-bg);
    color: var(--gtk-error-fg);
    padding: 6px 10px;
    border-radius: 3px;
    font-size: 0.85em;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .col-flex label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.9em;
  }
</style>
