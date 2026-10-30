<script lang="ts">
  import { aboutDialogOpen } from '../../lib/stores/ui';
  import Dialog from '../gtk/Dialog.svelte';
  import Button from '../gtk/Button.svelte';
  import { getIcon } from '../../lib/theme/icons';

  function close() {
    aboutDialogOpen.set(false);
  }
</script>

<Dialog
  open={$aboutDialogOpen}
  title="About LeanTerm"
  width={460}
  movable
  resizable
  minWidth={360}
  minHeight={220}
  geometryKey="about"
  onclose={close}
>
  <div class="about-container">
    <div class="logo-area">
      <img src={getIcon('asbru-app-big')} alt="LeanTerm Logo" class="about-logo" />
    </div>
    <div class="about-info">
      <h2 class="app-title">LeanTerm</h2>
      <div class="app-version">Version 1.0.0</div>
      <p class="app-desc">
        Next-generation Connection Manager, modern rewrite of Ásbrú Connection Manager
        built with Go, Wails v3, and Svelte 5.
      </p>
      <div class="app-license">
        Licensed under the <b>GNU General Public License v3 (GPLv3)</b>
      </div>
    </div>
  </div>

  {#snippet actions()}
    <Button label="Close" icon="gtk-close" isDefault={true} onclick={close} />
  {/snippet}
</Dialog>

<style>
  .about-container {
    display: flex;
    gap: 16px;
    align-items: center;
    padding: 10px 0;
  }
  .about-logo {
    width: 64px;
    height: 64px;
    object-fit: contain;
  }
  .about-info {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    line-height: 1.4;
  }
  .app-title {
    margin: 0;
    font-size: 18px;
    font-weight: bold;
    color: var(--text-color, #2e3436);
  }
  .app-version {
    color: var(--text-muted, #888);
    font-size: 11px;
  }
  .app-desc {
    margin: 0;
  }
  .app-license {
    font-size: 11px;
    color: var(--text-muted, #666);
  }
</style>
