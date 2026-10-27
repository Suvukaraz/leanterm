<script lang="ts">
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Entry from '../gtk/Entry.svelte';
  import SpinButton from '../gtk/SpinButton.svelte';
  import ColorButton from '../gtk/ColorButton.svelte';
  import FontButton from '../gtk/FontButton.svelte';
  import Scale from '../gtk/Scale.svelte';
  import Button from '../gtk/Button.svelte';
  import FileChooserButton from '../gtk/FileChooserButton.svelte';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';

  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  let activeSubTab = $state('behaviour');

  const subTabs: TabItem[] = [
    { id: 'behaviour', label: 'Behaviour' },
    { id: 'look_and_feel', label: 'Look and Feel' },
    { id: 'advanced', label: 'Advanced' },
    { id: 'sftp_browser', label: 'SFTP browser' },
  ];

  const whenLastTabClosedOptions = [
    { value: 'nothing', label: 'Do nothing' },
    { value: 'close', label: 'Close' },
    { value: 'hide', label: 'Hide' },
  ];

  const openNewInOptions = [
    { value: 'tab', label: 'Tabs' },
    { value: 'window', label: 'Windows' },
  ];

  const macrosBarOptions = [
    { value: 'never', label: 'Never' },
    { value: 'combo', label: 'In combo box' },
    { value: 'buttons', label: 'As buttons' },
  ];

  const sftpPositionOptions = [
    { value: 'left', label: 'Left' },
    { value: 'right', label: 'Right' },
  ];

  const tabsPositionOptions = [
    { value: 'top', label: 'Top' },
    { value: 'bottom', label: 'Bottom' },
    { value: 'left', label: 'Left' },
    { value: 'right', label: 'Right' },
  ];

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

  const term = $derived(settings.terminal);
  const look = $derived(settings.terminal.Look);
  const sftp = $derived(settings.sftpBrowser);

  // Default 16-color ANSI palette if not present
  const defaultPalette = [
    '#2e3436', '#cc0000', '#4e9a06', '#c4a000', '#3465a4', '#75507b', '#06989a', '#d3d7cf',
    '#555753', '#ef2929', '#8ae234', '#fce94f', '#729fcf', '#ad7fa8', '#34e2e2', '#eeeeec'
  ];

  $effect(() => {
    if (!look.Palette || look.Palette.length < 16) {
      look.Palette = [...defaultPalette];
    }
  });

  function openLogsFolder() {
    if (term.SessionLogs?.Folder) {
      SystemService.OpenPath(term.SessionLogs.Folder);
    }
  }
</script>

