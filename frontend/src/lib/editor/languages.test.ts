import { describe, expect, it } from 'vitest';
import { StringStream, type StreamParser } from '@codemirror/language';
import { LANGUAGES, detectLanguage, isTextFile } from './languages';
import * as modes from './customModes';

const id = (path: string, firstLine = '') => detectLanguage(path, firstLine)?.id;

describe('detectLanguage', () => {
  it('detects by extension', () => {
    expect(id('/src/main.c')).toBe('c');
    expect(id('/src/a.HPP')).toBe('cpp');
    expect(id('/sketch/blink.ino')).toBe('arduino');
    expect(id('Program.cs')).toBe('csharp');
    expect(id('build.gradle.kts')).toBe('kotlin');
    expect(id('lib/main.dart')).toBe('dart');
    expect(id('App.tsx')).toBe('tsx');
    expect(id('App.jsx')).toBe('jsx');
    expect(id('types.d.ts')).toBe('typescript');
    expect(id('Page.svelte')).toBe('svelte');
    expect(id('main.go')).toBe('go');
    expect(id('lib.rs')).toBe('rust');
    expect(id('app.nim')).toBe('nim');
    expect(id('hello.mojo')).toBe('mojo');
    expect(id('hello.🔥')).toBe('mojo');
    expect(id('core.cljs')).toBe('clojure');
    expect(id('init.lua')).toBe('lua');
    expect(id('deploy.sh')).toBe('bash');
    expect(id('setup.ps1')).toBe('powershell');
    expect(id('build.BAT')).toBe('batch');
    expect(id('tweak.reg')).toBe('reg');
    expect(id('docker-compose.yml')).toBe('yaml');
    expect(id('pubspec.yaml')).toBe('yaml');
    expect(id('app.service')).toBe('systemd');
    for (const ext of ['socket', 'device', 'mount', 'automount', 'swap', 'target', 'path', 'timer', 'slice', 'scope']) {
      expect(id(`/etc/systemd/system/x.${ext}`)).toBe('systemd');
    }
    expect(id('/etc/systemd/network/10-eth.network')).toBe('systemd');
    expect(id('/etc/systemd/nspawn/box.nspawn')).toBe('systemd');
    expect(id('/usr/share/applications/firefox.desktop')).toBe('desktop');
    expect(id('/home/u/.config/niri/config.kdl')).toBe('kdl');
    expect(id('/project/meson.build')).toBeUndefined();
    expect(id('web.config')).toBe('xml');
  });

  it('detects by file name and path', () => {
    expect(id('/home/u/.bashrc')).toBe('bash');
    expect(id('/project/Makefile')).toBe('makefile');
    expect(id('/project/Dockerfile')).toBe('dockerfile');
    expect(id('/project/Dockerfile.dev')).toBe('dockerfile');
    expect(id('/project/CMakeLists.txt')).toBe('cmake');
    expect(id('/project/.gitignore')).toBe('ignore');
    expect(id('/project/tsconfig.json')).toBe('jsonc');
    expect(id('/project/package.json')).toBe('json');
    expect(id('/project/Cargo.lock')).toBe('toml');
    expect(id('/etc/nginx/nginx.conf')).toBe('nginx');
    expect(id('/home/u/conf/site.nginx.conf')).toBe('nginx');
    expect(id('/etc/nginx/sites-available/default')).toBe('nginx');
    expect(id('/etc/apache2/sites-enabled/000-default.conf')).toBe('apache');
    expect(id('/var/www/.htaccess')).toBe('apache');
    expect(id('/etc/ssh/sshd_config')).toBe('sshconfig');
    expect(id('/home/u/.ssh/config')).toBe('sshconfig');
    expect(id('/etc/cron.d/backup')).toBe('crontab');
    expect(id('/etc/fstab')).toBe('conf');
    expect(id('/etc/hosts')).toBe('conf');
    expect(id('C:\\Windows\\System32\\drivers\\etc\\hosts')).toBe('conf');
    expect(id('/project/.env.local')).toBe('ini');
    expect(id('/repo/.git/config')).toBe('ini');
    expect(id('/etc/systemd/journald.conf')).toBe('systemd');
    expect(id('/etc/systemd/system/nginx.service.d/override.conf')).toBe('systemd');
    expect(id('/etc/containers/systemd/web.container')).toBe('systemd');
    expect(id('/home/u/.config/mimeapps.list')).toBe('desktop');
  });

  it('detects by shebang', () => {
    expect(id('/usr/local/bin/backup', '#!/bin/bash')).toBe('bash');
    expect(id('/usr/local/bin/tool', '#!/usr/bin/env python3')).toBe('python');
    expect(id('/usr/local/bin/tool', '#!/usr/bin/env node')).toBe('javascript');
    expect(id('/usr/local/bin/blob')).toBeUndefined();
  });
});

