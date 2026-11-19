import { mount } from 'svelte'
import '../lib/theme/gtk.css'
import Gallery from './Gallery.svelte'

const theme = new URLSearchParams(location.search).get('theme') === 'light' ? 'light' : 'dark'
document.documentElement.dataset.theme = theme
mount(Gallery, { target: document.getElementById('app')! })
