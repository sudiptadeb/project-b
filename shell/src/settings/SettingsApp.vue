<script setup>
import { computed, onMounted, onUnmounted, ref, watchEffect } from 'vue';
import { tones } from '../tones.js';
import General from './General.vue';
import Keyboard from './Keyboard.vue';
import { send, state } from './state.js';

// The settings page: a page of its own over the whole window. Its colours come from the
// frame colour, as the shell's do.
watchEffect(() => {
  if (state.windowColor) {
    for (const [name, value] of Object.entries(tones(state.windowColor))) {
      document.documentElement.style.setProperty(name, value);
    }
  }
});

const sections = [
  { id: 'general', label: 'General' },
  { id: 'keyboard', label: 'Keyboard' },
];
const section = ref('general');

// The header leaves the native window buttons their space, as the shell's top row does.
const headerStart = computed(() => {
  const c = state.controls;
  return c.w > 0 ? c.x + c.w + 16 + 'px' : '24px';
});

function close() {
  send({ t: 'close' });
}

// Escape closes, unless a key is being recorded, which takes Escape for itself.
function onKey(e) {
  if (e.key === 'Escape' && !e.defaultPrevented) close();
}
onMounted(() => window.addEventListener('keydown', onKey));
onUnmounted(() => window.removeEventListener('keydown', onKey));
</script>

<template>
  <div v-if="state.ready" class="page">
    <header class="header" :style="{ paddingLeft: headerStart }">
      <h1>Settings</h1>
      <button class="icon-button" type="button" aria-label="Close settings" title="Close settings" @click="close">
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l8 8M9 1L1 9"/></svg>
      </button>
    </header>
    <div class="body">
      <nav class="nav" aria-label="Settings sections">
        <button
          v-for="s in sections"
          :key="s.id"
          type="button"
          class="nav-item"
          :aria-current="section === s.id ? 'page' : undefined"
          @click="section = s.id"
        >{{ s.label }}</button>
      </nav>
      <main class="content">
        <General v-if="section === 'general'" />
        <Keyboard v-else />
      </main>
    </div>
  </div>
</template>

<style>
/* Type and surfaces from the Ulaa preferences design, at a page's size. The frame colour's
   tones (--frame, --accent, --bar, --icon, --accent-soft, --accent-tint) come from tones.js. */
:root {
  --surface: #ffffff;
  --text: #1f1f1f;
  --muted: #5c6270;
  --line: rgba(15, 20, 25, 0.1);
  --danger: #ea3323;
  font: 14px/1.4 'Zoho Puvi', 'Zoho_Puvi', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
  color: var(--text);
  -webkit-font-smoothing: antialiased;
}

body {
  background: var(--bar);
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 52px;
  padding-right: 16px;
  box-sizing: border-box;
}

h1 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.nav {
  flex: none;
  width: 200px;
  padding: 8px 12px;
  box-sizing: border-box;
}

.nav-item {
  display: block;
  width: 100%;
  height: 34px;
  padding: 0 12px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: inherit;
  font: inherit;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.nav-item:hover {
  background: var(--accent-tint);
}

.nav-item[aria-current='page'] {
  background: var(--accent-soft);
}

.content {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 8px 32px 32px 8px;
}

.content > * {
  max-width: 720px;
}

section + section {
  margin-top: 24px;
}

h2 {
  margin: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--muted);
}

.card {
  margin-top: 8px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.row {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 48px;
  padding: 8px 14px;
  box-sizing: border-box;
}

.row + .row {
  border-top: 1px solid var(--line);
}

.label {
  flex: 1;
  min-width: 0;
  font-weight: 500;
}

.icon-button {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.icon-button:hover:not(:disabled) {
  background: var(--accent-tint);
}

.text-button {
  flex: none;
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: inherit;
  font: inherit;
  font-weight: 500;
  cursor: pointer;
}

.text-button:hover:not(:disabled) {
  background: var(--accent-tint);
}

.text-button:disabled {
  opacity: 0.5;
  cursor: default;
}

.icon-button:focus-visible,
.text-button:focus-visible,
.nav-item:focus-visible,
select:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}

select {
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: inherit;
  font: inherit;
}

/* A switch drawn on the checkbox, which keeps its keys and its meaning. */
.switch {
  appearance: none;
  position: relative;
  flex: none;
  width: 36px;
  height: 20px;
  margin: 0;
  border: 1px solid var(--muted);
  border-radius: 10px;
  box-sizing: border-box;
  background: transparent;
  cursor: pointer;
}

.switch::before {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--muted);
  transition: transform 120ms;
}

.switch:checked {
  border-color: var(--accent);
  background: var(--accent);
}

.switch:checked::before {
  background: #fff;
  transform: translateX(16px);
}

.switch:disabled {
  opacity: 0.5;
  cursor: default;
}

.switch:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}
</style>