describe('isTextFile', () => {
  it('accepts known and plain text files only', () => {
    expect(isTextFile('/var/log/syslog.log')).toBe(true);
    expect(isTextFile('/repo/README')).toBe(true);
    expect(isTextFile('/repo/notes.txt')).toBe(true);
    expect(isTextFile('/repo/main.py')).toBe(true);
    expect(isTextFile('/home/u/photo.jpg')).toBe(false);
    expect(isTextFile('/usr/bin/ls')).toBe(false);
    expect(isTextFile('/home/u/archive.tar.gz')).toBe(false);
  });
});

describe('language loaders', () => {
  it('every language loads its CodeMirror support', async () => {
    for (const lang of LANGUAGES) {
      await expect(lang.load(), lang.id).resolves.toBeTruthy();
    }
  }, 30000);
});

/** Tokenizes text line by line and returns [token text, style] pairs (styled tokens only). */
function tokens(parser: StreamParser<any>, text: string): Array<[string, string]> {
  const state = parser.startState ? parser.startState(2) : {};
  const out: Array<[string, string]> = [];
  for (const line of text.split('\n')) {
    const stream = new StringStream(line, 2, 2);
    while (!stream.eol()) {
      const style = parser.token(stream, state);
      if (style) out.push([stream.current(), style]);
      stream.start = stream.pos;
    }
  }
  return out;
}

const has = (list: Array<[string, string]>, text: string, style: string) =>
  list.some(([t, s]) => t.trim() === text && s === style);

