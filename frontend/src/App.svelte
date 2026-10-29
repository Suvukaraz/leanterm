<script lang="ts">
  import { onMount } from 'svelte';
  import MainWindow from './components/main/MainWindow.svelte';
  import { loadTree, loadSettings, checkUnsaved, settings } from './lib/stores/config';
  import { refreshSessions, initSessionEvents } from './lib/stores/sessions';
  import { loadUIState } from './lib/stores/uistate';
  import { activeLeftTab, prefsDialogOpen, prefsStartPage } from './lib/stores/ui';
  import { Events } from '@wailsio/runtime';
  import { setIconTheme } from './lib/theme/icons';
  import './lib/theme/gtk.css';

  let initialized = $state(false);

  // Colour scheme follows the preference live; the icon theme needs a restart (as in the original).
  $effect(() => {
    const scheme = $settings?.general?.ColorScheme === 'light' ? 'light' : 'dark';
    document.documentElement.dataset.theme = scheme;
  });

  onMount(() => {
    const stopEvents = initSessionEvents();
    // "Settings → Configure Editor…" in the editor window.
    const stopConfigure = Events.On('editor:configure', () => {
      prefsStartPage.set('editor');
      prefsDialogOpen.set(true);
    });

    (async () => {
      try {
        const [s] = await Promise.all([
          loadSettings(),
          loadTree(),
          refreshSessions(),
          checkUnsaved(),
          loadUIState(),
        ]);
        setIconTheme(s?.general?.Theme);
        const startOn = s?.appearance?.StartTreeOn;
        if (startOn === 'connections' || startOn === 'favourites' || startOn === 'history' || startOn === 'clusters') {
          activeLeftTab.set(startOn);
        }
      } catch (err) {
        console.error('Failed to initialize app state:', err);
      } finally {
        initialized = true;
      }
    })();

    return () => {
      stopEvents();
      stopConfigure();
    };
  });
</script>

{#if initialized}
  <MainWindow />
{:else}
  <div class="app-loading-screen">
    <div class="loading-spinner"></div>
    <span class="loading-label">Loading LeanTerm...</span>
  </div>
{/if}

<style>
  .app-loading-screen {
    width: 100vw;
    height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    background: var(--gtk-bg-color);
    color: var(--gtk-muted-fg);
  }
  .loading-spinner {
    width: 28px;
    height: 28px;
    border: 3px solid var(--gtk-border-color);
    border-top-color: var(--gtk-accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .loading-label {
    font-size: 13px;
  }
</style>
