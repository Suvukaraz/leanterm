// Languages of the editor: detection by file name, path, extension and shebang,
// and lazily loaded CodeMirror support (each language is its own chunk).
import type { Extension } from '@codemirror/state';
import type { StreamParser } from '@codemirror/language';

export type LanguageGroup = 'code' | 'web' | 'config';

export interface LanguageDef {
  id: string;
  name: string;
  /** Submenu of Tools → Highlighting. */
  group: LanguageGroup;
  /** Lower-case extensions without dot; the longest match wins (e.g. "d.ts"). */
  extensions?: string[];
  /** Exact base names (compared case-insensitively). */
  filenames?: string[];
  /** Tested against the full path ("/" separators). */
  pathPatterns?: RegExp[];
  /** Tested against the interpreter of a "#!" first line. */
  shebang?: RegExp;
  load: () => Promise<Extension>;
}

async function legacy(load: () => Promise<StreamParser<unknown>>): Promise<Extension> {
  const [{ StreamLanguage }, parser] = await Promise.all([import('@codemirror/language'), load()]);
  return StreamLanguage.define(parser);
}

const custom = (name: keyof typeof import('./customModes')) =>
  () => legacy(async () => (await import('./customModes'))[name] as StreamParser<unknown>);

const clike = (name: 'csharp' | 'kotlin' | 'dart') =>
  () => legacy(async () => (await import('@codemirror/legacy-modes/mode/clike'))[name] as StreamParser<unknown>);

const cpp = async () => (await import('@codemirror/lang-cpp')).cpp();
const js = (jsx: boolean, typescript: boolean) => async () =>
  (await import('@codemirror/lang-javascript')).javascript({ jsx, typescript });
const iniLike = () => legacy(async () => (await import('@codemirror/legacy-modes/mode/properties')).properties);
const shell = () => legacy(async () => (await import('@codemirror/legacy-modes/mode/shell')).shell);

