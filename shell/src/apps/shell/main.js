import { createApp } from 'vue';
import App from './App.vue';
import { vDrag, vShell } from './regions.js';
import { connect, phone } from './state.js';

document.documentElement.classList.toggle('touch', phone);

createApp(App).directive('shell', vShell).directive('drag', vDrag).mount('#app');
connect();
