<script lang="ts">
  // Quitting the application (window X, compact-layout buttons): confirms running
  // SFTP transfers, open connections or (if configured) every exit first.
  import { onMount } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { settings } from '../../lib/stores/config';
  import { sessions } from '../../lib/stores/sessions';
  import { hasRunningTransfers } from '../../lib/stores/sftp';
  import { quitRequested } from '../../lib/stores/ui';
  import MessageDialog from '../gtk/MessageDialog.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  let quitConfirm = $state<{ open: boolean; message: string; subtext: string }>({
    open: false,
    message: '',
    subtext: '',
  });

  function handleQuit() {
    if (quitConfirm.open) return;
    if (hasRunningTransfers()) {
      quitConfirm = {
        open: true,
        message: 'Active File Transfers in Progress',
        subtext: 'There are active SFTP transfers in progress. Do you want to cancel the transfers and quit?',
      };
      return;
    }
    if ($sessions.length > 0) {
      quitConfirm = {
        open: true,
        message: 'Active Connections Open',
        subtext: `There are ${$sessions.length} active connection(s) open. Are you sure you want to disconnect and quit?`,
      };
      return;
    }
    if ($settings?.general?.ConfirmExit) {
      quitConfirm = {
        open: true,
        message: 'Quit LeanTerm',
        subtext: 'Are you sure you want to close LeanTerm?',
      };
      return;
    }
    SystemService.Quit(false);
  }

  onMount(() => {
    const unsubStore = quitRequested.subscribe((n) => {
      if (n > 0) handleQuit();
    });
    const unsubEvent = Events.On('app:close-requested', () => quitRequested.update((n) => n + 1));
    return () => {
      unsubStore();
      unsubEvent();
    };
  });
</script>

{#if quitConfirm.open}
  <MessageDialog
    title="Confirm Exit"
    primaryText={quitConfirm.message}
    secondaryText={quitConfirm.subtext}
    type="warning"
    buttons="yes_no"
    onresponse={(res) => {
      quitConfirm.open = false;
      if (res === 'yes') {
        SystemService.Quit(true);
      }
    }}
  />
{/if}
