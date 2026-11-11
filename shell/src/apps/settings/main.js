import { createApp } from 'vue';
import '@/design.css';
import SettingsApp from './SettingsApp.vue';
import { phone } from '@/phone.js';
import { connect, end, firstPaint, inputs, mark } from './state.js';

mark('settings.script');
inputs();
// Opening settings is over when its first frame is on the screen.
firstPaint((onScreen) => end('settings.open', onScreen));

document.documentElement.classList.toggle('touch', phone);

createApp(SettingsApp).mount('#app');
mark('settings.mounted');
connect();
