import { mount } from 'svelte'
import App from './App.svelte'

// The editor window (LeanEdit) loads the same bundle with "#/editor", its
// preferences window with "#/editor-prefs".
const target = document.getElementById('app')!
if (location.hash.startsWith('#/editor-prefs')) {
  import('./components/editor/EditorPrefsApp.svelte').then(({ default: EditorPrefsApp }) => mount(EditorPrefsApp, { target }))
} else if (location.hash.startsWith('#/editor')) {
  import('./components/editor/EditorApp.svelte').then(({ default: EditorApp }) => mount(EditorApp, { target }))
} else {
  mount(App, { target })
}