describe('custom modes', () => {
  it('nim', () => {
    const t = tokens(modes.nim, 'proc greet(name: string) =\n  echo "hi" # say hi\n#[ block\n comment ]#\nlet x = 0x1F');
    expect(has(t, 'proc', 'keyword')).toBe(true);
    expect(has(t, 'greet', 'def')).toBe(true);
    expect(has(t, 'string', 'typeName')).toBe(true);
    expect(has(t, '"hi"', 'string')).toBe(true);
    expect(has(t, '# say hi', 'comment')).toBe(true);
    expect(has(t, 'comment ]#', 'comment')).toBe(true);
    expect(has(t, '0x1F', 'number')).toBe(true);
  });

  it('mojo', () => {
    const t = tokens(modes.mojo, 'fn add(x: Int, y: Int) -> Int:\n    var s = """doc"""\n    return x + y  # sum');
    expect(has(t, 'fn', 'keyword')).toBe(true);
    expect(has(t, 'add', 'def')).toBe(true);
    expect(has(t, 'Int', 'builtin')).toBe(true);
    expect(has(t, 'var', 'keyword')).toBe(true);
    expect(has(t, '# sum', 'comment')).toBe(true);
  });

  it('makefile', () => {
    const t = tokens(modes.makefile, 'CC := gcc\nall: main.o\n\t$(CC) -o $@ $^\nifeq ($(X),1)\nendif');
    expect(has(t, 'CC', 'def')).toBe(true);
    expect(has(t, 'all', 'def')).toBe(true);
    expect(has(t, '$(CC)', 'variableName')).toBe(true);
    expect(has(t, '$@', 'variableName')).toBe(true);
    expect(has(t, 'ifeq', 'keyword')).toBe(true);
  });

  it('batch', () => {
    const t = tokens(modes.batch, '@echo off\nREM comment\n:loop\nset X=%PATH%\nfor %%i in (*) do echo %%i');
    expect(has(t, 'echo', 'keyword')).toBe(true);
    expect(has(t, 'REM comment', 'comment')).toBe(true);
    expect(has(t, ':loop', 'def')).toBe(true);
    expect(has(t, '%PATH%', 'variableName')).toBe(true);
    expect(has(t, '%%i', 'variableName')).toBe(true);
  });

  it('windows registry', () => {
    const t = tokens(modes.winRegistry, 'Windows Registry Editor Version 5.00\n[HKEY_CURRENT_USER\\Software\\X]\n"Name"="value"\n"Count"=dword:00000001');
    expect(has(t, 'Windows Registry Editor Version 5.00', 'meta')).toBe(true);
    expect(has(t, '[HKEY_CURRENT_USER\\Software\\X]', 'heading')).toBe(true);
    expect(has(t, '"Name"', 'propertyName')).toBe(true);
    expect(has(t, '"value"', 'string')).toBe(true);
    expect(has(t, 'dword:', 'typeName')).toBe(true);
  });

  it('ssh config, crontab, generic conf', () => {
    const ssh = tokens(modes.sshConfig, 'Host web*\n  HostName 10.0.0.1\n  ForwardAgent yes');
    expect(has(ssh, 'Host', 'heading')).toBe(true);
    expect(has(ssh, 'web*', 'def')).toBe(true);
    expect(has(ssh, 'HostName', 'keyword')).toBe(true);
    expect(has(ssh, 'yes', 'atom')).toBe(true);

    const cron = tokens(modes.crontab, 'MAILTO=root\n*/5 * * * * /usr/bin/backup\n@reboot /bin/true');
    expect(has(cron, 'MAILTO', 'propertyName')).toBe(true);
    expect(has(cron, '*/5', 'number')).toBe(true);
    expect(has(cron, '@reboot', 'keyword')).toBe(true);

    const conf = tokens(modes.genericConf, '127.0.0.1 localhost # loopback');
    expect(has(conf, '127.0.0.1', 'number')).toBe(true);
    expect(has(conf, '# loopback', 'comment')).toBe(true);
  });

  it('apache and ignore files', () => {
    const a = tokens(modes.apache, '<VirtualHost *:80>\n  ServerName example.com\n  # comment\n</VirtualHost>');
    expect(has(a, '<VirtualHost', 'tagName')).toBe(true);
    expect(has(a, 'ServerName', 'keyword')).toBe(true);
    expect(has(a, '# comment', 'comment')).toBe(true);

    const g = tokens(modes.ignoreFile, '# build output\n!keep.log\n**/*.o');
    expect(has(g, '# build output', 'comment')).toBe(true);
    expect(has(g, '!', 'operator')).toBe(true);
    expect(has(g, '**', 'operator')).toBe(true);
  });

  it('systemd', () => {
    const t = tokens(
      modes.systemd,
      '# unit\n[Service]\nType=notify\nExecStart=-/usr/bin/app --id %i $OPTS \\\n    --verbose\nRestart=on-failure\nRestartSec=5s\nPrivateTmp=yes\nAfter=network.target\nX-Custom=1'
    );
    expect(has(t, '# unit', 'comment')).toBe(true);
    expect(has(t, '[Service]', 'heading')).toBe(true);
    expect(has(t, 'Type', 'keyword')).toBe(true);
    expect(has(t, 'ExecStart', 'keyword')).toBe(true);
    expect(has(t, '-', 'operator')).toBe(true);
    expect(has(t, '%i', 'variableName.special')).toBe(true);
    expect(has(t, '$OPTS', 'variableName')).toBe(true);
    expect(has(t, '\\', 'operator')).toBe(true);
    expect(has(t, '--verbose', 'keyword')).toBe(false);
    expect(has(t, '5s', 'number')).toBe(true);
    expect(has(t, 'yes', 'atom')).toBe(true);
    expect(has(t, 'network.target', 'typeName')).toBe(true);
    expect(has(t, 'X-Custom', 'propertyName')).toBe(true);
  });

  it('desktop entry', () => {
    const t = tokens(modes.desktopEntry, '[Desktop Entry]\nName[de]=Datei\\sManager\nExec=app %U\nTerminal=false\nCategories=Utility;System;\nX-KDE-Foo=1');
    expect(has(t, '[Desktop Entry]', 'heading')).toBe(true);
    expect(has(t, 'Name', 'keyword')).toBe(true);
    expect(has(t, '[de]', 'meta')).toBe(true);
    expect(has(t, '\\s', 'escape')).toBe(true);
    expect(has(t, '%U', 'variableName.special')).toBe(true);
    expect(has(t, 'false', 'atom')).toBe(true);
    expect(has(t, ';', 'punctuation')).toBe(true);
    expect(has(t, 'X-KDE-Foo', 'propertyName')).toBe(true);
  });

  it('kdl', () => {
    const t = tokens(
      modes.kdl,
      '// niri\ninput {\n    keyboard repeat-rate=25 {\n        xkb layout="de"\n    }\n    /-touchpad { tap }\n}\nnode (u8)0x1F #true r#"raw"# /-skipped 1.5\n/* a /* nested */ comment */'
    );
    expect(has(t, '// niri', 'comment')).toBe(true);
    expect(has(t, 'input', 'tagName')).toBe(true);
    expect(has(t, 'keyboard', 'tagName')).toBe(true);
    expect(has(t, 'repeat-rate', 'propertyName')).toBe(true);
    expect(has(t, '25', 'number')).toBe(true);
    expect(has(t, 'xkb', 'tagName')).toBe(true);
    expect(has(t, '"de"', 'string')).toBe(true);
    expect(has(t, 'touchpad', 'comment')).toBe(true);
    expect(has(t, 'tap', 'tagName')).toBe(false);
    expect(has(t, 'node', 'tagName')).toBe(true);
    expect(has(t, '(u8)', 'typeName')).toBe(true);
    expect(has(t, '0x1F', 'number')).toBe(true);
    expect(has(t, '#true', 'atom')).toBe(true);
    expect(has(t, 'r#"raw"#', 'string')).toBe(true);
    expect(has(t, 'skipped', 'comment')).toBe(true);
    expect(has(t, '1.5', 'number')).toBe(true);
    expect(has(t, '/* a /* nested */ comment */', 'comment')).toBe(true);
  });
});
