<script lang="ts">
  import ContextMenu, { type MenuItem } from '../gtk/ContextMenu.svelte';
  import type { Terminal } from '@xterm/xterm';
  import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
  import * as SystemService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/systemservice.js';
  import * as ClusterService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/clusterservice.js';
  import { sessions } from '../../lib/stores/sessions';
  import { clusters } from '../../lib/stores/clusters';
  import { settings, tree, findNode } from '../../lib/stores/config';
  import { pccDialogOpen, clusterAdminDialogOpen, clustersVisible } from '../../lib/stores/ui';
  import {
    splits,
    splitTab,
    unsplitTab,
    equalResize,
    isSecondaryInSplit,
  } from '../../lib/stores/splits';

  interface Props {
    open: boolean;
    x: number;
    y: number;
    tmpID: string;
    kind: string;
    term?: Terminal;
    hasSFTP?: boolean;
    sftpOpen?: boolean;
    ontoggleSftp?: () => void;
    onfind?: () => void;
    onrestart?: () => void;
    ondisconnect?: () => void;
    oncloseTerm?: () => void;
    oncloseMenu: () => void;
  }

  let {
    open = false,
    x,
    y,
    tmpID,
    kind,
    term,
    hasSFTP = false,
    sftpOpen = false,
    ontoggleSftp,
    onfind,
    onrestart,
    ondisconnect,
    oncloseTerm,
    oncloseMenu,
  }: Props = $props();

  let sessionInfo = $derived($sessions.find((s) => s.tmpID === tmpID));
  let conn: any = $derived(
    sessionInfo?.connID && $tree ? findNode(sessionInfo.connID, $tree) : null
  );

  async function takeScreenshot() {
    if (!term) return;
    const canvas = term.element?.querySelector('canvas');
    let dataUrl = '';
    if (canvas) {
      dataUrl = canvas.toDataURL('image/png');
    } else if (term.element) {
      const offscreen = document.createElement('canvas');
      offscreen.width = term.element.clientWidth || 800;
      offscreen.height = term.element.clientHeight || 600;
      const ctx = offscreen.getContext('2d');
      if (ctx) {
        ctx.fillStyle = '#1e1e1e';
        ctx.fillRect(0, 0, offscreen.width, offscreen.height);
        ctx.fillStyle = '#ffffff';
        ctx.font = '12px monospace';
        const buffer = term.buffer.active;
        for (let i = 0; i < Math.min(buffer.length, 50); i++) {
          const line = buffer.getLine(i)?.translateToString(true) || '';
          ctx.fillText(line, 10, 20 + i * 14);
        }
        dataUrl = offscreen.toDataURL('image/png');
      }
    }
    if (dataUrl) {
      try {
        await SystemService.SaveScreenshot(sessionInfo?.connID || '', dataUrl);
      } catch (err) {
        console.error('Failed to save screenshot:', err);
      }
    }
  }

  function pasteText(text: string) {
    if (term) term.paste(text);
  }

  let items = $derived.by<MenuItem[]>(() => {
    const list: MenuItem[] = [
      {
        label: 'Show Terminal Keybindings',
        icon: 'gtk-help',
        action: () => {},
      },
      { separator: true },
      // Copy & Paste group
      {
        label: 'Copy',
        icon: 'gtk-copy',
        shortcut: 'Ctrl+Shift+C',
        action: () => {
          if (term) {
            const selection = term.getSelection();
            if (selection) navigator.clipboard.writeText(selection);
          }
        },
      },
      {
        label: 'Paste',
        icon: 'gtk-paste',
        shortcut: 'Ctrl+Shift+V',
        action: async () => {
          if (term) {
            const text = await navigator.clipboard.readText();
            if (text) term.paste(text);
          }
        },
      },
      { separator: true },
    ];

    // Clusters group (only while the cluster functions are shown)
    if ($clustersVisible) {
      const currentCluster = sessionInfo?.cluster || '';
      const clusterSubmenu: MenuItem[] = [
        {
          label: 'New Cluster...',
          action: () => clusterAdminDialogOpen.set(true),
        },
      ];
      for (const c of $clusters) {
        clusterSubmenu.push({
          label: `${c.name} (${c.sessions?.length || 0} terminals)`,
          action: () => ClusterService.Add(tmpID, c.name),
        });
      }

      list.push({
        label: currentCluster ? `Change Cluster (${currentCluster})` : 'Add to Cluster',
        icon: 'asbru-cluster-manager',
        submenu: clusterSubmenu,
      });

      if (currentCluster) {
        list.push({
          label: 'Remove from Cluster',
          action: () => ClusterService.Remove(tmpID),
        });
      }

      list.push({
        label: 'Power Cluster Controller',
        icon: 'gtk-justify-fill',
        action: () => pccDialogOpen.set(true),
      });

      list.push({ separator: true });
    }

    // Remote & Local Macros
    const remoteMacros = [
      ...(conn?.macros?.remote || []),
      ...($settings?.macros?.Remote || []),
    ];
    if (remoteMacros.length > 0) {
      list.push({
        label: 'Remote commands',
        submenu: remoteMacros.map((m: any) => ({
          label: m.confirm ? `CONFIRM: ${m.description || m.command}` : m.description || m.command,
          action: () => SessionService.RunRemoteMacro(tmpID, m.command, m.append_return !== false),
        })),
      });
    }

    const localMacros = [
      ...(conn?.macros?.local || []),
      ...($settings?.macros?.Local || []),
    ];
    if (localMacros.length > 0) {
      list.push({
        label: 'Local commands',
        submenu: localMacros.map((m: any) => ({
          label: m.confirm ? `CONFIRM: ${m.description || m.command}` : m.description || m.command,
          action: () => SessionService.RunLocalMacro(tmpID, m.command),
        })),
      });
    }

    // Insert Value Submenu
    const insertSubmenu: MenuItem[] = [];
    if (conn?.variables && conn.variables.length > 0) {
      insertSubmenu.push({
        label: 'User variables...',
        submenu: conn.variables.map((v: any, idx: number) => ({
          label: `<V:${idx}> ${v.value ? `(${v.value})` : ''}`,
          action: () => pasteText(v.value || ''),
        })),
      });
    }
    if ($settings?.variables && $settings.variables.length > 0) {
      insertSubmenu.push({
        label: 'Global variables...',
        submenu: $settings.variables.map((gv: any) => ({
          label: `<GV:${gv.name}> ${gv.value ? `(${gv.value})` : ''}`,
          action: () => pasteText(gv.value || ''),
        })),
      });
    }

    if (insertSubmenu.length > 0) {
      list.push({
        label: 'Insert value',
        submenu: insertSubmenu,
      });
    }

    list.push(
      { separator: true },
      {
        label: 'Find...',
        icon: 'gtk-find',
        shortcut: 'Ctrl+Shift+F',
        action: () => onfind && onfind(),
      },
      {
        label: 'Take Screenshot',
        icon: 'gtk-media-record',
        action: takeScreenshot,
      },
      {
        label: 'Save session log...',
        icon: 'gtk-save',
        action: async () => {
          const path = `/tmp/leanterm_session_${tmpID}.log`;
          await SessionService.SaveSessionLog(tmpID, path, true);
        },
      },
      {
        label: 'Reset',
        action: () => term?.reset(),
      },
      {
        label: 'Reset and clear',
        action: () => {
          term?.reset();
          term?.clear();
        },
      }
    );

    if (hasSFTP) {
      list.push(
        { separator: true },
        {
          label: kind === 'local'
            ? (sftpOpen ? 'Hide file browser' : 'Show file browser')
            : (sftpOpen ? 'Hide SFTP browser' : 'Show SFTP browser'),
          icon: 'asbru-method-sftp',
          shortcut: 'Alt+Ctrl+S',
          action: () => ontoggleSftp && ontoggleSftp(),
        }
      );
    }

    if (kind === 'telnet') {
      list.push(
        { separator: true },
        {
          label: 'Telnet',
          icon: 'asbru-method-telnet',
          submenu: [
            {
              label: 'Send BREAK',
              action: () => SessionService.SendTelnetCommand(tmpID, 'brk'),
            },
            {
              label: 'Send Interrupt Process (IP)',
              action: () => SessionService.SendTelnetCommand(tmpID, 'ip'),
            },
            {
              label: 'Send Abort Output (AO)',
              action: () => SessionService.SendTelnetCommand(tmpID, 'ao'),
            },
            {
              label: 'Send Are You There (AYT)',
              action: () => SessionService.SendTelnetCommand(tmpID, 'ayt'),
            },
            {
              label: 'Send Erase Character (EC)',
              action: () => SessionService.SendTelnetCommand(tmpID, 'ec'),
            },
            {
              label: 'Send Erase Line (EL)',
              action: () => SessionService.SendTelnetCommand(tmpID, 'el'),
            },
          ],
        }
      );
    }

    // Split group
    if (tmpID in $splits) {
      list.push(
        { separator: true },
        {
          label: 'Unsplit',
          action: () => unsplitTab(tmpID),
        },
        {
          label: 'Equally resize terminals',
          action: () => equalResize(tmpID),
        }
      );
    } else {
      const otherSessions = $sessions.filter(
        (s) => s.tmpID !== tmpID && !(s.tmpID in $splits) && !isSecondaryInSplit(s.tmpID, $splits)
      );
      if (otherSessions.length > 0) {
        list.push(
          { separator: true },
          {
            label: 'Split',
            submenu: [
              {
                label: 'Vertically',
                submenu: otherSessions.map((s, idx) => ({
                  label: `${idx + 1}: ${s.title || 'Terminal'}`,
                  action: () => splitTab(tmpID, s.tmpID, 'horizontal'),
                })),
              },
              {
                label: 'Horizontally',
                submenu: otherSessions.map((s, idx) => ({
                  label: `${idx + 1}: ${s.title || 'Terminal'}`,
                  action: () => splitTab(tmpID, s.tmpID, 'vertical'),
                })),
              },
            ],
          }
        );
      }
    }

    list.push(
      { separator: true },
      {
        label: 'Restart session',
        action: () => onrestart && onrestart(),
      },
      {
        label: 'Disconnect',
        icon: 'gtk-stop',
        action: () => ondisconnect && ondisconnect(),
      },
      {
        label: 'Close Terminal',
        icon: 'gtk-close',
        shortcut: 'Ctrl+Shift+W',
        action: () => oncloseTerm && oncloseTerm(),
      }
    );

    return list;
  });
</script>

{#if open}
  <ContextMenu {items} {x} {y} onclose={oncloseMenu} />
{/if}
