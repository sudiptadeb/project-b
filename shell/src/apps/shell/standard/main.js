import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { connect, vDrag, vPage, vShell } from '@/sdk/vue.js';
import { addressBar } from './state.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
connect({
  suggestions: (m) => addressBar.suggestions(m.id, m.items || []),
  focusAddressBar: () => addressBar.focus(),
});
