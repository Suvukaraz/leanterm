<script lang="ts">
  
  interface Props {
    settings: any;
  }

  let { settings = $bindable() }: Props = $props();

  interface KeybindingRow {
    window: string;
    action: string;
    description: string;
    defaultKey: string;
    currentKey: string;
  }

  const defaultKeybindings: KeybindingRow[] = [
    { window: 'main', action: 'find', description: 'Find connection', defaultKey: 'Ctrl+f', currentKey: 'Ctrl+f' },
    { window: 'main', action: 'quit', description: 'Quit application', defaultKey: 'Ctrl+q', currentKey: 'Ctrl+q' },
    { window: 'main', action: 'localshell', description: 'Open local shell', defaultKey: 'Ctrl+Shift+T', currentKey: 'Ctrl+Shift+T' },
    { window: 'main', action: 'showconnections', description: 'Toggle connections tree', defaultKey: 'Alt+n', currentKey: 'Alt+n' },

    { window: 'tree_connections', action: 'edit_node', description: 'Edit node', defaultKey: 'Alt+e', currentKey: 'Alt+e' },
    { window: 'tree_connections', action: 'add_favourite', description: 'Add favourite', defaultKey: 'Alt+f', currentKey: 'Alt+f' },
    { window: 'tree_connections', action: 'del_favourite', description: 'Remove favourite', defaultKey: 'Alt+Shift+F', currentKey: 'Alt+Shift+F' },
    { window: 'tree_connections', action: 'find', description: 'Find in tree', defaultKey: 'Ctrl+f', currentKey: 'Ctrl+f' },
    { window: 'tree_connections', action: 'expand_all', description: 'Expand all', defaultKey: 'Ctrl+r', currentKey: 'Ctrl+r' },
    { window: 'tree_connections', action: 'collapse_all', description: 'Collapse all', defaultKey: 'Ctrl+t', currentKey: 'Ctrl+t' },
    { window: 'tree_connections', action: 'clone', description: 'Clone connection', defaultKey: 'Ctrl+d', currentKey: 'Ctrl+d' },
    { window: 'tree_connections', action: 'copy', description: 'Copy node', defaultKey: 'Ctrl+c', currentKey: 'Ctrl+c' },
    { window: 'tree_connections', action: 'cut', description: 'Cut node', defaultKey: 'Ctrl+x', currentKey: 'Ctrl+x' },
    { window: 'tree_connections', action: 'paste', description: 'Paste node', defaultKey: 'Ctrl+v', currentKey: 'Ctrl+v' },
    { window: 'tree_connections', action: 'protection', description: 'Toggle node protection', defaultKey: 'Alt+r', currentKey: 'Alt+r' },
    { window: 'tree_connections', action: 'rename', description: 'Rename node', defaultKey: 'F2', currentKey: 'F2' },
    { window: 'tree_connections', action: 'connect_node', description: 'Connect node', defaultKey: 'Alt+c', currentKey: 'Alt+c' },

    { window: 'tabs', action: 'close', description: 'Close current tab', defaultKey: 'Ctrl+F4', currentKey: 'Ctrl+F4' },
    { window: 'tabs', action: 'next', description: 'Next tab', defaultKey: 'Ctrl+Page_Down', currentKey: 'Ctrl+Page_Down' },
    { window: 'tabs', action: 'previous', description: 'Previous tab', defaultKey: 'Ctrl+Page_Up', currentKey: 'Ctrl+Page_Up' },

    { window: 'terminal', action: 'fullscreen', description: 'Toggle fullscreen', defaultKey: 'F11', currentKey: 'F11' },
    { window: 'terminal', action: 'start', description: 'Start session', defaultKey: 'Ctrl+Return', currentKey: 'Ctrl+Return' },
    { window: 'terminal', action: 'copy', description: 'Copy selection', defaultKey: 'Ctrl+Shift+C', currentKey: 'Ctrl+Shift+C' },
    { window: 'terminal', action: 'paste', description: 'Paste clipboard', defaultKey: 'Ctrl+Shift+V', currentKey: 'Ctrl+Shift+V' },
    { window: 'terminal', action: 'paste_password', description: 'Paste password from vault', defaultKey: 'Ctrl+p', currentKey: 'Ctrl+p' },
    { window: 'terminal', action: 'disconnect', description: 'Disconnect session', defaultKey: 'Ctrl+Shift+W', currentKey: 'Ctrl+Shift+W' },
    { window: 'terminal', action: 'close', description: 'Close tab', defaultKey: 'Ctrl+w', currentKey: 'Ctrl+w' },
    { window: 'terminal', action: 'restart', description: 'Restart connection', defaultKey: 'Ctrl+Shift+R', currentKey: 'Ctrl+Shift+R' },
    { window: 'terminal', action: 'find_terminal', description: 'Find in terminal output', defaultKey: 'Ctrl+F3', currentKey: 'Ctrl+F3' },
    { window: 'terminal', action: 'paste_primary', description: 'Paste selection', defaultKey: 'Shift+Insert', currentKey: 'Shift+Insert' },
    { window: 'terminal', action: 'zoom_in', description: 'Zoom in', defaultKey: 'Ctrl+plus', currentKey: 'Ctrl+plus' },
    { window: 'terminal', action: 'zoom_out', description: 'Zoom out', defaultKey: 'Ctrl+minus', currentKey: 'Ctrl+minus' },
    { window: 'terminal', action: 'zoom_reset', description: 'Zoom reset', defaultKey: 'Ctrl+0', currentKey: 'Ctrl+0' },
    { window: 'terminal', action: 'toggle_sftp_browser', description: 'Toggle SFTP browser panel', defaultKey: 'Alt+Ctrl+s', currentKey: 'Alt+Ctrl+s' },

    { window: 'sftp_browser', action: 'refresh', description: 'Refresh remote files', defaultKey: 'F5', currentKey: 'F5' },
    { window: 'sftp_browser', action: 'parent', description: 'Navigate to parent folder', defaultKey: 'Backspace', currentKey: 'Backspace' },
    { window: 'sftp_browser', action: 'new_folder', description: 'Create new remote folder', defaultKey: 'F7', currentKey: 'F7' },
    { window: 'sftp_browser', action: 'toggle_hidden', description: 'Toggle hidden files', defaultKey: 'Ctrl+h', currentKey: 'Ctrl+h' },
    { window: 'sftp_browser', action: 'edit_path', description: 'Focus remote path bar', defaultKey: 'Ctrl+l', currentKey: 'Ctrl+l' },
  ];

