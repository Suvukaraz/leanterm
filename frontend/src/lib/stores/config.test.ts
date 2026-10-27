import { describe, it, expect } from 'vitest';
import { findNode, findParent, collectFavourites, getAllNodes } from './config';
import type { TreeNode } from '../../../bindings/github.com/Suvukaraz/leanterm/internal/config/models';

describe('Tree traversal helpers', () => {
  const sampleTree: any = {
    id: '',
    name: 'My Connections',
    isGroup: true,
    protected: false,
    favourite: false,
    hasPassword: false,
    children: [
      {
        id: 'grp-1',
        parentId: '',
        name: 'Datacenter',
        isGroup: true,
        protected: false,
        favourite: false,
        hasPassword: false,
        children: [
          {
            id: 'conn-1',
            parentId: 'grp-1',
            name: 'web01',
            isGroup: false,
            method: 'ssh',
            host: '10.0.0.1',
            port: 22,
            protected: false,
            favourite: true,
            hasPassword: true,
          },
          {
            id: 'conn-2',
            parentId: 'grp-1',
            name: 'db01',
            isGroup: false,
            method: 'ssh',
            host: '10.0.0.2',
            port: 22,
            protected: true,
            favourite: false,
            hasPassword: true,
          },
        ],
      },
      {
        id: 'conn-3',
        parentId: '',
        name: 'switch-core',
        isGroup: false,
        method: 'telnet',
        host: '192.168.1.1',
        port: 23,
        protected: false,
        favourite: true,
        hasPassword: false,
      },
    ],
  };

  it('finds node by id correctly', () => {
    const root = findNode('', sampleTree);
    expect(root?.name).toBe('My Connections');

    const grp = findNode('grp-1', sampleTree);
    expect(grp?.name).toBe('Datacenter');

    const conn = findNode('conn-1', sampleTree);
    expect(conn?.name).toBe('web01');
    expect(conn?.host).toBe('10.0.0.1');

    const notFound = findNode('unknown-id', sampleTree);
    expect(notFound).toBeNull();
  });

  it('finds parent node correctly', () => {
    const parentOfConn1 = findParent('conn-1', sampleTree);
    expect(parentOfConn1?.id).toBe('grp-1');

    const parentOfGrp1 = findParent('grp-1', sampleTree);
    expect(parentOfGrp1?.id).toBe('');

    const parentOfRoot = findParent('', sampleTree);
    expect(parentOfRoot).toBeNull();
  });

  it('collects all favourites', () => {
    const favs = collectFavourites(sampleTree);
    expect(favs).toHaveLength(2);
    expect(favs.map((f) => f.id)).toEqual(['conn-1', 'conn-3']);
  });

  it('collects all nodes', () => {
    const all = getAllNodes(sampleTree);
    expect(all).toHaveLength(5); // root, grp-1, conn-1, conn-2, conn-3
    expect(all.map((n) => n.id)).toEqual(['', 'grp-1', 'conn-1', 'conn-2', 'conn-3']);
  });
});
