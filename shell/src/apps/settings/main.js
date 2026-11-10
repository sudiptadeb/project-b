import { createApp } from 'vue';
import SettingsApp from './SettingsApp.vue';
import { connect } from './state.js';

// The CSS pointer queries call a phone with a stylus a fine pointer, so they can't tell us.
document.documentElement.classList.toggle('touch', navigator.maxTouchPoints > 0);

createApp(SettingsApp).mount('#app');
connect();
