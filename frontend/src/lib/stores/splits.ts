import { writable, get } from 'svelte/store';
import { sessions, activeTab } from './sessions';
import { settings } from './config';

export interface TabSplit {
  primaryTmpID: string;
  secondaryTmpID: string;
  orientation: 'horizontal' | 'vertical';
  ratio: number; // percentage, default 50
}

export const splits = writable<Record<string, TabSplit>>({});

export function splitTab(
  primaryTmpID: string,
  secondaryTmpID: string,
  orientation: 'horizontal' | 'vertical' = 'vertical'
): void {
  if (!primaryTmpID || !secondaryTmpID || primaryTmpID === secondaryTmpID) return;

  splits.update((curr) => ({
    ...curr,
    [primaryTmpID]: {
      primaryTmpID,
      secondaryTmpID,
      orientation,
      ratio: 50,
    },
  }));

  activeTab.set(primaryTmpID);
}

export function unsplitTab(primaryTmpID: string): void {
  splits.update((curr) => {
    const copy = { ...curr };
    delete copy[primaryTmpID];
    return copy;
  });
}

export function equalResize(primaryTmpID: string): void {
  splits.update((curr) => {
    if (!curr[primaryTmpID]) return curr;
    return {
      ...curr,
      [primaryTmpID]: {
        ...curr[primaryTmpID],
        ratio: 50,
      },
    };
  });
}

export function isSecondaryInSplit(tmpID: string, splitMap: Record<string, TabSplit>): boolean {
  for (const s of Object.values(splitMap)) {
    if (s.secondaryTmpID === tmpID) return true;
  }
  return false;
}

// Monitor sessions for auto-unsplit on disconnect if configured
sessions.subscribe((sessionList) => {
  const currentSplits = get(splits);
  const cfg = get(settings);
  if (!cfg?.terminal?.UnsplitDisconnected) return;

  for (const [primaryId, s] of Object.entries(currentSplits)) {
    const primarySess = sessionList.find((sess) => sess.tmpID === s.primaryTmpID);
    const secondarySess = sessionList.find((sess) => sess.tmpID === s.secondaryTmpID);

    if (
      (primarySess && primarySess.state === 'disconnected') ||
      (secondarySess && secondarySess.state === 'disconnected')
    ) {
      unsplitTab(primaryId);
    }
  }
});
