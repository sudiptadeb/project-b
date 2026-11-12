import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { vDrag, vShell } from './regions.js';
import { connect, phone } from './state.js';
import { firstPaint, inputs, mark } from '@/bridge.js';

mark('shell.script');
firstPaint();
inputs();

document.documentElement.classList.toggle('touch', phone);

createApp(App).directive('shell', vShell).directive('drag', vDrag).mount('#app');
mark('shell.mounted');
connect();
