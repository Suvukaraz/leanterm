// LeanTerm icon mapping

const stockIcons: Record<string, string> = {
  'gtk-edit': '/icons/stock/gtk-edit.svg',
  'gtk-delete': '/icons/stock/gtk-delete.svg',
  'gtk-connect': '/icons/stock/gtk-connect.svg',
  'gtk-find': '/icons/stock/gtk-find.svg',
  'gtk-preferences': '/icons/stock/gtk-preferences.svg',
  'gtk-save': '/icons/stock/gtk-save.svg',
  'gtk-quit': '/icons/stock/gtk-quit.svg',
  'gtk-about': '/icons/stock/gtk-about.svg',
  'gtk-close': '/icons/stock/gtk-close.svg',
  'gtk-execute': '/icons/stock/gtk-execute.svg',
  'gtk-copy': '/icons/stock/gtk-copy.svg',
  'gtk-cut': '/icons/stock/gtk-cut.svg',
  'gtk-paste': '/icons/stock/gtk-paste.svg',
  'gtk-info': '/icons/stock/gtk-info.svg',
  'gtk-media-previous': '/icons/stock/gtk-media-previous.svg',
  'gtk-media-next': '/icons/stock/gtk-media-next.svg',
  'gtk-spell-check': '/icons/stock/gtk-spell-check.svg',
  'gtk-justify-fill': '/icons/stock/gtk-justify-fill.svg',
  'gtk-home': '/icons/stock/gtk-home.svg',
  'gtk-new': '/icons/stock/gtk-new.svg',
  'gtk-add': '/icons/stock/gtk-add.svg',
  'gtk-remove': '/icons/stock/gtk-remove.svg',
  'gtk-open': '/icons/stock/gtk-open.svg',
  'gtk-save-as': '/icons/stock/gtk-save-as.svg',
  'gtk-help': '/icons/stock/gtk-help.svg',
  'gtk-stop': '/icons/stock/gtk-stop.svg',
  'gtk-fullscreen': '/icons/stock/gtk-fullscreen.svg',
  'gtk-leave-fullscreen': '/icons/stock/gtk-leave-fullscreen.svg',
  'gtk-media-record': '/icons/stock/gtk-media-record.svg',
  'gtk-dialog-question': '/icons/stock/gtk-dialog-question.svg',
  'gtk-dialog-warning': '/icons/stock/gtk-dialog-warning.svg',
  'gtk-dialog-error': '/icons/stock/gtk-dialog-warning.svg',
  'gtk-undo': '/icons/stock/gtk-undo.svg',
  'gtk-refresh': '/icons/stock/gtk-refresh.svg',
  'gtk-go-up': '/icons/stock/gtk-go-up.svg',
  'gtk-go-back': '/icons/stock/gtk-go-back.svg',
  'gtk-go-forward': '/icons/stock/gtk-go-forward.svg',
  'gtk-directory': '/icons/stock/gtk-directory.svg',
  'gtk-file': '/icons/stock/gtk-file.svg',
};

// Map of theme-relative files
const themeIconFiles: Record<string, string> = {
  'asbru-help': 'asbru-help.svg',
  'asbru-group-add': 'asbru_group_add_16x16.svg',
  'asbru-node-add': 'asbru_node_add_16x16.svg',
  'asbru-node-del': 'asbru_node_del_16x16.png',
  'asbru-chain': 'asbru_chain.png',
  'asbru-cluster-auto': 'asbru_cluster_auto.png',
  'asbru-cluster-manager2': 'asbru_cluster_manager2.png',
  'asbru-cluster-manager': 'asbru_cluster_manager.svg',
  'asbru-cluster-manager-off': 'asbru_cluster_manager_off.svg',
  'asbru-favourite-on': 'asbru_favourite_on.svg',
  'asbru-favourite-off': 'asbru_favourite_off.svg',
  'asbru-group-closed': 'asbru_group_closed_16x16.svg',
  'asbru-group-open': 'asbru_group_open_16x16.svg',
  'asbru-group': 'asbru_group.svg',
  'asbru-group-connect': 'asbru_group.svg',
  'asbru-history': 'asbru_history.svg',
  'asbru-keepass': 'asbru_keepass.png',
  'asbru-method-WebDAV': 'asbru_method_cadaver.png',
  'asbru-method-MOSH': 'asbru_method_mosh.svg',
  'asbru-method-3270': 'asbru_method_3270.jpg',
  'asbru-method-cu': 'asbru_method_cu.jpg',
  'asbru-method-ftp': 'asbru_method_ftp.svg',
  'asbru-method-generic': 'asbru_method_generic.svg',
  'asbru-method-rdp': 'asbru_method_rdesktop.svg',
  'asbru-method-sftp': 'asbru_method_sftp.svg',
  'asbru-method-ssh': 'asbru_method_ssh.svg',
  'asbru-method-telnet': 'asbru_method_telnet.svg',
  'asbru-method-vnc': 'asbru_method_vncviewer.svg',
  'asbru-quick-connect': 'asbru_quick_connect.svg',
  'asbru-shell': 'asbru_shell.svg',
  'asbru-tab': 'asbru_tab.png',
  'asbru-treelist': 'asbru_treelist.svg',
  'asbru-wol': 'asbru_wol.svg',
  'asbru-prompt': 'asbru_prompt.png',
  'asbru-protected': 'asbru_protected.png',
  'asbru-unprotected': 'asbru_unprotected.png',
  'asbru-buttonbar-show': 'asbru_buttonbar_show.png',
  'asbru-buttonbar-hide': 'asbru_buttonbar_hide.png',
  'asbru_cluster_connection': 'asbru_cluster_connection.svg',
  'asbru_cluster_auto': 'asbru_cluster_auto.png',
};

// Root icons
const rootIconFiles: Record<string, string> = {
  'asbru-app-big': '/icons/leanterm-logo.svg',
  'asbru-terminal-ok-small': '/icons/asbru_terminal16x16.png',
  'asbru-terminal-ok-big': '/icons/asbru_terminal64x64.png',
  'asbru-terminal-ko-small': '/icons/asbru_terminal_x16x16.png',
  'asbru-terminal-ko-big': '/icons/asbru_terminal_x64x64.png',
  'leanterm-logo': '/icons/leanterm-logo.svg',
};

// Icon theme chosen in Preferences (general.theme). Callers that pass no theme
// (or the placeholder 'default') get the active one.
let activeIconTheme = 'default';

export function setIconTheme(theme: string | undefined | null): void {
  activeIconTheme = theme && theme !== 'system' ? theme : 'default';
}

export function getIcon(name: string, theme?: string): string {
  if (!name) return '';
  if (!theme || theme === 'default') theme = activeIconTheme;
  if (stockIcons[name]) {
    return stockIcons[name];
  }
  if (rootIconFiles[name]) {
    return rootIconFiles[name];
  }
  if (themeIconFiles[name]) {
    const safeTheme = theme === 'system' ? 'default' : theme;
    return `/icons/${safeTheme}/${themeIconFiles[name]}`;
  }
  // If given a direct path
  if (name.startsWith('/') || name.startsWith('data:') || name.startsWith('http')) {
    return name;
  }
  // Fallback to stock icon if gtk- style
  if (name.startsWith('gtk-')) {
    return `/icons/stock/${name}.svg`;
  }
  return `/icons/${theme}/${name}`;
}
