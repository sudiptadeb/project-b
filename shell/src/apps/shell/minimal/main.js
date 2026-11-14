import { mount } from 'svelte';
import '@/design.css';
import './minimal.css';
import App from './App.svelte';

mount(App, { target: document.getElementById('app') });
