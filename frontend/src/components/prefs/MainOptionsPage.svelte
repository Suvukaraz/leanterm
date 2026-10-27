<script lang="ts">
  import Notebook, { type TabItem } from '../gtk/Notebook.svelte';
  import ComboBoxText from '../gtk/ComboBoxText.svelte';
  import CheckButton from '../gtk/CheckButton.svelte';
  import RadioGroup from '../gtk/RadioGroup.svelte';
  import Frame from '../gtk/Frame.svelte';
  import Entry from '../gtk/Entry.svelte';
  import ColorButton from '../gtk/ColorButton.svelte';
  import FontButton from '../gtk/FontButton.svelte';
  import Button from '../gtk/Button.svelte';
  import { vaultStatus } from '../../lib/stores/config';

  interface Props {
    settings: any;
    onSetMasterPassword?: () => void;
  }

  let { settings = $bindable(), onSetMasterPassword }: Props = $props();

  let activeSubTab = $state('behaviour');

  const subTabs: TabItem[] = [
    { id: 'behaviour', label: 'Behaviour' },
    { id: 'look_and_feel', label: 'Look and Feel' },
    { id: 'advanced', label: 'Advanced' },
  ];

  const layoutOptions = [
    { value: 'traditional', label: 'Traditional' },
    { value: 'compact', label: 'Compact' },
  ];

  const themeOptions = [
    { value: 'default', label: 'default' },
    { value: 'asbru-color', label: 'asbru-color' },
    { value: 'asbru-dark', label: 'asbru-dark' },
    { value: 'system', label: 'system' },
  ];

  const colorSchemeOptions = [
    { value: 'dark', label: 'Dark' },
    { value: 'light', label: 'Light' },
  ];

  const startSizeOptions = [
    { value: 'maximized', label: 'Maximized' },
    { value: 'last', label: 'With last size' },
  ];

  const startTreeOnOptions = [
    { value: 'connections', label: 'Connections' },
    { value: 'favourites', label: 'Favourites' },
    { value: 'history', label: 'History' },
    { value: 'clusters', label: 'Clusters' },
  ];

  const treePositionOptions = [
    { value: 'left', label: 'Left' },
    { value: 'right', label: 'Right' },
  ];

  const buttonBarPositionOptions = [
    { value: 'top', label: 'Top' },
    { value: 'bottom', label: 'Bottom' },
  ];

  const colorTargetOptions = [
    { value: 'foreground', label: 'Foreground' },
    { value: 'background', label: 'Background' },
  ];

  const portForwardFormatOptions = [
    { value: 'full', label: 'Use full names (Local Forwarding, Remote Forwarding, Dynamic Forwarding)' },
    { value: 'short', label: 'Use abreviatures (-L, -R, -D)' },
  ];

  const screenshotViewerOptions = [
    { value: 'internal', label: 'Internal viewer' },
    { value: 'external', label: 'External viewer' },
  ];

  const vaultStatusDisplay = $derived.by(() => {
    switch ($vaultStatus) {
      case 'passphrase':
        return 'Master password (age/scrypt)';
      case 'session-only':
        return 'Not stored (this session only)';
      case 'locked':
        return 'Locked';
      case 'keyring':
      default:
        return 'System keyring';
    }
  });

  const gen = $derived(settings.general);
  const app = $derived(settings.appearance);
  const prompts = $derived(settings.prompts);
  const look = $derived(settings.terminal.Look);

  // Two booleans in the config are shown as one radio group (like the original).
  let startSize = $derived(gen.StartMainMaximized ? 'maximized' : 'last');
  function setStartSize(v: string) {
    gen.StartMainMaximized = v === 'maximized';
    gen.RememberMainSize = v !== 'maximized';
  }

  let forwardFormat = $derived(app.StatusBarForwardsFullNames ? 'full' : 'short');
  function setForwardFormat(v: string) {
    app.StatusBarForwardsFullNames = v === 'full';
  }

  // Unprotected colour is optional ("" = theme default).
  let useUnprotectedColor = $derived(!!app.UnprotectedColor);
  function toggleUnprotectedColor(on: boolean) {
    app.UnprotectedColor = on ? app.UnprotectedColor || '#cccccc' : '';
  }

  const PROMPT_DEFAULTS = {
    command: '(([#%:>~$\\] ])(?!\\g{-1})){3,4}|(\\w[@/]\\w|sftp).*?[#%>~$\\]]|…',
    username: '([lL]ogin|[uU]suario|([uU]ser-?)*[nN]ame.*|[uU]ser)\\s*:\\s*$',
    password: "([pP]ass|[pP]ass[wW]or[dt](\\s+for\\s+|\\w+@[\\w\\-\\.]+)*|[cC]ontrase.a|Enter passphrase for key '.+')\\s*:\\s*$",
    hostkey: '^.+ontinue connecting \\(([^/]+)\\/([^/]+)(?:[^)]+)?\\)\\?\\s*$',
    hostchanged: '.*ffending .*key in (.+?)\\:(\\d+).*',
    anykey: '.*(any key to continue|tecla para continuar).*',
  };