</script>

<div class="keybindings-page">
  <div class="info-header">
    Keyboard shortcut reference by window. Changing shortcuts is not supported yet.
  </div>

  <div class="table-container">
    <table class="keybindings-table">
      <thead>
        <tr>
          <th>Window</th>
          <th>Action / Description</th>
          <th>Keybinding</th>
        </tr>
      </thead>
      <tbody>
        {#each defaultKeybindings as row}
          <tr>
            <td class="col-window"><code>{row.window}</code></td>
            <td class="col-desc">
              <b>{row.action}</b>: {row.description}
            </td>
            <td class="col-key">
              <span class="key-badge">{row.currentKey}</span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .keybindings-page {
    padding: 10px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-sizing: border-box;
    overflow: hidden;
  }

  .info-header {
    font-size: 0.9em;
    color: var(--gtk-muted-fg);
    background: var(--gtk-row-alt);
    padding: 6px 8px;
    border-radius: 4px;
  }

  .table-container {
    flex: 1;
    overflow-y: auto;
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    background: var(--gtk-base-color, #ffffff);
  }

  .keybindings-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9em;
  }

  .keybindings-table th {
    text-align: left;
    padding: 6px 8px;
    background: var(--gtk-header-bg);
    border-bottom: 1px solid var(--gtk-border-color);
    position: sticky;
    top: 0;
    z-index: 1;
  }

  .keybindings-table td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--gtk-border-color);
  }

  .keybindings-table tr:hover {
    background: var(--gtk-row-alt);
  }

  .col-window {
    width: 120px;
    color: var(--gtk-muted-fg);
  }

  .col-desc {
    flex: 1;
  }

  .col-key {
    width: 160px;
  }

  .col-actions {
    width: 140px;
    display: flex;
    gap: 4px;
  }

  .key-badge {
    display: inline-block;
    padding: 2px 6px;
    background: var(--gtk-header-bg);
    border: 1px solid var(--gtk-border-color);
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.85em;
  }

  .recording-badge {
    display: inline-block;
    padding: 4px 8px;
    background: var(--gtk-warning-bg);
    border: 1px solid var(--gtk-border-color);
    color: var(--gtk-warning-fg);
    border-radius: 3px;
    font-size: 0.85em;
    outline: none;
    animation: blink 1s infinite;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.6; }
  }
</style>
