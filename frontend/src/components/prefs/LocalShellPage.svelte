<script lang="ts">
  import CheckButton from '../gtk/CheckButton.svelte';
  import Entry from '../gtk/Entry.svelte';
  import Frame from '../gtk/Frame.svelte';
  import ColorButton from '../gtk/ColorButton.svelte';
  import FontButton from '../gtk/FontButton.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';

  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  const shell = $derived(settings.localShell);

  const cursorShapeOptions = [
    { value: 'block', label: 'block' },
    { value: 'ibeam', label: 'ibeam' },
    { value: 'underline', label: 'underline' },
  ];

  // The Local Shell uses the global terminal look unless it has its own copy.
  let usePersonal = $derived(!!shell.Look);
  function togglePersonal(on: boolean) {
    shell.Look = on ? JSON.parse(JSON.stringify(settings.terminal.Look)) : null;
  }

  const look = $derived(shell.Look);
</script>

<div class="local-shell-page">
  <Frame label="Local Shell Launch Settings">
    <div class="col-flex">
      <CheckButton
        bind:checked={shell.Autostart}
        label="Start a shell upon startup"
      />
      <div class="row-flex">
        <span class="field-label">Shell Binary:</span>
        <Entry
          bind:value={shell.Binary}
          placeholder="(empty = $SHELL)"
        />
      </div>
      <div class="row-flex">
        <span class="field-label">Shell Options:</span>
        <Entry
          bind:value={shell.Options}
          placeholder="e.g. -l"
        />
      </div>
      <div class="row-flex">
        <span class="field-label">Initial Directory:</span>
        <Entry
          bind:value={shell.Directory}
          placeholder="(empty = $HOME)"
        />
      </div>
    </div>
  </Frame>

  <CheckButton
    checked={usePersonal}
    label="Use these personal options (otherwise the global Terminal UI settings apply)"
    onchange={togglePersonal}
  />

  {#if look}
    <Frame label="Terminal Look Override">
      <div class="col-flex">
        <div class="row-flex wrap-row">
          <span class="field-label">Text color:</span>
          <ColorButton bind:value={look.Foreground} />
          <span class="field-label" style="margin-left: 12px;">Back color:</span>
          <ColorButton bind:value={look.Background} />
          <span class="field-label" style="margin-left: 12px;">Bold Color:</span>
          <ColorButton bind:value={look.Bold} />
        </div>
        <div class="row-flex">
          <span class="field-label">Font:</span>
          <FontButton bind:font={look.Font} monospace />
        </div>
        <div class="row-flex wrap-row">
          <span class="field-label">Cursor shape:</span>
          <ComboBoxText
            options={cursorShapeOptions}
            bind:value={look.CursorShape}
          />
          <span class="field-label" style="margin-left: 16px;">Scrollback lines:</span>
          <SpinButton
            bind:value={look.ScrollbackLines}
            min={-1}
            max={100000}
          />
        </div>
      </div>
    </Frame>
  {/if}
</div>

<style>
  .local-shell-page {
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

  .wrap-row {
    flex-wrap: wrap;
    gap: 8px;
  }

  .field-label {
    min-width: 130px;
    font-size: 0.9em;
  }
</style>
