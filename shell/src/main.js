import { createApp } from 'vue';
import App from './App.vue';
import { vShell } from './regions.js';
import { connect } from './state.js';

createApp(App).directive('shell', vShell).mount('#app');
connect();
