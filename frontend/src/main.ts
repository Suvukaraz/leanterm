import { mount } from 'svelte'
import App from './App.svelte'

// The editor window loads the same bundle with "#/editor".
const target = document.getElementById('app')!
if (location.hash.startsWith('#/editor')) {
  import('./components/editor/EditorApp.svelte').then(({ default: EditorApp }) => mount(EditorApp, { target }))
} else {
  mount(App, { target })
}
