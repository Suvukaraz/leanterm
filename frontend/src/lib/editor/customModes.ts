// Syntax modes for formats CodeMirror has no package for (CodeMirror 5 style
// stream parsers, wrapped with StreamLanguage in languages.ts).
import type { StringStream, StreamParser } from '@codemirror/language';

const words = (list: string) => new Set(list.split(/\s+/).filter(Boolean));

function hashComments(name: string, token: StreamParser<unknown>['token']): StreamParser<unknown> {
  return { name, token, languageData: { commentTokens: { line: '#' } } };
}

function readQuoted(stream: StringStream, quote: string): void {
  let escaped = false;
  let ch: string | void;
  while ((ch = stream.next()) != null) {
    if (ch === quote && !escaped) return;
    escaped = !escaped && ch === '\\';
  }
}

// ---- Nim ---------------------------------------------------------------------

const NIM_KEYWORDS = words(`addr and as asm bind block break case cast concept const continue converter
  defer discard distinct div do elif else end enum except export finally for from func if import in
  include interface is isnot iterator let macro method mixin mod nil not notin object of or out proc
  ptr raise ref return shl shr static template try tuple type using var when while xor yield`);
const NIM_TYPES = words(`int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float float32 float64
  bool char string cstring pointer seq array set openArray varargs range auto void untyped typed
  Natural Positive byte`);
const NIM_ATOMS = words('true false nil');

interface NimState {
  comment: number;
  string: '' | '"""';
  defNext: boolean;
}

