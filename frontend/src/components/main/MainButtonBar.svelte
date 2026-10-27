<script lang="ts">
  import { settings } from '../../lib/stores/config';
  import {
    commandPanelVisible,
    clustersVisible,
    sftpVisible,
    aboutDialogOpen,
    prefsDialogOpen,
    wolDialog,
    isLocked,
  } from '../../lib/stores/ui';
  import { openLocalShell } from '../../lib/stores/sessions';
  import Button from '../gtk/Button.svelte';
  import ToggleButton from '../gtk/ToggleButton.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  let { position = 'top' }: { position?: 'top' | 'bottom' } = $props();

  let protectWithPassword = $derived(
    $settings?.general?.ProtectWithPassword ?? false
  );

  function handleLockToggle() {
    if (protectWithPassword) {
      if ($isLocked) {
        // Unlock prompt will be shown in dialog
        isLocked.set(false);
      } else {
        isLocked.set(true);
        SystemService.LockGUI();
      }
    }
  }
</script>

<div class="main-button-bar" class:at-top={position === 'top'} role="toolbar" aria-label="Main Button Bar">
  <ToggleButton
    checked={$commandPanelVisible}
    icon="asbru-treelist"
    label="Connections"
    tooltip="Show/Hide connections tree panel (F9)"
    onchange={(v) => commandPanelVisible.set(v)}
  />

  <ToggleButton
    checked={$sftpVisible}
    icon="asbru-method-sftp"
    label="SFTP"
    tooltip="Show/Hide the SFTP browser of the terminals"
    onchange={(v) => sftpVisible.set(v)}
  />

  <ToggleButton
    checked={$clustersVisible}
    icon="asbru-cluster-manager2"
    label="Clusters"
    tooltip="Show/Hide the cluster functions"
    onchange={(v) => clustersVisible.set(v)}
  />

  <Button
    label="Local shell"
    icon="asbru-shell"
    tooltip="Launch new local shell (Ctrl+Alt+L)"
    onclick={() => openLocalShell('tab')}
  />

  <Button
    label="Wake On Lan"
    icon="asbru-wol"
    tooltip="Start the 'Wake On LAN' utility window"
    onclick={() => wolDialog.update((s) => ({ ...s, open: true }))}
  />

  <Button
    label="Preferences"
    icon="gtk-preferences"
    tooltip="Open the general preferences control"
    onclick={() => prefsDialogOpen.set(true)}
  />

  <ToggleButton
    checked={$isLocked}
    icon={$isLocked ? 'asbru-protected' : 'asbru-unprotected'}
    tooltip="Password [un]lock GUI. In order to use this functionality, check the 'Protect with password' field under 'Preferences'->'Main Options'"
    disabled={!protectWithPassword}
    onchange={handleLockToggle}
  />

  <Button
    label="About"
    icon="gtk-about"
    tooltip="Show the *so needed* 'About' dialog"
    onclick={() => aboutDialogOpen.set(true)}
  />
</div>

<style>
  .main-button-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    background: linear-gradient(to bottom, var(--gtk-bg-color), var(--gtk-header-bg));
    border-top: 1px solid var(--gtk-border-color, #ccc);
    user-select: none;
    overflow-x: auto;
  }
  .main-button-bar.at-top {
    border-top: none;
    border-bottom: 1px solid var(--gtk-border-color, #ccc);
  }
</style>
