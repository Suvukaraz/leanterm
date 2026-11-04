<script lang="ts">
  // LeanEdit's settings window (#/editor-prefs): only the editor settings, opened
  // with "Settings" in the editor's menu bar, inside LeanTerm and on its own.
  import { onMount } from 'svelte';
  import * as EditorService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/editorservice.js';
  import { setIconTheme } from '../../lib/theme/icons';
  import EditorOptionsPage from '../prefs/EditorOptionsPage.svelte';
  import Button from '../gtk/Button.svelte';
  import '../../lib/theme/gtk.css';

  let local = $state<any>(null);
  let standalone = $state(false);
  let error = $state('');
  let saving = $state(false);

  onMount(() => {
    document.title = 'LeanEdit Settings';
    (async () => {
      try {
        const s: any = await EditorService.GetEditorSettings();
        document.documentElement.dataset.theme = s?.colorScheme === 'light' ? 'light' : 'dark';
        setIconTheme(s?.iconTheme);
        standalone = !!s?.standalone;
        local = { editor: JSON.parse(JSON.stringify(s?.editor ?? {})) };
      } catch (err: any) {
        error = `The settings could not be loaded: ${err?.message || err}`;
      }
    })();
  });

  async function save(close: boolean) {
    if (!local || saving) return;
    saving = true;
    error = '';
    try {
      await EditorService.SaveEditorSettings(local.editor);
      if (close) await EditorService.ClosePreferences();
    } catch (err: any) {
      error = `The settings could not be saved:\n${err?.message || err}`;
    } finally {
      saving = false;
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && !e.defaultPrevented) {
      e.preventDefault();
      void EditorService.ClosePreferences();
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="editor-prefs">
  <div class="prefs-body">
    {#if local}
      <EditorOptionsPage bind:settings={local} {standalone} />
    {:else if !error}
      <div class="loading">Loading…</div>
    {/if}
  </div>
  {#if error}
    <div class="prefs-error">{error}</div>
  {/if}
  <div class="prefs-footer">
    <Button label="_Cancel" icon="gtk-close" onclick={() => EditorService.ClosePreferences()} />
    <Button label="_Apply" icon="gtk-refresh" disabled={saving || !local} onclick={() => save(false)} />
    <Button label="_OK" icon="gtk-save" isDefault={true} disabled={saving || !local} onclick={() => save(true)} />
  </div>
</div>

<style>
  :global(html),
  :global(body),
  :global(#app) {
    height: 100%;
    margin: 0;
  }

  .editor-prefs {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--gtk-bg-color);
    color: var(--gtk-fg-color);
  }

  .prefs-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  .loading {
    padding: 32px;
    text-align: center;
    color: var(--gtk-muted-fg);
  }

  .prefs-error {
    margin: 0 10px 6px;
    padding: 6px 8px;
    white-space: pre-wrap;
    color: var(--gtk-error-fg, #e06c6c);
    border: 1px solid currentColor;
    border-radius: 4px;
    font-size: 0.9em;
  }

  .prefs-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 8px 10px;
    border-top: 1px solid var(--gtk-border-color);
  }
</style>
