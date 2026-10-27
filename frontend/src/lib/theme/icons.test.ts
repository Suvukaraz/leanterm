import { describe, it, expect } from 'vitest';
import { getIcon } from './icons';

describe('icons', () => {
  it('resolves stock icons correctly', () => {
    expect(getIcon('gtk-edit')).toBe('/icons/stock/gtk-edit.svg');
    expect(getIcon('gtk-delete')).toBe('/icons/stock/gtk-delete.svg');
    expect(getIcon('gtk-connect')).toBe('/icons/stock/gtk-connect.svg');
  });

  it('resolves theme icon files', () => {
    expect(getIcon('asbru-group-add', 'default')).toBe('/icons/default/asbru_group_add_16x16.svg');
    expect(getIcon('asbru-method-ssh', 'asbru-dark')).toBe('/icons/asbru-dark/asbru_method_ssh.svg');
  });

  it('resolves root icons', () => {
    expect(getIcon('leanterm-logo')).toBe('/icons/leanterm-logo.svg');
    expect(getIcon('asbru-terminal-ok-small')).toBe('/icons/asbru_terminal16x16.png');
  });
});
