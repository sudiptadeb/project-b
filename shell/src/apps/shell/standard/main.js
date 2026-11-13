import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { vDrag, vPage, vShell } from '@/kit/regions.js';
import { connect } from '@/kit/shell.js';
import { addressBar, phone } from './state.js';

document.documentElement.classList.toggle('touch', phone);

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
connect({
  suggestions: (m) => addressBar.suggestions(m.id, m.items || []),
  focusAddressBar: () => addressBar.focus(),
});
