<script lang="ts">
  import CheckButton from '../gtk/CheckButton.svelte';
  import Frame from '../gtk/Frame.svelte';
  import FontButton from '../gtk/FontButton.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';

  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  const editor = $derived(settings.editor);

  const openInOptions = [
    { value: 'editor', label: 'LeanTerm Editor' },
    { value: 'default_app', label: 'Default application' },
  ];
</script>

<div class="editor-options-page">
  <Frame label="Font">
    <div class="row-flex">
      <span class="field-label">Editor font:</span>
      <FontButton bind:font={editor.Font} monospace />
    </div>
  </Frame>

  <Frame label="Opening files">
    <div class="col-flex">
      <div class="row-flex">
        <span class="field-label">Double-click on a text file opens it in:</span>
        <RadioGroup name="editor_open_in" options={openInOptions} bind:value={editor.OpenTextFilesIn} />
      </div>
      <span class="hint-text">
        Applies to the SFTP and the local file browser. "Open in Editor" and "Open with default application" stay
        available in the context menu.
      </span>
    </div>
  </Frame>

  <Frame label="Editing">
    <div class="col-flex">
      <div class="row-flex">
        <span class="field-label">Tab width:</span>
        <SpinButton bind:value={editor.TabWidth} min={1} max={16} />
      </div>
      <CheckButton bind:checked={editor.IndentWithTabs} label="Indent with tabs instead of spaces" />
      <CheckButton bind:checked={editor.WordWrap} label="Wrap long lines" />
      <CheckButton bind:checked={editor.ShowWhitespace} label="Show spaces and tabs" />
      <CheckButton bind:checked={editor.ShowMinimap} label="Show scrollbar minimap" />
    </div>
  </Frame>

  <Frame label="Large files">
    <div class="col-flex">
      <div class="row-flex">
        <span class="field-label">Ask once above:</span>
        <SpinButton bind:value={editor.LargeFileMB} min={1} max={100000} />
        <span>MB</span>
      </div>
      <div class="row-flex">
        <span class="field-label">Ask twice above:</span>
        <SpinButton bind:value={editor.HugeFileMB} min={1} max={100000} />
        <span>MB</span>
      </div>
      <span class="hint-text">
        There is no size limit. Files above the second limit open without syntax highlighting (View → Syntax
        Highlighting turns it on).
      </span>
    </div>
  </Frame>
</div>

<style>
  .editor-options-page {
    padding: 10px;
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-sizing: border-box;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .field-label {
    min-width: 130px;
    font-size: 0.9em;
  }

  .hint-text {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
  }
</style>
