import { createApp } from 'vue';
import App from './App.vue';
import { vDrag, vShell } from './regions.js';
import { connect } from './state.js';

// The CSS pointer queries call a phone with a stylus a fine pointer, so they can't tell us.
document.documentElement.classList.toggle('touch', navigator.maxTouchPoints > 0);

createApp(App).directive('shell', vShell).directive('drag', vDrag).mount('#app');
connect();
