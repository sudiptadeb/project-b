import { createApp } from 'vue';
import App from './App.vue';
import { vDrag, vShell } from './regions.js';
import { connect } from './state.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).mount('#app');
connect();
