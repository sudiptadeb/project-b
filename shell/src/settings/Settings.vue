<script setup>
import { computed, reactive } from 'vue';
import { listen, send } from '../bridge.js';

// The page's copy of what Go owns: the prefs and the updater's state. It changes only when
// Go says so, and every settings page open hears the same.
const prefs = reactive({ tabLayout: '', autoUpdate: true });
const update = reactive({ running: '', state: { phase: '' } });

listen(
  (m) => {
    if (m.t === 'prefs') Object.assign(prefs, m.prefs);
    if (m.t === 'update') {
      update.state = m.update;
      if ('running' in m) update.running = m.running;
    }
  },
  () => {
    send({ t: 'prefs' });
    send({ t: 'updateState' });
  },
);

const layouts = [
  { value: 'vertical', label: 'Vertical' },
  { value: 'horizontal', label: 'Horizontal' },
];

// What the update manager is doing, or what the check the user asked for found.
const status = computed(() => {
  const s = update.state;
  if (s.check === 'checking') return 'Checking…';
  if (s.phase === 'downloading') return `Downloading ${s.version}…`;
  if (s.phase === 'ready') return `${s.version} is ready to relaunch`;
  if (s.phase === 'failed') return `Update${s.version ? ' to ' + s.version : ''} failed: ${s.error}`;
  if (s.check === 'current') return 'Up to date';
  if (s.check === 'failed') return s.checkError;
  return '';
});

const busy = computed(() => update.state.check === 'checking' || update.state.phase === 'downloading');
</script>

<template>
  <div class="page">
    <header class="header">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
      <h1>Settings</h1>
    </header>

    <main class="main">
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
                :checked="prefs.tabLayout === l.value"
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
            <span class="label">Version <span class="version">{{ update.running || 'Development build' }}</span></span>
            <span class="status" role="status">{{ status }}</span>
            <button type="button" :disabled="busy" @click="send({ t: 'checkUpdate' })">Check for updates</button>
          </div>
          <label class="row">
            <span class="label">Automatic updates</span>
            <input
              type="checkbox"
              role="switch"
              class="switch"
              :checked="prefs.autoUpdate"
              :aria-checked="String(prefs.autoUpdate)"
              @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
            />
          </label>
        </div>
      </section>
    </main>
  </div>
</template>

<style>
/* Type and layout from the Ulaa preferences design; colours are the system's, so the page
   follows light and dark like any other. */
:root {
  font: 14px/1.4 'Zoho Puvi', 'Zoho_Puvi', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  --rule: color-mix(in srgb, CanvasText 10%, transparent);
  --border: color-mix(in srgb, CanvasText 14%, transparent);
  --muted: color-mix(in srgb, CanvasText 62%, transparent);
}
</style>

<style scoped>
.header {
  display: flex;
  align-items: center;
  gap: 11px;
  height: 64px;
  padding: 0 26px;
  border-bottom: 1px solid var(--rule);
}

h1 {
  margin: 0;
  font-size: 21px;
  font-weight: 600;
  line-height: 1;
}

.main {
  max-width: 900px;
  margin: 0 auto;
  padding: 27px 32px 48px;
}

section + section {
  margin-top: 40px;
}

h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 500;
  line-height: 22px;
}

.card {
  margin-top: 15px;
  border: 1px solid var(--border);
  border-radius: 8px;
}

.row {
  display: flex;
  align-items: center;
  gap: 24px;
  min-height: 56px;
  padding: 8px 20px;
  box-sizing: border-box;
}

.row + .row {
  border-top: 1px solid var(--rule);
}

.label {
  flex: 1;
  min-width: 0;
  font-size: 15px;
  font-weight: 500;
}

.version {
  margin-left: 6px;
  color: var(--muted);
  font-weight: 400;
}

.choice {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.choice input,
input[type='checkbox'] {
  margin: 0;
}

/* A switch drawn on the checkbox, which keeps its keys and its meaning; Chromium 155 has the
   native one only behind a flag. Off is an outline, on is the system accent. */
.switch {
  appearance: none;
  position: relative;
  flex: none;
  width: 36px;
  height: 20px;
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
  border-color: AccentColor;
  background: AccentColor;
}

.switch:checked::before {
  background: AccentColorText;
  transform: translateX(16px);
}

.switch:focus-visible {
  outline: 2px solid AccentColor;
  outline-offset: 2px;
}

.status {
  color: var(--muted);
}

button {
  height: 32px;
  padding: 0 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 500;
  cursor: pointer;
}

button:hover:not(:disabled) {
  background: color-mix(in srgb, CanvasText 6%, transparent);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
