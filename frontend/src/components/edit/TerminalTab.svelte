<script lang="ts">
  import CheckButton from '../gtk/CheckButton.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import ColorButton from '../gtk/ColorButton.svelte';
  import FontButton from '../gtk/FontButton.svelte';
  import Scale from '../gtk/Scale.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Button from '../gtk/Button.svelte';
  import { settings } from '../../lib/stores/config';

  interface Props {
    conn: any;
    disabled?: boolean;
  }

  let { conn = $bindable(), disabled = false }: Props = $props();

  // Ensure Terminal sub-objects exist
  $effect(() => {
    if (!conn.Terminal) {
      conn.Terminal = {
        UsePersonal: false,
        OpenIn: 'tab',
        WindowWidth: 800,
        WindowHeight: 600,
        TimeoutConnect: 0,
        TimeoutCommand: 10,
        TabColor: '',
        Prompts: {
          Command: '',
          User: '',
          Password: '',
          HostKeyVerification: '',
          RemoteHostKeyChanged: '',
          PressAnyKey: '',
        },
        Look: null,
      };
    }
    if (!conn.Terminal.Prompts) {
      conn.Terminal.Prompts = {
        Command: '',
        User: '',
        Password: '',
        HostKeyVerification: '',
        RemoteHostKeyChanged: '',
        PressAnyKey: '',
      };
    }
  });

  const cursorShapeOptions = [
    { value: 'block', label: 'block' },
    { value: 'ibeam', label: 'ibeam' },
    { value: 'underline', label: 'underline' },
  ];

  const backspaceOptions = [
    { value: 'auto', label: 'auto' },
    { value: 'ascii-backspace', label: 'ascii-backspace (^H)' },
    { value: 'ascii-delete', label: 'ascii-delete (^?)' },
    { value: 'delete-sequence', label: 'delete-sequence (ESC [3~)' },
    { value: 'tty', label: 'tty' },
  ];

  const encodingOptions = [
    { value: 'UTF-8', label: 'UTF-8' },
    { value: 'ISO-8859-1', label: 'ISO-8859-1 (Latin 1)' },
    { value: 'ISO-8859-15', label: 'ISO-8859-15' },
    { value: 'Windows-1252', label: 'Windows-1252' },
    { value: 'ASCII', label: 'ASCII' },
  ];

  const openInOptions = [
    { value: 'tab', label: 'Tab' },
    { value: 'window', label: 'Window' },
  ];

  function setUsePersonal(val: boolean) {
    if (conn.Terminal) {
      conn.Terminal.UsePersonal = val;
      if (val && !conn.Terminal.Look) {
        conn.Terminal.Look = JSON.parse(JSON.stringify($settings?.terminal?.Look || {
          Foreground: '#ffffff',
          Background: '#000000',
          Bold: '#ffffff',
          BoldLikeText: true,
          BoldIsBright: true,
          CursorShape: 'block',
          ScrollbackLines: 5000,
          Transparency: 0.0,
          WordChars: '-A-Za-z0-9_.~:/?#[]@!$&\'()*+,;=',
          Encoding: 'UTF-8',
          Backspace: 'auto',
          Font: { Family: 'Monospace', Size: 10 },
        }));
      }
    }
  }

  function resetToDefaults() {
    if (conn.Terminal) {
      conn.Terminal.TimeoutConnect = 0;
      conn.Terminal.TimeoutCommand = 10;
      conn.Terminal.TabColor = '';
      conn.Terminal.OpenIn = 'tab';
      conn.Terminal.WindowWidth = 800;
      conn.Terminal.WindowHeight = 600;
      conn.Terminal.Prompts = {
        Command: '',
        User: '',
        Password: '',
        HostKeyVerification: '',
        RemoteHostKeyChanged: '',
        PressAnyKey: '',
      };
      conn.Terminal.Look = JSON.parse(JSON.stringify($settings?.terminal?.Look || {
        Foreground: '#ffffff',
        Background: '#000000',
        Bold: '#ffffff',
        BoldLikeText: true,
        BoldIsBright: true,
        CursorShape: 'block',
        ScrollbackLines: 5000,
        Transparency: 0.0,
        WordChars: '-A-Za-z0-9_.~:/?#[]@!$&\'()*+,;=',
        Encoding: 'UTF-8',
        Backspace: 'auto',
        Font: { Family: 'Monospace', Size: 10 },
      }));
    }
  }

  // Safe helper look object when Look is null
  let dummyLook = $state({
    Foreground: '#ffffff',
    Background: '#000000',
    Bold: '#ffffff',
    BoldLikeText: true,
    BoldIsBright: true,
    CursorShape: 'block',
    ScrollbackLines: 5000,
    Transparency: 0.0,
    WordChars: '',
    Encoding: 'UTF-8',
    Backspace: 'auto',
    Font: { Family: 'Monospace', Size: 10 },
  });
</script>