<div class="pref-page-container">
  <Notebook
    tabs={subTabs}
    bind:activeTab={activeSubTab}
    position="top"
  >
    {#snippet children(tabId: string)}
      <div class="pref-subpage">
        {#if tabId === 'behaviour'}
          <div class="fields-stack">
            <CheckButton
              bind:checked={term.CloseOnDisconnect}
              label="Close Terminal on disconnection: close container (TAB or Window) when connection is closed"
            />
            <CheckButton
              bind:checked={term.UnsplitDisconnected}
              label="Auto unsplit disconnected Terminals"
            />

            <Frame label="Timeouts (seconds)">
              <div class="row-flex">
                <span class="field-label">Time to connect:</span>
                <SpinButton
                  bind:value={term.TimeoutConnect}
                  min={0}
                  max={3600}
                  tooltip="0 = infinite"
                />
                <span class="field-label" style="margin-left: 16px;">Expect Cmd exec:</span>
                <SpinButton
                  bind:value={term.TimeoutCommand}
                  min={0}
                  max={3600}
                />
              </div>
            </Frame>

            <Frame label="Session Logs">
              <div class="col-flex">
                <CheckButton
                  bind:checked={term.SessionLogs.Enabled}
                  label="Enable session logs"
                />
                <div class="row-flex wrap-row">
                  <CheckButton
                    bind:checked={term.SessionLogs.StripControlChars}
                    label="Remove CONTROL characters"
                    disabled={!term.SessionLogs.Enabled}
                  />
                  <CheckButton
                    bind:checked={term.SessionLogs.Timestamps}
                    label="Add timestamp"
                    disabled={!term.SessionLogs.Enabled}
                  />
                </div>
                <div class="row-flex">
                  <span>Files to save (0=infinite):</span>
                  <SpinButton
                    bind:value={term.SessionLogs.Keep}
                    min={0}
                    max={999}
                    disabled={!term.SessionLogs.Enabled}
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Save in:</span>
                  <Entry
                    bind:value={term.SessionLogs.Folder}
                    placeholder="Session logs folder"
                    disabled={!term.SessionLogs.Enabled}
                  />
                  <Button
                    label="Open Folder"
                    disabled={!term.SessionLogs.Enabled}
                    onclick={openLogsFolder}
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Logfile name pattern:</span>
                  <Entry
                    bind:value={term.SessionLogs.Pattern}
                    placeholder="<DATE_Y><DATE_M><DATE_D>_<NAME>.log"
                    disabled={!term.SessionLogs.Enabled}
                  />
                </div>
              </div>
            </Frame>

            <Frame label="When closing last TAB">
              <RadioGroup
                name="when_last_tab_closed"
                options={whenLastTabClosedOptions}
                bind:value={term.WhenLastTabClosed}
              />
            </Frame>

            <Frame label="Open NEW connections on">
              <div class="row-flex">
                <RadioGroup
                  name="open_new_in"
                  options={openNewInOptions}
                  bind:value={term.OpenNewIn}
                />
                <span class="field-label" style="margin-left: 16px;">Width:</span>
                <SpinButton
                  bind:value={term.WindowWidth}
                  min={200}
                  max={4000}
                  tooltip="Size in Pixels"
                />
                <span class="field-label" style="margin-left: 12px;">Height:</span>
                <SpinButton
                  bind:value={term.WindowHeight}
                  min={150}
                  max={3000}
                  tooltip="Size in Pixels"
                />
              </div>
            </Frame>
          </div>

        {:else if tabId === 'look_and_feel'}
          <div class="fields-stack">
            <CheckButton
              bind:checked={term.StartMaximized}
              label="Start Terminals maximized: maximize either TABBED or OWN window"
            />
            <CheckButton
              bind:checked={term.ShowStatusBar}
              label="Show status bar on every terminal"
            />
            <CheckButton
              bind:checked={term.ChangeMainTitle}
              label="Change main window title with terminal title"
            />

            <Frame label="Show MACROS">
              <RadioGroup
                name="macros_bar"
                options={macrosBarOptions}
                bind:value={term.MacrosBar}
              />
              <CheckButton
                bind:checked={term.ShowGlobalMacros}
                label="Show GLOBAL commands in terminals"
              />
            </Frame>

            <Frame label="Tab Label Status Colors">
              <div class="row-flex wrap-row">
                <span class="field-label">Connected color:</span>
                <ColorButton bind:value={term.Colors.Connected} />
                <span class="field-label" style="margin-left: 12px;">Disconnected color:</span>
                <ColorButton bind:value={term.Colors.Disconnected} />
                <span class="field-label" style="margin-left: 12px;">New data color:</span>
                <ColorButton bind:value={term.Colors.NewData} />
              </div>
            </Frame>

            <Frame label="Tabs">
              <div class="row-flex">
                <span class="field-label">Tabs position:</span>
                <RadioGroup
                  name="tabs_position"
                  options={tabsPositionOptions}
                  bind:value={term.TabsPosition}
                />
              </div>
            </Frame>

            <Frame label="Terminal UI">
              <div class="col-flex">
                <div class="row-flex wrap-row">
                  <span class="field-label">Text color:</span>
                  <ColorButton bind:value={look.Foreground} />
                  <span class="field-label" style="margin-left: 12px;">Back color:</span>
                  <ColorButton bind:value={look.Background} />
                  <span class="field-label" style="margin-left: 12px;">Bold Color:</span>
                  <ColorButton bind:value={look.Bold} />
                </div>
                <div class="row-flex wrap-row">
                  <span>Bold is:</span>
                  <CheckButton
                    bind:checked={look.BoldIsBright}
                    label="also bright"
                  />
                  <CheckButton
                    bind:checked={look.BoldLikeText}
                    label="same as text"
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Font:</span>
                  <FontButton bind:font={look.Font} />
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
                    tooltip="positive = length, 0 = disabled, -1 = unlimited"
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Transparency (*):</span>
                  <Scale
                    bind:value={look.Transparency}
                    min={0}
                    max={1}
                    step={0.05}
                  />
                </div>

                <div class="col-flex" style="margin-top: 6px;">
                  <span style="font-weight: bold; font-size: 0.9em;">16-Color ANSI Palette:</span>
                  <div class="palette-grid">
                    {#if look.Palette}
                      {#each look.Palette as col, i}
                        <div class="palette-swatch">
                          <ColorButton bind:value={look.Palette[i]} />
                          <span class="palette-idx">{i < 8 ? i : `B${i - 8}`}</span>
                        </div>
                      {/each}
                    {/if}
                  </div>
                </div>
              </div>
            </Frame>
          </div>

        {:else if tabId === 'advanced'}
          <div class="fields-stack">
            <div class="row-flex">
              <span class="field-label">Character encoding:</span>
              <ComboBoxText
                options={encodingOptions}
                bind:value={look.Encoding}
              />
            </div>
            <div class="row-flex">
              <span class="field-label">Backspace:</span>
              <ComboBoxText
                options={backspaceOptions}
                bind:value={look.Backspace}
              />
            </div>

            <CheckButton
              bind:checked={term.Debug}
              label="Expect DEBUG"
              tooltip="Log expect pattern matching details"
            />
            <CheckButton
              bind:checked={term.HideConnectionsSubmenu}
              label="Hide Connections submenu"
            />
            <CheckButton
              bind:checked={term.AudibleBell}
              label="Audible Bell"
            />
            <CheckButton
              bind:checked={term.ConfirmChains}
              label="Confirm Chains selection / execution"
            />
            <CheckButton
              bind:checked={term.SkipFirstChainExpect}
              label="Skip first Chain expect"
            />
          </div>

        {:else if tabId === 'sftp_browser'}
          <div class="fields-stack">
            <Frame label="Behaviour">
              <div class="col-flex">
                <CheckButton
                  bind:checked={sftp.AutoOpen}
                  label="Open SFTP browser automatically on SSH connections"
                />
                <CheckButton
                  bind:checked={sftp.FollowTerminalFolder}
                  label="Follow terminal folder (OSC 7)"
                  tooltip="Change the SFTP browser folder when the shell's working directory changes (requires the remote shell to report it, see Help)"
                />
                <CheckButton
                  bind:checked={sftp.ShowHidden}
                  label="Show hidden files"
                />
                <CheckButton
                  bind:checked={sftp.ConfirmDelete}
                  label="Confirm before deleting"
                />
              </div>
            </Frame>

            <Frame label="Transfers">
              <div class="col-flex">
                <CheckButton
                  bind:checked={sftp.PreserveTimestamps}
                  label="Preserve timestamps"
                />
                <CheckButton
                  bind:checked={sftp.PreservePermissions}
                  label="Preserve permissions"
                />
                <CheckButton
                  bind:checked={sftp.UsePartFiles}
                  label="Write to temporary .part files and rename when complete"
                />
                <div class="row-flex">
                  <span class="field-label">Parallel transfers:</span>
                  <SpinButton
                    bind:value={sftp.ParallelTransfers}
                    min={1}
                    max={8}
                  />
                </div>
                <div class="row-flex">
                  <span class="field-label">Default download folder:</span>
                  <FileChooserButton
                    bind:path={sftp.DownloadDir}
                    mode="folder"
                    title="Choose Default Download Folder"
                  />
                  <Entry
                    bind:value={sftp.DownloadDir}
                    placeholder="(empty = XDG Downloads)"
                  />
                </div>
              </div>
            </Frame>

            <Frame label="Look and Feel">
              <div class="row-flex">
                <span class="field-label">Panel width:</span>
                <SpinButton
                  bind:value={sftp.Width}
                  min={200}
                  max={1200}
                  step={10}
                />
                <span>px</span>
              </div>
              <div class="row-flex">
                <span class="field-label">Show panel on:</span>
                <RadioGroup name="sftp_position" options={sftpPositionOptions} bind:value={sftp.Position} />
              </div>
            </Frame>
          </div>
        {/if}
      </div>
    {/snippet}
  </Notebook>
</div>

<style>
  .pref-page-container {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .pref-subpage {
    padding: 10px;
    height: 100%;
    overflow-y: auto;
    box-sizing: border-box;
  }

  .fields-stack {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .row-flex {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .col-flex {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .wrap-row {
    flex-wrap: wrap;
    gap: 10px;
  }

  .field-label {
    min-width: 140px;
    font-size: 0.9em;
  }

  .palette-grid {
    display: grid;
    grid-template-columns: repeat(8, 1fr);
    gap: 6px;
    margin-top: 4px;
  }

  .palette-swatch {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .palette-idx {
    font-size: 0.75em;
    color: var(--gtk-muted-fg);
  }
</style>
