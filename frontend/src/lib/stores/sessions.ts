import { writable, derived, get } from 'svelte/store';
import { Events } from '@wailsio/runtime';
import * as SessionService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/sessionservice.js';
import * as models from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/models.js';

export interface HistoryEntry {
  id: string;
  name: string;
  timestamp: string;
  method: string;
}

export const sessions = writable<models.SessionInfo[]>([]);
export const activeTab = writable<string>('info'); // 'info' or session tmpID
export const history = writable<HistoryEntry[]>([]);

export const activeTerminalCount = derived(sessions, ($sessions) => $sessions.length);

/** Last status messages per session (max. 15, like the original status tooltip). */
export const sessionStatus = writable<Record<string, string[]>>({});
/** Sessions that produced output while their tab was not visible. */
export const newDataTabs = writable<Set<string>>(new Set());

const MAX_STATUS_HISTORY = 15;

function pushStatus(tmpID: string, text: string): void {
  if (!tmpID || !text) return;
  sessionStatus.update((all) => {
    const list = [...(all[tmpID] ?? []), text];
    if (list.length > MAX_STATUS_HISTORY) {
      list.splice(0, list.length - MAX_STATUS_HISTORY + 1, '(... older status skipped...)');
    }
    return { ...all, [tmpID]: list };
  });
}

function setSessionState(tmpID: string, state: string, title?: string): void {
  let found = false;
  sessions.update((list) =>
    list.map((s) => {
      if (s.tmpID !== tmpID) return s;
      found = true;
      if (s.state === state && (!title || s.title === title)) return s;
      return { ...s, state, ...(title ? { title } : {}) } as models.SessionInfo;
    })
  );
  if (!found) scheduleRefresh();
  pushStatus(tmpID, state.toUpperCase());
}

/** Called by terminal streams when output arrives; cheap no-op for the visible tab. */
export function markNewData(tmpID: string): void {
  if (get(activeTab) === tmpID) return;
  const current = get(newDataTabs);
  if (current.has(tmpID)) return;
  const next = new Set(current);
  next.add(tmpID);
  newDataTabs.set(next);
}

let refreshTimer: ReturnType<typeof setTimeout> | undefined;
function scheduleRefresh(): void {
  clearTimeout(refreshTimer);
  refreshTimer = setTimeout(() => {
    refreshSessions();
  }, 50);
}

function payload(ev: any): any {
  return ev && typeof ev === 'object' && 'data' in ev ? ev.data : ev;
}

/**
 * Subscribes to backend session events. Sessions opened by the backend itself
 * (CLI arguments, "launch at startup", second instance) appear without polling.
 */
export function initSessionEvents(): () => void {
  const offs = [
    Events.On('session:opened', (ev: any) => {
      const data = payload(ev);
      scheduleRefresh();
      refreshHistory();
      const tmpID = data?.session?.tmpID;
      if (tmpID && data?.where !== 'background') activeTab.set(tmpID);
    }),
    Events.On('session:state', (ev: any) => {
      const data = payload(ev);
      if (data?.tmpID && data?.state) setSessionState(data.tmpID, data.state, data.title);
    }),
    Events.On('session:status', (ev: any) => {
      const data = payload(ev);
      if (data?.tmpID) pushStatus(data.tmpID, data.text || data.code || '');
    }),
    Events.On('session:title', (ev: any) => {
      const data = payload(ev);
      if (data?.tmpID && data?.title) {
        sessions.update((list) => list.map((s) => (s.tmpID === data.tmpID ? ({ ...s, title: data.title } as models.SessionInfo) : s)));
      }
    }),
    Events.On('session:closed', () => scheduleRefresh()),
  ];

  const unsubActive = activeTab.subscribe((id) => {
    const current = get(newDataTabs);
    if (current.has(id)) {
      const next = new Set(current);
      next.delete(id);
      newDataTabs.set(next);
    }
  });

  return () => {
    offs.forEach((off) => off && off());
    unsubActive();
  };
}

export async function refreshSessions(): Promise<models.SessionInfo[]> {
  try {
    const list = await SessionService.List();
    sessions.set(list);
    return list;
  } catch (err) {
    console.error('Failed to list sessions:', err);
    return [];
  }
}

export async function refreshHistory(): Promise<void> {
  try {
    const list = await SessionService.GetHistory();
    if (list) {
      history.set(
        list.map((e: any) => ({
          id: e.id,
          name: e.name,
          timestamp: formatDate(new Date(e.startedAt)),
          method: e.method,
        }))
      );
    }
  } catch (err) {
    console.warn('Failed to load history:', err);
  }
}

// Initial fetch of persisted history
refreshHistory();

function formatDate(date: Date): string {
  const pad = (n: number) => (n < 10 ? '0' + n : n.toString());
  const hh = pad(date.getHours());
  const mm = pad(date.getMinutes());
  const ss = pad(date.getSeconds());
  const d = pad(date.getDate());
  const m = pad(date.getMonth() + 1);
  const y = date.getFullYear();
  return `${hh}:${mm}:${ss} ${d}-${m}-${y}`;
}

export async function openConnections(
  connIDs: string[],
  where = 'tab',
  cluster = '',
  instances = 1
): Promise<string[]> {
  try {
    const req = new models.OpenRequest({
      connIDs,
      where,
      cluster,
      instances,
    });
    const tmpIDs = await SessionService.Open(req);
    await refreshSessions();
    await refreshHistory();

    if (tmpIDs.length > 0) {
      activeTab.set(tmpIDs[0]);
    }
    return tmpIDs;
  } catch (err) {
    console.error('Failed to open connections:', err);
    throw err;
  }
}

export async function openLocalShell(where = 'tab'): Promise<string> {
  try {
    const tmpID = await SessionService.OpenLocalShell(where);
    await refreshSessions();
    activeTab.set(tmpID);

    // Record history
    history.update((curr) => [
      {
        id: tmpID,
        name: 'Local Shell',
        timestamp: formatDate(new Date()),
        method: 'local',
      },
      ...curr,
    ]);

    return tmpID;
  } catch (err) {
    console.error('Failed to open local shell:', err);
    throw err;
  }
}

export async function disconnectSession(tmpID: string): Promise<void> {
  try {
    await SessionService.Disconnect(tmpID);
    await refreshSessions();
  } catch (err) {
    console.error('Failed to disconnect session:', err);
    throw err;
  }
}

export async function closeSession(tmpID: string): Promise<void> {
  try {
    await SessionService.Close(tmpID);
    const currSessions = await refreshSessions();

    const currActive = get(activeTab);
    if (currActive === tmpID) {
      if (currSessions.length > 0) {
        activeTab.set(currSessions[currSessions.length - 1].tmpID);
      } else {
        activeTab.set('info');
      }
    }
  } catch (err) {
    console.error('Failed to close session:', err);
    throw err;
  }
}

export function setActiveTab(tabId: string): void {
  activeTab.set(tabId);
}

export async function sendSessionInput(tmpID: string, text: string): Promise<void> {
  try {
    const bytes = new TextEncoder().encode(text);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    const b64 = window.btoa(binary);
    await SessionService.Write(tmpID, b64);
  } catch (err) {
    console.error(`Failed to send input to session ${tmpID}:`, err);
  }
}
