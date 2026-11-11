import { createApp } from 'vue';
import '@/design.css';
import SettingsApp from './SettingsApp.vue';
import { phone } from '@/phone.js';
import { connect } from './state.js';

document.documentElement.classList.toggle('touch', phone);

createApp(SettingsApp).mount('#app');
connect();