export const nim: StreamParser<NimState> = {
  name: 'nim',
  startState: () => ({ comment: 0, string: '', defNext: false }),
  token(stream, state) {
    if (state.comment > 0) {
      while (!stream.eol()) {
        if (stream.match('#[')) state.comment++;
        else if (stream.match(']#')) {
          state.comment--;
          if (state.comment === 0) break;
        } else stream.next();
      }
      return 'comment';
    }
    if (state.string) {
      if (stream.skipTo('"""')) {
        stream.match('"""');
        state.string = '';
      } else stream.skipToEnd();
      return 'string';
    }
    if (stream.eatSpace()) return null;
    if (stream.match('#[') || stream.match('##[')) {
      state.comment = 1;
      return 'comment';
    }
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.match(/^[rR]?"""/)) {
      state.string = '"""';
      return 'string';
    }
    if (stream.match(/^[rR]?"/)) {
      readQuoted(stream, '"');
      return 'string';
    }
    if (stream.match(/^'(\\.|[^'\\])'/)) return 'string';
    if (stream.match(/^\{\./)) {
      stream.skipTo('.}') ? stream.match('.}') : stream.skipToEnd();
      return 'meta';
    }
    if (stream.match(/^(0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|\d[\d_]*(\.[\d_]+)?([eE][+-]?\d+)?)('?[iIuUfF]\d*)?/)) {
      return 'number';
    }
    if (stream.match(/^[A-Za-z_][\w]*/)) {
      const w = stream.current();
      if (state.defNext) {
        state.defNext = false;
        return 'def';
      }
      if (NIM_KEYWORDS.has(w)) {
        state.defNext = /^(proc|func|method|iterator|template|macro|converter|type)$/.test(w);
        return 'keyword';
      }
      if (NIM_ATOMS.has(w)) return 'atom';
      if (NIM_TYPES.has(w) || /^[A-Z]/.test(w)) return 'typeName';
      return 'variableName';
    }
    if (stream.match(/^[=+\-*/<>@$~&%|!?^.:\\]+/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#', block: { open: '#[', close: ']#' } } },
};

// ---- Mojo (Python with Mojo's additions) ----------------------------------------

const MOJO_KEYWORDS = words(`and as assert async await break class continue def del elif else except finally
  for from global if import in is lambda match case nonlocal not or pass raise return try while with
  yield fn struct trait var let alias inout owned borrowed mut ref out raises capturing escaping
  comptime self Self`);
const MOJO_BUILTINS = words(`print len range int str float bool list dict set tuple abs min max sum any all
  Int Int8 Int16 Int32 Int64 UInt UInt8 UInt16 UInt32 UInt64 Float16 Float32 Float64 Bool String
  StringLiteral SIMD DType List Dict Optional Pointer UnsafePointer object AnyType`);

interface PyState {
  string: string;
  defNext: boolean;
}

export const mojo: StreamParser<PyState> = {
  name: 'mojo',
  startState: () => ({ string: '', defNext: false }),
  token(stream, state) {
    if (state.string) {
      if (stream.skipTo(state.string)) {
        stream.match(state.string);
        state.string = '';
      } else stream.skipToEnd();
      return 'string';
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    const triple = stream.match(/^[rRbBfFuU]{0,2}("""|''')/) as RegExpMatchArray | null;
    if (triple) {
      state.string = triple[1];
      return 'string';
    }
    const single = stream.match(/^[rRbBfFuU]{0,2}(["'])/) as RegExpMatchArray | null;
    if (single) {
      readQuoted(stream, single[1]);
      return 'string';
    }
    if (stream.match(/^@[\w.]+/)) return 'meta';
    if (stream.match(/^(0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|\d[\d_]*(\.[\d_]*)?([eE][+-]?\d+)?j?)/)) {
      return 'number';
    }
    if (stream.match(/^[A-Za-z_]\w*/)) {
      const w = stream.current();
      if (state.defNext) {
        state.defNext = false;
        return 'def';
      }
      if (w === 'True' || w === 'False' || w === 'None') return 'atom';
      if (MOJO_KEYWORDS.has(w)) {
        state.defNext = /^(def|fn|class|struct|trait|alias)$/.test(w);
        return 'keyword';
      }
      if (MOJO_BUILTINS.has(w)) return 'builtin';
      return 'variableName';
    }
    if (stream.match(/^[=+\-*/<>!&|^~%:.@-]+/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- Makefile ------------------------------------------------------------------

const MAKE_DIRECTIVES = words(`include -include sinclude ifeq ifneq ifdef ifndef else endif define endef
  export unexport override undefine vpath private`);

function makeVariable(stream: StringStream): boolean {
  if (stream.match(/^\$[@<^+?*%|]/) || stream.match(/^\$\$/)) return true;
  const open = stream.match(/^\$([({])/) as RegExpMatchArray | null;
  if (!open) return false;
  const close = open[1] === '(' ? ')' : '}';
  let depth = 1;
  let ch: string | void;
  while ((ch = stream.next()) != null) {
    if (ch === open[1]) depth++;
    else if (ch === close && --depth === 0) break;
  }
  return true;
}

export const makefile: StreamParser<unknown> = hashComments('makefile', (stream) => {
  if (stream.sol() && stream.peek() === '\t') {
    // Recipe line (shell commands): only variables and comments stand out.
    stream.next();
  }
  if (stream.eatSpace()) return null;
  if (stream.peek() === '#') {
    stream.skipToEnd();
    return 'comment';
  }
  if (makeVariable(stream)) return 'variableName';
  if (stream.sol() || stream.string.slice(0, stream.pos).trim() === '') {
    if (stream.match(/^[\w.\-/%$(){}]+(?=\s*(::?=|\?=|\+=|!=|=))/)) return 'def';
    if (stream.match(/^[^\s:=#]+(?=\s*::?(?!=))/)) return 'def';
  }
  if (stream.match(/^[A-Za-z_-]+/)) {
    return MAKE_DIRECTIVES.has(stream.current()) ? 'keyword' : null;
  }
  if (stream.match(/^["']/)) {
    readQuoted(stream, stream.current());
    return 'string';
  }
  if (stream.match(/^(::?=|\?=|\+=|!=|=|::?|\|)/)) return 'operator';
  stream.next();
  return null;
});

// ---- Windows batch (.bat/.cmd) -----------------------------------------------------

const BATCH_KEYWORDS = words(`if else not exist defined errorlevel equ neq lss leq gtr geq for in do goto
  call set setlocal endlocal enabledelayedexpansion disabledelayedexpansion enableextensions shift exit
  echo pause start cd chdir pushd popd md mkdir rd rmdir del erase copy xcopy robocopy move ren rename
  type cls title color choice timeout where findstr find dir path prompt ver vol break nul con`);

export const batch: StreamParser<unknown> = {
  name: 'batch',
  token(stream) {
    if (stream.sol()) {
      stream.eatSpace();
      if (stream.match(/^(rem(\s|$)|::)/i)) {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^@/)) return 'operator';
      if (stream.match(/^:[^\s:]\S*/)) return 'def';
    }
    if (stream.eatSpace()) return null;
    if (stream.match(/^%%~?[a-zA-Z]/) || stream.match(/^%~?[0-9*]/) || stream.match(/^%[^%\s]+%/) || stream.match(/^![^!\s]+!/)) {
      return 'variableName';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return 'string';
    }
    if (stream.match(/^\d+\b/)) return 'number';
    if (stream.match(/^[A-Za-z_][\w.-]*/)) {
      return BATCH_KEYWORDS.has(stream.current().toLowerCase()) ? 'keyword' : null;
    }
    if (stream.match(/^(==|[<>|&]+|\^)/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: 'REM ' } },
};

// ---- Windows registry (.reg) -----------------------------------------------------------

export const winRegistry: StreamParser<unknown> = {
  name: 'reg',
  token(stream) {
    if (stream.sol()) {
      if (stream.match(/^(Windows Registry Editor Version [\d.]+|REGEDIT4)\s*$/)) return 'meta';
      if (stream.match(/^\[-?[^\]]*\]/)) return 'heading';
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === ';') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return stream.match(/^\s*=/, false) ? 'propertyName' : 'string';
    }
    if (stream.match(/^@(?=\s*=)/)) return 'propertyName';
    if (stream.match(/^(dword|qword|hex(\([0-9a-fA-F]+\))?):/i)) return 'typeName';
    if (stream.match(/^[0-9a-fA-F]+/)) return 'number';
    if (stream.match(/^[=,\\-]/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: ';' } },
};

// ---- ssh_config / sshd_config -------------------------------------------------------------

export const sshConfig: StreamParser<{ blockLine: boolean }> = {
  name: 'sshconfig',
  startState: () => ({ blockLine: false }),
  token(stream, state) {
    if (stream.sol()) {
      state.blockLine = false;
      stream.eatSpace();
      if (stream.peek() === '#') {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^[A-Za-z][A-Za-z0-9]*/)) {
        state.blockLine = /^(host|match)$/i.test(stream.current());
        return state.blockLine ? 'heading' : 'keyword';
      }
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"') {
      stream.next();
      readQuoted(stream, '"');
      return 'string';
    }
    if (state.blockLine && stream.match(/^\S+/)) return 'def';
    if (stream.match(/^(yes|no|any|none|auto|ask)\b/i)) return 'atom';
    if (stream.match(/^\d+[smhdwKMG]?\b/)) return 'number';
    if (stream.match(/^%[a-zA-Z%]/) || stream.match(/^\$\{[^}]*\}/)) return 'variableName';
    if (stream.match(/^=/)) return 'operator';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- crontab ---------------------------------------------------------------------------

export const crontab: StreamParser<{ field: number }> = {
  name: 'crontab',
  startState: () => ({ field: 0 }),
  token(stream, state) {
    if (stream.sol()) {
      state.field = 0;
      stream.eatSpace();
      if (stream.peek() === '#') {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match(/^[A-Za-z_]\w*(?=\s*=)/)) {
        state.field = -1;
        return 'propertyName';
      }
      if (stream.match(/^@(reboot|yearly|annually|monthly|weekly|daily|midnight|hourly)\b/)) {
        state.field = 5;
        return 'keyword';
      }
    }
    if (stream.eatSpace()) return null;
    if (state.field >= 0 && state.field < 5 && stream.match(/^\S+/)) {
      state.field++;
      return 'number';
    }
    if (stream.match(/^=/)) return 'operator';
    if (stream.peek() === '"' || stream.peek() === "'") {
      const q = stream.next() as string;
      readQuoted(stream, q);
      return 'string';
    }
    if (stream.match(/^\$\{?\w+\}?/)) return 'variableName';
    stream.next();
    return null;
  },
  languageData: { commentTokens: { line: '#' } },
};

// ---- Apache httpd.conf / .htaccess ------------------------------------------------------

export const apache: StreamParser<unknown> = hashComments('apache', (stream) => {
  if (stream.sol()) stream.eatSpace();
  if (stream.peek() === '#' && stream.string.slice(0, stream.pos).trim() === '') {
    stream.skipToEnd();
    return 'comment';
  }
  if (stream.match(/^<\/?[A-Za-z]+/)) return 'tagName';
  if (stream.match(/^>/)) return 'tagName';
  if (stream.eatSpace()) return null;
  if (stream.string.slice(0, stream.pos).trim() === '' && stream.match(/^[A-Za-z][\w]*/)) return 'keyword';
  if (stream.peek() === '"' || stream.peek() === "'") {
    const q = stream.next() as string;
    readQuoted(stream, q);
    return 'string';
  }
  if (stream.match(/^[%$]\{[^}]*\}/)) return 'variableName';
  if (stream.match(/^\b(on|off|all|none)\b/i)) return 'atom';
  if (stream.match(/^\d+\b/)) return 'number';
  stream.next();
  return null;
});

// ---- .gitignore and friends --------------------------------------------------------------

export const ignoreFile: StreamParser<unknown> = hashComments('ignore', (stream) => {
  if (stream.sol() && stream.peek() === '#') {
    stream.skipToEnd();
    return 'comment';
  }
  if (stream.sol() && stream.match(/^!/)) return 'operator';
  if (stream.match(/^(\*\*|\*|\?|\[[^\]]*\])/)) return 'operator';
  if (stream.match(/^\//)) return 'punctuation';
  stream.next();
  return null;
});

// ---- Generic whitespace-separated config (hosts, fstab, resolv.conf, ...) ----------

export const genericConf: StreamParser<{ first: boolean }> = {
  name: 'conf',
  startState: () => ({ first: true }),
  token(stream, state) {
    if (stream.sol()) state.first = true;
    if (stream.eatSpace()) return null;
    if (stream.peek() === '#' || stream.peek() === ';') {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.peek() === '"' || stream.peek() === "'") {
      const q = stream.next() as string;
      readQuoted(stream, q);
      state.first = false;
      return 'string';
    }
    if (stream.match(/^(\d{1,3}\.){3}\d{1,3}(\/\d+)?/) || stream.match(/^[0-9a-fA-F]*:[0-9a-fA-F:]+(\/\d+)?/)) {
      state.first = false;
      return 'number';
    }
    if (stream.match(/^\d+(\.\d+)?\b/)) {
      state.first = false;
      return 'number';
    }
    if (stream.match(/^[^\s#;"'=]+/)) {
      const style = state.first ? 'keyword' : null;
      state.first = false;
      return style;
    }
    stream.next();
    return 'operator';
  },
  languageData: { commentTokens: { line: '#' } },
};