</script>

<div class="pref-page-container">
  <Notebook tabs={subTabs} bind:activeTab={activeSubTab} position="top">
    {#snippet children(tabId: string)}
      <div class="pref-subpage">
        {#if tabId === 'behaviour'}
          <div class="fields-stack">
            <div class="form-grid">
              <span class="field-label">Main Window Layout (*):</span>
              <ComboBoxText options={layoutOptions} bind:value={gen.Layout} />
              <span class="field-label">Colour scheme:</span>
              <ComboBoxText options={colorSchemeOptions} bind:value={gen.ColorScheme} />
              <span class="field-label">Icons Theme (*):</span>
              <ComboBoxText options={themeOptions} bind:value={gen.Theme} />
            </div>

            <Frame label="At start">
              <div class="col-flex">
                <RadioGroup name="start_size" options={startSizeOptions} value={startSize} onchange={setStartSize} />
                <div class="row-flex wrap-row">
                  <CheckButton bind:checked={gen.StartAtLogin} label="Start at session startup" />
                </div>
              </div>
            </Frame>

            <Frame label="At exit">
              <div class="grid-2col">
                <CheckButton
                  bind:checked={gen.ConfirmExit}
                  label="Confirm exit"
                  tooltip="If checked, always ask confirmation before quiting the application."
                />
                <CheckButton bind:checked={gen.SaveOnExit} label="Auto save on exit" />
                <CheckButton bind:checked={gen.AutoSave} label="Automatically save every configuration change" />
              </div>
            </Frame>

            <div class="col-flex">
              <div class="row-flex">
                <CheckButton bind:checked={gen.ProtectWithPassword} label="Protect with password" />
                <Button label="Set..." onclick={onSetMasterPassword} />
              </div>
              <div class="status-vault-line">Passwords are stored in: <b>{vaultStatusDisplay}</b></div>
            </div>

            <div class="col-flex">
              <CheckButton bind:checked={gen.ForceSplit50} label="Force split tabs to 50%" />
              <CheckButton bind:checked={gen.HideOnConnect} label="Hide on connection" />
              <CheckButton
                bind:checked={gen.AutoAcceptHostKeys}
                label="Auto-accept or delete unknown or offending SSH keys"
              />
              <CheckButton
                bind:checked={gen.AppendGroupNameToTitle}
                label="Automatically append group name to title for new connections"
              />
              <CheckButton bind:checked={gen.SortGroupsFirst} label="Sort groups first in the connection tree" />
            </div>

            <div class="footnote-text"><i>(*) Requires restart to take effect</i></div>
          </div>
        {:else if tabId === 'look_and_feel'}
          <div class="fields-stack">
            <Frame label="Start on">
              <RadioGroup name="start_tree_on" options={startTreeOnOptions} bind:value={app.StartTreeOn} />
            </Frame>

            <div class="grid-2col">
              <CheckButton bind:checked={app.ShowScreenshots} label="Show screenshots" />
              <CheckButton bind:checked={app.ShowStatistics} label="Show statistics" />
              <CheckButton
                bind:checked={app.SelectionToClipboard}
                label="Mouse selection copies content to global clipboard"
                tooltip="not just primary clipboard"
              />
              <CheckButton
                bind:checked={gen.AllowMultipleInstances}
                label="Allow more than one instance to run at the same time"
              />
            </div>

            <div class="row-flex">
              <span class="field-label">Show connections on (*):</span>
              <RadioGroup name="tree_position" options={treePositionOptions} bind:value={app.TreePosition} />
            </div>

            <div class="row-flex">
              <span class="field-label">Show main button bar on:</span>
              <RadioGroup name="button_bar_position" options={buttonBarPositionOptions} bind:value={app.ButtonBarPosition} />
            </div>

            <div class="grid-2col">
              <CheckButton bind:checked={app.ShowTreeTooltips} label="Show connections tooltips" />
              <CheckButton bind:checked={app.ShowTreeLines} label="Show lines in connections tree" />
              <CheckButton bind:checked={app.TreeOverlayScrolling} label="Enable overlay scrolling for connections tree" />
              <CheckButton bind:checked={app.ShowTreeTitles} label="Show tree titles" />
            </div>

            <Frame label="Font for">
              <div class="form-grid">
                <span class="field-label">Info tab:</span>
                <FontButton bind:font={app.InfoFont} />
                <span class="field-label">Connection list:</span>
                <FontButton bind:font={app.TreeFont} />
              </div>
            </Frame>

            <Frame label="Color for">
              <div class="col-flex">
                <div class="row-flex wrap-row">
                  <span class="field-label">Protected nodes (*):</span>
                  <RadioGroup name="protected_target" options={colorTargetOptions} bind:value={app.ProtectedColorTarget} />
                  <ColorButton bind:value={app.ProtectedColor} />
                </div>
                <div class="row-flex wrap-row">
                  <span class="field-label">Unprotected nodes (*):</span>
                  <CheckButton checked={useUnprotectedColor} label="custom" onchange={toggleUnprotectedColor} />
                  {#if useUnprotectedColor}
                    <RadioGroup
                      name="unprotected_target"
                      options={colorTargetOptions}
                      bind:value={app.UnprotectedColorTarget}
                    />
                    <ColorButton bind:value={app.UnprotectedColor} />
                  {/if}
                </div>
              </div>
            </Frame>

            <div class="col-flex">
              <CheckButton
                bind:checked={app.AutoHideConnectionsList}
                label="Hide connections list (on terminal open)"
                tooltip="If checked, the connections list will be hidden when a new terminal is open."
              />
              <CheckButton
                bind:checked={app.AutoHideButtonBar}
                label="Auto hide main button bar (on terminal open)"
                tooltip="If checked, this option will hide the the button bar when on a terminal tab (increasing the vertical space usable for the terminal)"
              />
              <CheckButton
                bind:checked={app.ShowTreeOnHover}
                label="Auto show/hide when entering/leaving connections list"
              />
              <CheckButton
                bind:checked={app.TabsInMainWindow}
                label="Put Terminal TABS in Main Window (*)"
                tooltip="If unchecked, every terminal will be opened by default in a separate window"
              />
            </div>

            <Frame label="Port forwarding">
              <div class="col-flex">
                <CheckButton
                  bind:checked={app.StatusBarForwards}
                  label="Show port forwarding information in status bar"
                />
                <RadioGroup
                  name="port_forward_format"
                  options={portForwardFormatOptions}
                  value={forwardFormat}
                  onchange={setForwardFormat}
                  direction="vertical"
                  disabled={!app.StatusBarForwards}
                />
              </div>
            </Frame>
          </div>
        {:else if tabId === 'advanced'}
          <div class="fields-stack">
            <Frame label="Prompts (empty = built-in default)">
              <div class="form-grid wide">
                <span class="field-label">Command prompt:</span>
                <Entry bind:value={prompts.Command} placeholder={PROMPT_DEFAULTS.command} />
                <span class="field-label">USERNAME prompt:</span>
                <Entry bind:value={prompts.Username} placeholder={PROMPT_DEFAULTS.username} />
                <span class="field-label">PASSWORD prompt:</span>
                <Entry bind:value={prompts.Password} placeholder={PROMPT_DEFAULTS.password} />
                <span class="field-label">Host-key verification string:</span>
                <Entry bind:value={prompts.HostKeyVerification} placeholder={PROMPT_DEFAULTS.hostkey} />
                <span class="field-label">Remote host key changed:</span>
                <Entry bind:value={prompts.RemoteHostKeyChanged} placeholder={PROMPT_DEFAULTS.hostchanged} />
                <span class="field-label">Press any key to continue:</span>
                <Entry bind:value={prompts.PressAnyKey} placeholder={PROMPT_DEFAULTS.anykey} />
                <span class="field-label">Select-by-word characters:</span>
                <Entry bind:value={look.WordChars} placeholder="-.:_/" />
              </div>
              <div class="telnet-prompt-hint">
                <i>USERNAME/PASSWORD/Press any key prompts are used for Telnet logins.</i>
              </div>
            </Frame>

            <Frame label="When opening screenshots, use">
              <div class="col-flex">
                <RadioGroup name="screenshot_viewer" options={screenshotViewerOptions} bind:value={app.ScreenshotViewer} />
                <div class="row-flex">
                  <span class="field-label">External viewer:</span>
                  <Entry
                    bind:value={app.ScreenshotViewerCommand}
                    placeholder="xdg-open"
                    disabled={app.ScreenshotViewer !== 'external'}
                    tooltip="The screenshot's file name will be appended at the end of the given command"
                  />
                </div>
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
    padding: 12px;
    height: 100%;
    overflow-y: auto;
  }

  .fields-stack {
    display: flex;
    flex-direction: column;
    gap: 12px;
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

  .grid-2col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px 16px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: max-content minmax(160px, 260px);
    align-items: center;
    gap: 6px 10px;
  }

  .form-grid.wide {
    grid-template-columns: max-content 1fr;
  }

  .form-grid :global(input) {
    width: 100%;
  }

  .field-label {
    font-size: 0.95em;
  }

  .status-vault-line {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
    margin-left: 24px;
  }

  .footnote-text {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
  }

  .telnet-prompt-hint {
    font-size: 0.85em;
    color: var(--gtk-muted-fg);
    margin-top: 6px;
  }
</style>
