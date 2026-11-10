import { createApp } from 'vue';
import SettingsApp from './SettingsApp.vue';
import { connect } from './state.js';

createApp(SettingsApp).mount('#app');
connect();