export const LANGUAGES: LanguageDef[] = [
  // ---- Programming languages
  { id: 'c', name: 'C', group: 'code', extensions: ['c', 'h'], load: cpp },
  {
    id: 'cpp',
    name: 'C++',
    group: 'code',
    extensions: ['cpp', 'cc', 'cxx', 'c++', 'hpp', 'hh', 'hxx', 'h++', 'ipp', 'tpp', 'inl'],
    load: cpp,
  },
  { id: 'arduino', name: 'Arduino', group: 'code', extensions: ['ino', 'pde'], load: cpp },
  { id: 'csharp', name: 'C#', group: 'code', extensions: ['cs', 'csx'], load: clike('csharp') },
  { id: 'java', name: 'Java', group: 'code', extensions: ['java'], load: async () => (await import('@codemirror/lang-java')).java() },
  { id: 'kotlin', name: 'Kotlin', group: 'code', extensions: ['kt', 'kts'], load: clike('kotlin') },
  { id: 'dart', name: 'Dart / Flutter', group: 'code', extensions: ['dart'], load: clike('dart') },
  { id: 'go', name: 'Go', group: 'code', extensions: ['go'], load: async () => (await import('@codemirror/lang-go')).go() },
  { id: 'rust', name: 'Rust', group: 'code', extensions: ['rs'], load: async () => (await import('@codemirror/lang-rust')).rust() },
  {
    id: 'python',
    name: 'Python',
    group: 'code',
    extensions: ['py', 'pyw', 'pyi'],
    filenames: ['SConstruct', 'SConscript'],
    shebang: /python[\d.]*$/,
    load: async () => (await import('@codemirror/lang-python')).python(),
  },
  { id: 'mojo', name: 'Mojo', group: 'code', extensions: ['mojo', '🔥'], load: custom('mojo') },
  { id: 'nim', name: 'Nim', group: 'code', extensions: ['nim', 'nims', 'nimble'], load: custom('nim') },
  {
    id: 'clojure',
    name: 'Clojure',
    group: 'code',
    extensions: ['clj', 'cljs', 'cljc', 'edn', 'bb'],
    shebang: /^(bb|clojure|clj)$/,
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/clojure')).clojure),
  },
  {
    id: 'lua',
    name: 'Lua',
    group: 'code',
    extensions: ['lua', 'rockspec'],
    shebang: /^lua[\d.]*$/,
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/lua')).lua),
  },
  {
    id: 'bash',
    name: 'Bash / Shell',
    group: 'code',
    extensions: ['sh', 'bash', 'zsh', 'ksh', 'ash', 'dash', 'ebuild', 'eclass', 'bats', 'command'],
    filenames: [
      '.bashrc', '.bash_profile', '.bash_login', '.bash_logout', '.bash_aliases', '.profile', '.zshrc',
      '.zshenv', '.zprofile', '.zlogin', '.kshrc', '.xprofile', '.xinitrc', '.xsessionrc', 'PKGBUILD',
      'APKBUILD', 'bashrc', 'profile', 'zshrc',
    ],
    shebang: /^(ba|z|k|da|a)?sh$/,
    load: shell,
  },
  {
    id: 'powershell',
    name: 'PowerShell',
    group: 'code',
    extensions: ['ps1', 'psm1', 'psd1'],
    shebang: /^pwsh$/,
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/powershell')).powerShell),
  },
  { id: 'batch', name: 'Windows Batch', group: 'code', extensions: ['bat', 'cmd'], load: custom('batch') },

  // ---- Web
  { id: 'html', name: 'HTML', group: 'web', extensions: ['html', 'htm', 'xhtml', 'shtml'], load: async () => (await import('@codemirror/lang-html')).html() },
  { id: 'css', name: 'CSS', group: 'web', extensions: ['css'], load: async () => (await import('@codemirror/lang-css')).css() },
  { id: 'scss', name: 'SCSS', group: 'web', extensions: ['scss'], load: async () => (await import('@codemirror/lang-sass')).sass() },
  {
    id: 'sass',
    name: 'Sass',
    group: 'web',
    extensions: ['sass'],
    load: async () => (await import('@codemirror/lang-sass')).sass({ indented: true }),
  },
  { id: 'less', name: 'Less', group: 'web', extensions: ['less'], load: async () => (await import('@codemirror/lang-less')).less() },
  {
    id: 'javascript',
    name: 'JavaScript',
    group: 'web',
    extensions: ['js', 'mjs', 'cjs'],
    shebang: /^(node|nodejs|deno|bun)$/,
    load: js(false, false),
  },
  { id: 'jsx', name: 'React (JSX)', group: 'web', extensions: ['jsx'], load: js(true, false) },
  { id: 'typescript', name: 'TypeScript', group: 'web', extensions: ['ts', 'mts', 'cts'], load: js(false, true) },
  { id: 'tsx', name: 'React (TSX)', group: 'web', extensions: ['tsx'], load: js(true, true) },
  {
    id: 'svelte',
    name: 'Svelte',
    group: 'web',
    extensions: ['svelte'],
    load: async () => (await import('@replit/codemirror-lang-svelte')).svelte(),
  },
  { id: 'markdown', name: 'Markdown', group: 'web', extensions: ['md', 'markdown', 'mdown', 'mkd'], load: async () => (await import('@codemirror/lang-markdown')).markdown() },
  { id: 'sql', name: 'SQL', group: 'web', extensions: ['sql'], load: async () => (await import('@codemirror/lang-sql')).sql() },

  // ---- Data and configuration formats
  {
    id: 'json',
    name: 'JSON',
    group: 'config',
    extensions: ['json', 'webmanifest', 'har', 'geojson', 'ipynb'],
    filenames: ['composer.lock', '.prettierrc'],
    load: async () => (await import('@codemirror/lang-json')).json(),
  },
  {
    id: 'jsonc',
    name: 'JSON with comments / JSON5',
    group: 'config',
    extensions: ['jsonc', 'json5'],
    filenames: ['tsconfig.json', 'jsconfig.json', '.eslintrc', '.eslintrc.json', '.babelrc', '.jshintrc', 'devcontainer.json'],
    pathPatterns: [/\/\.vscode\/[^/]+\.json$/],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/javascript')).json),
  },
  {
    id: 'yaml',
    name: 'YAML',
    group: 'config',
    extensions: ['yaml', 'yml'],
    filenames: ['.clang-format', '.clang-tidy', '.yamllint'],
    load: async () => (await import('@codemirror/lang-yaml')).yaml(),
  },
  {
    id: 'toml',
    name: 'TOML',
    group: 'config',
    extensions: ['toml'],
    filenames: ['Cargo.lock', 'Pipfile', 'poetry.lock', 'uv.lock'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/toml')).toml),
  },
  {
    id: 'xml',
    name: 'XML',
    group: 'config',
    extensions: [
      'xml', 'xsd', 'xsl', 'xslt', 'svg', 'plist', 'csproj', 'vbproj', 'fsproj', 'vcxproj', 'props',
      'targets', 'xaml', 'resx', 'config', 'wsdl', 'rss', 'atom', 'kml', 'gpx', 'xul', 'ui', 'glade',
      'manifest', 'nuspec', 'pom',
    ],
    load: async () => (await import('@codemirror/lang-xml')).xml(),
  },
  {
    id: 'ini',
    name: 'INI / Conf / systemd',
    group: 'config',
    extensions: [
      'ini', 'cfg', 'conf', 'cnf', 'properties', 'prefs', 'desktop', 'directory', 'service', 'socket',
      'timer', 'target', 'mount', 'automount', 'path', 'slice', 'scope', 'network', 'netdev', 'link',
      'inf', 'url', 'lnk', 'env', 'npmrc', 'cfg.example', 'conf.example', 'ini.example',
    ],
    filenames: [
      '.editorconfig', '.gitconfig', '.gitmodules', '.gitattributes', '.npmrc', '.yarnrc', '.pypirc',
      'pip.conf', 'my.cnf', 'php.ini', 'setup.cfg', 'tox.ini', 'pytest.ini', 'mypy.ini', 'desktop.ini',
      '.flake8', '.pylintrc', '.coveragerc', 'smb.conf', 'pacman.conf', 'yum.conf', 'dnf.conf',
    ],
    pathPatterns: [/\/\.env(\.[^/]*)?$/, /\/\.git\/config$/, /\/systemd\/.+\.(conf|d\/[^/]+)$/],
    load: iniLike,
  },
  {
    id: 'nginx',
    name: 'nginx',
    group: 'config',
    extensions: ['nginx', 'nginx.conf', 'vhost'],
    filenames: ['nginx.conf', 'mime.types', 'fastcgi_params', 'fastcgi.conf', 'proxy_params', 'uwsgi_params', 'scgi_params'],
    pathPatterns: [/\/nginx\/(sites-(available|enabled)|conf\.d|snippets)\//],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/nginx')).nginx),
  },
  {
    id: 'apache',
    name: 'Apache httpd',
    group: 'config',
    filenames: ['httpd.conf', 'apache2.conf', 'apache.conf', '.htaccess', '.htpasswd', 'ports.conf'],
    pathPatterns: [/\/(apache2?|httpd)\/(sites-(available|enabled)|conf(-available|-enabled|\.d|\.modules\.d)?|mods-(available|enabled))\//],
    load: custom('apache'),
  },
  {
    id: 'sshconfig',
    name: 'SSH config',
    group: 'config',
    filenames: ['ssh_config', 'sshd_config'],
    pathPatterns: [/\/\.ssh\/config$/, /\/ssh\/(ssh|sshd)_config\.d\//],
    load: custom('sshConfig'),
  },
  {
    id: 'crontab',
    name: 'crontab',
    group: 'config',
    filenames: ['crontab', 'anacrontab'],
    pathPatterns: [/\/cron\.d\//, /\/crontabs\//],
    load: custom('crontab'),
  },
  {
    id: 'conf',
    name: 'Config (whitespace separated)',
    group: 'config',
    filenames: [
      'hosts', 'fstab', 'crypttab', 'resolv.conf', 'exports', 'hosts.allow', 'hosts.deny', 'networks',
      'protocols', 'services', 'nsswitch.conf', 'sudoers', 'login.defs', 'limits.conf', 'modules',
      'environment', 'inittab', 'shells', 'go.mod', 'go.work', 'requirements.txt',
    ],
    pathPatterns: [/\/sudoers\.d\//, /\/modprobe\.d\//, /\/security\/[^/]+\.conf$/],
    load: custom('genericConf'),
  },
  {
    id: 'dockerfile',
    name: 'Dockerfile',
    group: 'config',
    extensions: ['dockerfile', 'containerfile'],
    filenames: ['Dockerfile', 'Containerfile'],
    pathPatterns: [/\/(Dockerfile|Containerfile)\.[^/]+$/],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/dockerfile')).dockerFile),
  },
  {
    id: 'makefile',
    name: 'Makefile',
    group: 'config',
    extensions: ['mk', 'mak', 'make'],
    filenames: ['Makefile', 'GNUmakefile', 'makefile', 'Kbuild', 'Makefile.am', 'Makefile.in'],
    load: custom('makefile'),
  },
  {
    id: 'cmake',
    name: 'CMake',
    group: 'config',
    extensions: ['cmake'],
    filenames: ['CMakeLists.txt'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/cmake')).cmake),
  },
  {
    id: 'ignore',
    name: 'Ignore file',
    group: 'config',
    filenames: ['.gitignore', '.dockerignore', '.npmignore', '.hgignore', '.prettierignore', '.eslintignore', '.stignore'],
    pathPatterns: [/\/\.git\/info\/exclude$/],
    load: custom('ignoreFile'),
  },
  { id: 'reg', name: 'Windows Registry', group: 'config', extensions: ['reg'], load: custom('winRegistry') },
  {
    id: 'protobuf',
    name: 'Protocol Buffers',
    group: 'config',
    extensions: ['proto'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/protobuf')).protobuf),
  },
  {
    id: 'jinja2',
    name: 'Jinja2 / Ansible template',
    group: 'config',
    extensions: ['j2', 'jinja', 'jinja2'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/jinja2')).jinja2),
  },
  {
    id: 'rpmspec',
    name: 'RPM spec',
    group: 'config',
    extensions: ['spec'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/rpm')).rpmSpec),
  },
  {
    id: 'diff',
    name: 'Diff / Patch',
    group: 'config',
    extensions: ['diff', 'patch', 'rej'],
    load: () => legacy(async () => (await import('@codemirror/legacy-modes/mode/diff')).diff),
  },
];

/** Files without highlighting that still open in the editor on double-click. */
const PLAIN_TEXT_EXTENSIONS = new Set([
  'txt', 'text', 'log', 'out', 'csv', 'tsv', 'nfo', 'asc', 'pem', 'crt', 'csr', 'key', 'pub', 'srt', 'sub',
  'vtt', 'lst', 'list', 'rst', 'adoc', 'asciidoc', 'org', 'tex', 'bib', 'sum', 'sha256', 'md5', 'gpg',
  'lock', 'example', 'sample', 'template', 'tmpl', 'tpl', 'dist', 'bak', 'orig', 'old', 'in',
]);
const PLAIN_TEXT_NAMES = new Set(
  ['README', 'LICENSE', 'LICENCE', 'COPYING', 'AUTHORS', 'CHANGELOG', 'CHANGES', 'NEWS', 'TODO', 'INSTALL',
   'NOTICE', 'CONTRIBUTORS', 'MAINTAINERS', 'VERSION', 'CODEOWNERS', 'motd', 'issue', 'hostname',
   'machine-id', 'passwd', 'group', 'shadow', 'authorized_keys', 'known_hosts', 'id_rsa', 'id_ed25519',
   'id_ecdsa', 'timezone', 'locale.gen', 'vconsole.conf', 'os-release', 'mailname', 'Procfile', 'Brewfile',
   'Gemfile', 'Rakefile', 'Vagrantfile', 'Jenkinsfile', 'Caddyfile', 'Justfile', 'justfile'].map((n) => n.toLowerCase())
);

const byId = new Map(LANGUAGES.map((l) => [l.id, l]));

export function languageById(id: string): LanguageDef | undefined {
  return byId.get(id);
}

function baseName(path: string): string {
  const parts = path.replace(/\\/g, '/').split('/');
  return parts[parts.length - 1] || '';
}

/** Extensions of a file name, longest first ("x.d.ts" → ["d.ts", "ts"]). */
function extensionsOf(name: string): string[] {
  const lower = name.toLowerCase().replace(/^\.+/, '');
  const parts = lower.split('.');
  const out: string[] = [];
  for (let i = 1; i < parts.length; i++) out.push(parts.slice(i).join('.'));
  return out;
}

/** Detects the language of a file from its path and, if given, its first line. */
export function detectLanguage(path: string, firstLine = ''): LanguageDef | undefined {
  const unixPath = path.replace(/\\/g, '/');
  const name = baseName(unixPath);
  const lowerName = name.toLowerCase();

  for (const l of LANGUAGES) {
    if (l.filenames?.some((f) => f.toLowerCase() === lowerName)) return l;
  }
  for (const l of LANGUAGES) {
    if (l.pathPatterns?.some((re) => re.test(unixPath))) return l;
  }
  for (const ext of extensionsOf(name)) {
    const l = LANGUAGES.find((x) => x.extensions?.includes(ext));
    if (l) return l;
  }
  const shebang = /^#!\s*(\S+)(?:\s+(\S+))?/.exec(firstLine);
  if (shebang) {
    let interpreter = baseName(shebang[1]);
    if (interpreter === 'env' && shebang[2]) interpreter = baseName(shebang[2]);
    const l = LANGUAGES.find((x) => x.shebang?.test(interpreter));
    if (l) return l;
  }
  return undefined;
}

/** Whether a double-click should open the file in the editor (known text formats only). */
export function isTextFile(path: string): boolean {
  if (detectLanguage(path)) return true;
  const name = baseName(path.replace(/\\/g, '/'));
  if (PLAIN_TEXT_NAMES.has(name.toLowerCase())) return true;
  return extensionsOf(name).some((ext) => PLAIN_TEXT_EXTENSIONS.has(ext));
}
