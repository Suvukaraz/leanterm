import { writable } from 'svelte/store';
import * as ClusterService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/clusterservice.js';
import type { ClusterInfo } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/cluster/models.js';
import { Events } from '@wailsio/runtime';

export const clusters = writable<ClusterInfo[]>([]);

export async function loadClusters() {
  try {
    const list = await ClusterService.List();
    clusters.set(list || []);
  } catch (err) {
    console.error('Failed to load clusters:', err);
  }
}

// Subscribe to backend cluster changes
if (typeof window !== 'undefined') {
  Events.On('cluster:changed', (data: any) => {
    if (Array.isArray(data)) {
      clusters.set(data);
    } else if (data?.data && Array.isArray(data.data)) {
      clusters.set(data.data);
    }
  });
}
