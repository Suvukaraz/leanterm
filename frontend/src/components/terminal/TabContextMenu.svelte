<script lang="ts">
  import ContextMenu, { type MenuItem } from '../gtk/ContextMenu.svelte';
  import {
    sessions,
    activeTab,
    setActiveTab,
    disconnectSession,
    closeSession,
  } from '../../lib/stores/sessions';
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
    tabId: string;
    onfind?: (tmpID: string) => void;
    ondetach?: (tmpID: string) => void;
    oncloseMenu: () => void;
  }

  let {
    open = false,
    x,
    y,
    tabId,
    onfind,
    ondetach,
    oncloseMenu,
  }: Props = $props();

  let isSplit = $derived(tabId in $splits);

  let items = $derived.by<MenuItem[]>(() => {
    if (tabId === 'info') return [];

    const list: MenuItem[] = [];

    // 1. Goto TAB submenu
    const gotoItems: MenuItem[] = $sessions.map((s, idx) => ({
      label: `${idx + 1}: ${s.title || 'Terminal'}`,
      icon: s.kind === 'telnet' ? 'asbru-method-telnet' : 'asbru-method-ssh',
      action: () => setActiveTab(s.tmpID),
    }));

    list.push({
      label: 'Goto TAB',
      icon: 'asbru-tab',
      submenu: gotoItems,
    });

    list.push({ separator: true });

    // 2. Split group
    if (isSplit) {
      list.push(
        {
          label: 'Unsplit',
          action: () => unsplitTab(tabId),
        },
        {
          label: 'Equally resize terminals',
          action: () => equalResize(tabId),
        }
      );
    } else {
      // Find other candidates for splitting
      const otherSessions = $sessions.filter(
        (s) => s.tmpID !== tabId && !(s.tmpID in $splits) && !isSecondaryInSplit(s.tmpID, $splits)
      );

      const verticalItems: MenuItem[] = otherSessions.map((s, idx) => ({
        label: `${idx + 1}: ${s.title || 'Terminal'}`,
        action: () => splitTab(tabId, s.tmpID, 'horizontal'), // side-by-side
      }));

      const horizontalItems: MenuItem[] = otherSessions.map((s, idx) => ({
        label: `${idx + 1}: ${s.title || 'Terminal'}`,
        action: () => splitTab(tabId, s.tmpID, 'vertical'), // stacked
      }));

      list.push({
        label: 'Split',
        submenu: [
          {
            label: 'Vertically',
            disabled: verticalItems.length === 0,
            submenu: verticalItems.length > 0 ? verticalItems : undefined,
          },
          {
            label: 'Horizontally',
            disabled: horizontalItems.length === 0,
            submenu: horizontalItems.length > 0 ? horizontalItems : undefined,
          },
        ],
      });
    }

    list.push({ separator: true });

    // 3. Detach TAB
    list.push({
      label: 'Detach TAB to a new Window',
      icon: 'gtk-fullscreen',
      action: () => ondetach && ondetach(tabId),
    });

    list.push({ separator: true });

    // 4. Session actions
    list.push(
      {
        label: 'Find...',
        icon: 'gtk-find',
        shortcut: 'Ctrl+Shift+F',
        action: () => onfind && onfind(tabId),
      },
      {
        label: 'Disconnect session',
        icon: 'gtk-stop',
        action: () => disconnectSession(tabId),
      },
      {
        label: 'Close terminal',
        icon: 'gtk-close',
        shortcut: 'Ctrl+Shift+W',
        action: () => closeSession(tabId),
      },
      {
        label: 'Close other terminals',
        action: () => {
          for (const s of $sessions) {
            if (s.tmpID !== tabId) {
              closeSession(s.tmpID);
            }
          }
        },
      },
      {
        label: 'Close all terminals',
        action: () => {
          for (const s of $sessions) {
            closeSession(s.tmpID);
          }
        },
      }
    );

    return list;
  });
</script>

{#if open && tabId !== 'info'}
  <ContextMenu {items} {x} {y} onclose={oncloseMenu} />
{/if}
