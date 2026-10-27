import { writable, get } from 'svelte/store';
import * as ConfigService from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/configservice.js';
import * as models from '../../../bindings/github.com/Suvukaraz/leanterm/internal/services/models.js';
import * as configModels from '../../../bindings/github.com/Suvukaraz/leanterm/internal/config/models.js';

export const tree = writable<configModels.TreeNode | null>(null);
export const settings = writable<models.Settings | null>(null);
export const hasUnsavedChanges = writable<boolean>(false);
export const selectedIds = writable<string[]>([]);
export const clipboard = writable<{ ids: string[]; op: 'copy' | 'cut' | null }>({ ids: [], op: null });
export const vaultStatus = writable<string>('keyring');

export async function loadTree(): Promise<configModels.TreeNode | null> {
  try {
    const t = await ConfigService.GetTree();
    tree.set(t);
    return t;
  } catch (err) {
    console.error('Failed to load tree:', err);
    return null;
  }
}

export async function loadSettings(): Promise<models.Settings | null> {
  try {
    const s = await ConfigService.GetSettings();
    settings.set(s);
    return s;
  } catch (err) {
    console.error('Failed to load settings:', err);
    return null;
  }
}

export async function checkUnsaved(): Promise<boolean> {
  try {
    const dirty = await ConfigService.HasUnsavedChanges();
    hasUnsavedChanges.set(dirty);
    return dirty;
  } catch (err) {
    console.error('Failed to check unsaved:', err);
    return false;
  }
}

export async function save(): Promise<void> {
  try {
    await ConfigService.Save();
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to save config:', err);
    throw err;
  }
}

export async function addNode(parentId: string, isGroup: boolean, name: string, title: string): Promise<string> {
  try {
    const id = await ConfigService.AddNode(parentId, isGroup, name, title);
    await loadTree();
    await checkUnsaved();
    return id;
  } catch (err) {
    console.error('Failed to add node:', err);
    throw err;
  }
}

export async function renameNode(id: string, name: string, title: string): Promise<void> {
  try {
    await ConfigService.RenameNode(id, name, title);
    await loadTree();
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to rename node:', err);
    throw err;
  }
}

export async function deleteNodes(ids: string[]): Promise<void> {
  try {
    await ConfigService.DeleteNodes(ids);
    await loadTree();
    await checkUnsaved();
    selectedIds.update((current) => current.filter((id) => !ids.includes(id)));
  } catch (err) {
    console.error('Failed to delete nodes:', err);
    throw err;
  }
}

export async function moveNodes(ids: string[], targetParent: string): Promise<void> {
  try {
    await ConfigService.MoveNodes(ids, targetParent);
    await loadTree();
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to move nodes:', err);
    throw err;
  }
}

export async function copyNodes(ids: string[]): Promise<void> {
  try {
    await ConfigService.CopyNodes(ids);
    clipboard.set({ ids, op: 'copy' });
  } catch (err) {
    console.error('Failed to copy nodes:', err);
    throw err;
  }
}

export async function cutNodes(ids: string[]): Promise<void> {
  try {
    await ConfigService.CutNodes(ids);
    clipboard.set({ ids, op: 'cut' });
  } catch (err) {
    console.error('Failed to cut nodes:', err);
    throw err;
  }
}

export async function pasteNodes(parent: string): Promise<void> {
  try {
    await ConfigService.PasteNodes(parent);
    await loadTree();
    await checkUnsaved();
    clipboard.set({ ids: [], op: null });
  } catch (err) {
    console.error('Failed to paste nodes:', err);
    throw err;
  }
}

export async function cloneNode(id: string): Promise<string> {
  try {
    const newId = await ConfigService.CloneNode(id);
    await loadTree();
    await checkUnsaved();
    return newId;
  } catch (err) {
    console.error('Failed to clone node:', err);
    throw err;
  }
}

export async function setProtected(ids: string[], on: boolean): Promise<void> {
  try {
    await ConfigService.SetProtected(ids, on);
    await loadTree();
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to set protected:', err);
    throw err;
  }
}

export async function setFavourite(id: string, on: boolean): Promise<void> {
  try {
    await ConfigService.SetFavourite(id, on);
    await loadTree();
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to set favourite:', err);
    throw err;
  }
}

export async function setDescription(id: string, text: string): Promise<void> {
  try {
    await ConfigService.SetDescription(id, text);
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to set description:', err);
    throw err;
  }
}

export async function search(query: string, field: string, fromId: string, backwards: boolean): Promise<string> {
  try {
    return await ConfigService.Search(query, field, fromId, backwards);
  } catch (err) {
    console.error('Failed to search:', err);
    return '';
  }
}

export async function saveSettings(s: models.Settings): Promise<void> {
  try {
    await ConfigService.SaveSettings(s);
    settings.set(s);
    await checkUnsaved();
  } catch (err) {
    console.error('Failed to save settings:', err);
    throw err;
  }
}

// Tree traversal helpers
export function findNode(id: string, root?: configModels.TreeNode | null): configModels.TreeNode | null {
  const currentRoot = root ?? get(tree);
  if (!currentRoot) return null;
  if (currentRoot.id === id) return currentRoot;
  if (currentRoot.children) {
    for (const child of currentRoot.children) {
      if (!child) continue;
      const found = findNode(id, child);
      if (found) return found;
    }
  }
  return null;
}

export function findParent(id: string, root?: configModels.TreeNode | null): configModels.TreeNode | null {
  const currentRoot = root ?? get(tree);
  if (!currentRoot || !currentRoot.children) return null;
  for (const child of currentRoot.children) {
    if (!child) continue;
    if (child.id === id) return currentRoot;
    const found = findParent(id, child);
    if (found) return found;
  }
  return null;
}

export function collectFavourites(root?: configModels.TreeNode | null): configModels.TreeNode[] {
  const result: configModels.TreeNode[] = [];
  const currentRoot = root ?? get(tree);
  if (!currentRoot) return result;

  function traverse(n: configModels.TreeNode) {
    if (!n.isGroup && n.favourite) {
      result.push(n);
    }
    if (n.children) {
      for (const child of n.children) {
        if (child) traverse(child);
      }
    }
  }
  traverse(currentRoot);
  return result;
}

export function getAllNodes(root?: configModels.TreeNode | null): configModels.TreeNode[] {
  const result: configModels.TreeNode[] = [];
  const currentRoot = root ?? get(tree);
  if (!currentRoot) return result;

  function traverse(n: configModels.TreeNode) {
    result.push(n);
    if (n.children) {
      for (const child of n.children) {
        if (child) traverse(child);
      }
    }
  }
  traverse(currentRoot);
  return result;
}