<div class="terminal-tab-container">
  <div class="top-unlock-bar">
    <CheckButton
      checked={Boolean(conn.Terminal?.UsePersonal)}
      label="Use these personal options"
      {disabled}
      onchange={setUsePersonal}
    />
    <Button
      label="Reset to DEFAULT values"
      icon="gtk-undo"
      disabled={disabled || !conn.Terminal?.UsePersonal}
      onclick={resetToDefaults}
    />
  </div>

  <div class="options-body" class:disabled-overlay={!conn.Terminal?.UsePersonal || disabled}>
    <Frame label="Prompt RegExp">
      <div class="col-flex">
        <div class="row-flex">
          <span class="field-label">Command prompt:</span>
          <Entry
            bind:value={conn.Terminal.Prompts.Command}
            placeholder="Default: (?!\g{-1})[\$#>]\s*$"
            disabled={!conn.Terminal?.UsePersonal || disabled}
          />
        </div>
        <div class="row-flex">
          <span class="field-label">Username RegExp:</span>
          <Entry
            bind:value={conn.Terminal.Prompts.User}
            placeholder="Default: (user(name)?|login)\s*:\s*$"
            disabled={!conn.Terminal?.UsePersonal || disabled}
          />
        </div>
        <div class="row-flex">
          <span class="field-label">Password RegExp:</span>
          <Entry
            bind:value={conn.Terminal.Prompts.Password}
            placeholder="Default: (password|passphrase)\s*:\s*$"
            disabled={!conn.Terminal?.UsePersonal || disabled}
          />
        </div>
      </div>
    </Frame>

    <Frame label="Terminal UI">
      <div class="col-flex">
        {#if conn.Terminal?.Look}
          <div class="row-flex wrap-row">
            <span class="field-label">Text color:</span>
            <ColorButton
              bind:value={conn.Terminal.Look.Foreground}
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
            <span class="field-label" style="margin-left: 12px;">Back color:</span>
            <ColorButton
              bind:value={conn.Terminal.Look.Background}
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
            <CheckButton
              bind:checked={conn.Terminal.Look.BoldLikeText}
              label="Bold color like Text color"
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
          </div>

          <div class="row-flex">
            <span class="field-label">Font:</span>
            <FontButton
              bind:font={conn.Terminal.Look.Font}
              disabled={!conn.Terminal?.UsePersonal || disabled}
              monospace
            />
          </div>

          <div class="row-flex">
            <span class="field-label">Tab background color:</span>
            <ColorButton
              bind:value={conn.Terminal.TabColor}
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
          </div>

          <div class="row-flex wrap-row">
            <span class="field-label">Cursor Shape:</span>
            <ComboBoxText
              options={cursorShapeOptions}
              bind:value={conn.Terminal.Look.CursorShape}
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
            <span class="field-label" style="margin-left: 12px;">Scrollback lines:</span>
            <SpinButton
              bind:value={conn.Terminal.Look.ScrollbackLines}
              min={-1}
              max={100000}
              tooltip="positive = length, 0 = disabled, -1 = unlimited"
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
          </div>

          <div class="row-flex">
            <span class="field-label">Transparency:</span>
            <Scale
              bind:value={conn.Terminal.Look.Transparency}
              min={0}
              max={1}
              step={0.05}
              disabled={!conn.Terminal?.UsePersonal || disabled}
            />
          </div>
        {/if}
      </div>
    </Frame>

    {#if conn.Terminal?.Look}
      <Frame label="Select Word CHARS">
        <Entry
          bind:value={conn.Terminal.Look.WordChars}
          placeholder="-A-Za-z0-9_.~:/?#[]@!$&'()*+,;="
          disabled={!conn.Terminal?.UsePersonal || disabled}
        />
      </Frame>
    {/if}

    <Frame label="Time outs (seconds)">
      <div class="row-flex">
        <span class="field-label">Connection:</span>
        <SpinButton
          bind:value={conn.Terminal.TimeoutConnect}
          min={0}
          max={3600}
          tooltip="Set to 0 (zero) to wait forever until connection is established"
          disabled={!conn.Terminal?.UsePersonal || disabled}
        />
        <span class="field-label" style="margin-left: 16px;">Expect Cmd exec:</span>
        <SpinButton
          bind:value={conn.Terminal.TimeoutCommand}
          min={0}
          max={3600}
          disabled={!conn.Terminal?.UsePersonal || disabled}
        />
      </div>
    </Frame>

    <Frame label="Open NEW connection on">
      <div class="row-flex">
        <RadioGroup
          name="open_new_conn"
          options={openInOptions}
          value={conn.Terminal.OpenIn}
          disabled={!conn.Terminal?.UsePersonal || disabled}
          onchange={(val) => {
            conn.Terminal.OpenIn = val;
          }}
        />
        <span class="field-label" style="margin-left: 16px;">Width:</span>
        <SpinButton
          bind:value={conn.Terminal.WindowWidth}
          min={200}
          max={4000}
          disabled={!conn.Terminal?.UsePersonal || disabled}
        />
        <span class="field-label" style="margin-left: 12px;">Height:</span>
        <SpinButton
          bind:value={conn.Terminal.WindowHeight}
          min={150}
          max={3000}
          disabled={!conn.Terminal?.UsePersonal || disabled}
        />
      </div>
    </Frame>

    {#if conn.Terminal?.Look}
      <Frame label="Terminal Emulation">
        <div class="row-flex">
          <span class="field-label">Backspace binding:</span>
          <ComboBoxText
            options={backspaceOptions}
            bind:value={conn.Terminal.Look.Backspace}
            disabled={!conn.Terminal?.UsePersonal || disabled}
          />
          <span class="field-label" style="margin-left: 16px;">Character Encoding:</span>
          <ComboBoxText
            options={encodingOptions}
            bind:value={conn.Terminal.Look.Encoding}
            disabled={!conn.Terminal?.UsePersonal || disabled}
          />
        </div>
      </Frame>
    {/if}
  </div>
</div>

<style>
  .terminal-tab-container {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
    overflow-y: auto;
  }

  .top-unlock-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 4px;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .options-body {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .disabled-overlay {
    opacity: 0.55;
    pointer-events: none;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 6px;
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
    min-width: 140px;
    font-size: 0.9em;
  }
</style>
