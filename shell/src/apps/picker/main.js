import { createApp } from 'vue';
import '@/design.css';
import PickerApp from './PickerApp.vue';
import { connect, inputs, mark } from './state.js';

mark('picker.script');
inputs();
createApp(PickerApp).mount('#app');
mark('picker.mounted');
connect();
