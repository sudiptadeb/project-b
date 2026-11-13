import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { vDrag, vPage, vShell } from '@/kit/regions.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
