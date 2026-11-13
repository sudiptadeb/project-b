import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { vDrag, vPage, vShell } from '@/kit/regions.js';
import { connect } from '@/kit/shell.js';
import { addressField } from './address.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
connect({
  suggestions: (m) => addressField.suggestions(m.id, m.items || []),
  focusAddressBar: () => addressField.focus(),
});
