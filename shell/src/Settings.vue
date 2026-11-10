<script setup>
import { computed, onMounted, ref } from 'vue';
import { send } from './bridge.js';
import { state } from './state.js';

// The settings panel: the shell draws it over the page, under the toolbar's gear. It shows
// what Go owns and sends the user's changes back; it changes only when Go says so.
const panel = ref(null);
onMounted(() => panel.value.focus());

function close() {
  state.settings = false;
}

const layouts = [
  { value: 'vertical', label: 'Vertical' },
  { value: 'horizontal', label: 'Horizontal' },
];

// What the update manager is doing, or what the check the user asked for found.
const status = computed(() => {
  const s = state.update;
  if (s.check === 'checking') return 'Checking…';
  if (s.phase === 'downloading') return `Downloading ${s.version}…`;
  if (s.phase === 'ready') return `${s.version} is ready to relaunch`;
  if (s.phase === 'failed') return `Update${s.version ? ' to ' + s.version : ''} failed: ${s.error}`;
  if (s.check === 'current') return 'Up to date';
  if (s.check === 'failed') return s.checkError;
  return '';
});

const busy = computed(() => state.update.check === 'checking' || state.update.phase === 'downloading');
</script>

<template>
  <div ref="panel" v-shell class="settings-panel" role="dialog" aria-labelledby="settings-title" tabindex="-1" @keydown.esc="close">
    <header class="header">
      <h1 id="settings-title">Settings</h1>
      <button class="icon-button" type="button" aria-label="Close settings" @click="close">
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l8 8M9 1L1 9"/></svg>
      </button>
    </header>

    <section aria-labelledby="appearance">
      <h2 id="appearance">Appearance</h2>
      <div class="card">
        <div class="row" role="radiogroup" aria-labelledby="tab-layout">
          <span id="tab-layout" class="label">Tab layout</span>
          <label v-for="l in layouts" :key="l.value" class="choice">
            <input
              type="radio"
              name="tabLayout"
              :value="l.value"
              :checked="state.tabLayout === l.value"
              @change="send({ t: 'setTabLayout', text: l.value })"
            />
            {{ l.label }}
          </label>
        </div>
      </div>
    </section>

    <section aria-labelledby="updates">
      <h2 id="updates">Updates</h2>
      <div class="card">
        <div class="row">
          <span class="label">
            Version <span class="version">{{ state.running || 'Development build' }}</span>
            <span class="status" role="status">{{ status }}</span>
          </span>
          <button type="button" class="text-button" :disabled="busy" @click="send({ t: 'checkUpdate' })">Check for updates</button>
        </div>
        <label class="row">
          <span class="label">Automatic updates</span>
          <input
            type="checkbox"
            role="switch"
            class="switch"
            :checked="state.autoUpdate"
            :aria-checked="String(state.autoUpdate)"
            @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
          />
        </label>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* A panel over the page's top-right corner, under the gear, in the shell's own colours, so
   it follows the frame colour like the rest of the shell. Type from the Ulaa preferences
   design, at a panel's size. */
.settings-panel {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 2;
  width: 380px;
  max-height: calc(100% - 16px);
  overflow: auto;
  padding: 0 16px 16px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface);
  box-shadow: 0 8px 28px rgba(16, 24, 40, 0.14), 0 1px 3px rgba(16, 24, 40, 0.08);
  outline: none;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 52px;
}

h1 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

section + section {
  margin-top: 18px;
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

.version {
  margin-left: 4px;
  color: var(--muted);
  font-weight: 400;
}

.status {
  display: block;
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.status:empty {
  display: none;
}

.choice {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.choice input {
  margin: 0;
  accent-color: var(--accent);
}

/* A switch drawn on the checkbox, which keeps its keys and its meaning; Chromium 155 has the
   native one only behind a flag. Off is an outline, on is the accent. */
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

.switch:focus-visible,
.text-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
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
</style>
