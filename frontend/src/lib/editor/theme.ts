// Editor look: everything comes from LeanTerm's theme variables (gtk.css), so the
// editor follows the dark/light colour scheme without its own palette.
import { EditorView } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags as t } from '@lezer/highlight';
import type { Extension } from '@codemirror/state';

const v = (name: string) => `var(--gtk-${name})`;

export function editorTheme(dark: boolean): Extension {
  return EditorView.theme(
    {
      '&': {
        color: v('text-color'),
        backgroundColor: v('base-color'),
        height: '100%',
        fontSize: 'var(--editor-font-size, 14px)',
      },
      '.cm-scroller': {
        fontFamily: 'var(--editor-font-family, monospace)',
        fontWeight: 'var(--editor-font-weight, normal)',
        fontStyle: 'var(--editor-font-style, normal)',
        fontStretch: 'var(--editor-font-stretch, normal)',
        fontFeatureSettings: 'var(--editor-font-features, normal)',
        lineHeight: '1.45',
      },
      '.cm-content': { caretColor: v('text-color') },
      '.cm-cursor, .cm-dropCursor': { borderLeftColor: v('text-color'), borderLeftWidth: '2px' },
      '&.cm-overwrite .cm-cursor': { borderLeftWidth: '0.6em', opacity: '0.5' },
      '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection':
        { backgroundColor: v('editor-selection') },
      '.cm-gutters': {
        backgroundColor: v('header-bg'),
        color: v('muted-fg'),
        borderRight: `1px solid ${v('border-color')}`,
      },
      '.cm-activeLine': { backgroundColor: v('editor-active-line') },
      '.cm-activeLineGutter': { backgroundColor: v('editor-active-line'), color: v('fg-color') },
      '.cm-foldPlaceholder': {
        backgroundColor: v('btn-bg'),
        border: `1px solid ${v('border-color')}`,
        color: v('muted-fg'),
      },
      '.cm-searchMatch': { backgroundColor: v('editor-search-match'), outline: `1px solid ${v('warning-fg')}` },
      '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: v('editor-selection') },
      '.cm-selectionMatch': { backgroundColor: v('editor-search-match') },
      '&.cm-focused .cm-matchingBracket': { backgroundColor: v('editor-selection'), outline: `1px solid ${v('accent')}` },
      '&.cm-focused .cm-nonmatchingBracket': { backgroundColor: v('error-bg') },
      '.cm-highlightSpace': { backgroundImage: `radial-gradient(circle at 50% 55%, ${v('muted-fg')} 10%, transparent 12%)` },
      '.cm-highlightTab': { color: v('muted-fg') },
      '.cm-specialChar': { color: v('syntax-invalid') },
      '.cm-panels': { backgroundColor: v('header-bg'), color: v('fg-color') },
      '.cm-panels.cm-panels-top': { borderBottom: `1px solid ${v('border-color')}` },
      '.cm-panels.cm-panels-bottom': { borderTop: `1px solid ${v('border-color')}` },
      '.cm-panel input[type=text], .cm-panel .cm-textfield': {
        backgroundColor: v('base-color'),
        color: v('text-color'),
        border: `1px solid ${v('border-color')}`,
        borderRadius: '3px',
        padding: '2px 6px',
        fontSize: 'inherit',
      },
      '.cm-panel button, .cm-panel .cm-button': {
        backgroundColor: v('btn-bg'),
        backgroundImage: 'none',
        color: v('fg-color'),
        border: `1px solid ${v('btn-border')}`,
        borderRadius: '3px',
        padding: '2px 8px',
        fontSize: 'inherit',
      },
      '.cm-panel label': { fontSize: 'inherit' },
      '.cm-panel.cm-search [name=close]': { color: v('fg-color') },
      '.cm-tooltip': {
        backgroundColor: v('menu-bg'),
        color: v('fg-color'),
        border: `1px solid ${v('border-color')}`,
      },
    },
    { dark }
  );
}

const highlightStyle = HighlightStyle.define([
  { tag: [t.keyword, t.controlKeyword, t.moduleKeyword, t.operatorKeyword, t.definitionKeyword, t.modifier], color: v('syntax-keyword'), fontWeight: 'bold' },
  { tag: [t.string, t.special(t.string), t.character, t.regexp, t.docString], color: v('syntax-string') },
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: v('syntax-comment'), fontStyle: 'italic' },
  { tag: [t.number, t.integer, t.float], color: v('syntax-number') },
  { tag: [t.typeName, t.className, t.namespace, t.standard(t.typeName)], color: v('syntax-type') },
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.function(t.variableName)), t.macroName], color: v('syntax-function') },
  { tag: [t.definition(t.variableName), t.definition(t.propertyName)], color: v('syntax-function') },
  { tag: [t.bool, t.null, t.atom, t.self, t.constant(t.variableName), t.special(t.variableName), t.unit], color: v('syntax-constant') },
  { tag: [t.standard(t.variableName), t.standard(t.name)], color: v('syntax-type') },
  { tag: [t.operator, t.derefOperator, t.arithmeticOperator, t.logicOperator, t.bitwiseOperator, t.compareOperator, t.updateOperator], color: v('syntax-operator') },
  { tag: [t.tagName, t.angleBracket], color: v('syntax-tag') },
  { tag: [t.attributeName], color: v('syntax-attribute') },
  { tag: [t.propertyName], color: v('syntax-property') },
  { tag: [t.meta, t.processingInstruction, t.annotation, t.documentMeta], color: v('syntax-meta') },
  { tag: [t.heading, t.heading1, t.heading2, t.heading3], color: v('syntax-keyword'), fontWeight: 'bold' },
  { tag: [t.link, t.url], color: v('link-fg'), textDecoration: 'underline' },
  { tag: t.emphasis, fontStyle: 'italic' },
  { tag: t.strong, fontWeight: 'bold' },
  { tag: t.strikethrough, textDecoration: 'line-through' },
  { tag: [t.inserted], color: v('syntax-string') },
  { tag: [t.deleted, t.invalid], color: v('syntax-invalid') },
  { tag: [t.changed], color: v('syntax-number') },
  { tag: [t.labelName], color: v('syntax-function') },
  { tag: [t.escape], color: v('syntax-constant') },
]);

export const leanHighlighting: Extension = syntaxHighlighting(highlightStyle);
