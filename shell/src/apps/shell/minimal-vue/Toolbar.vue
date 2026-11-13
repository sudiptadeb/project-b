<script setup>
import { ref } from 'vue';
import { activeTab, controlSpace, focusPage, send, state } from '@/sdk/vue.js';
import Address from './Address.vue';
import { Extensions } from './extensions.js';

// Back, Forward, Reload or Stop, the address filling the rest, the extensions, an update
// ready to relaunch into, and Settings. With the tabs down the side it is the top row, so
// it starts with the window buttons' space, which drags the window.
defineProps({ vertical: Boolean });
const address = ref(null);
defineExpose({ address });

function act(msg) {
  send(msg);
  focusPage();
}
</script>

<template>
  <div v-shell class="toolbar" :class="{ vertical }">
    <div v-if="vertical" v-drag class="lights" :style="{ width: controlSpace.left }"></div>
    <button class="btn" aria-label="Back" :disabled="!activeTab.canGoBack" @click="act({ t: 'back' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M10 3 5 8l5 5" /></svg>
    </button>
    <button class="btn" aria-label="Forward" :disabled="!activeTab.canGoForward" @click="act({ t: 'forward' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m6 3 5 5-5 5" /></svg>
    </button>
    <button v-if="activeTab.loading" class="btn" aria-label="Stop" @click="act({ t: 'run', text: 'nav.stop' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m3.5 3.5 9 9m0-9-9 9" /></svg>
    </button>
    <button v-else class="btn" aria-label="Reload" :disabled="!activeTab.url" @click="act({ t: 'reload' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M13 8a5 5 0 1 1-1.5-3.5M13 2.5V5h-2.5" /></svg>
    </button>
    <Address ref="address" />
    <component :is="Extensions" v-if="Extensions" :extensions="state.extensions" />
    <button v-if="state.update.phase === 'ready'" class="relaunch" @click="send({ t: 'relaunch' })">Relaunch to update</button>
    <button class="btn" aria-label="Settings" @click="send({ t: 'run', text: 'app.settings' })">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
    </button>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 38px;
  padding: 0 6px;
  background: var(--bar);
  border-bottom: 1px solid var(--line);
}

.toolbar.vertical {
  padding-left: 0;
}

.lights {
  flex: none;
  align-self: stretch;
}

.relaunch {
  flex: none;
  height: 28px;
  margin: 0 4px;
  padding: 0 12px;
  border: 1px solid var(--line);
  background: none;
}

.relaunch:hover {
  background: var(--hover);
}
</style>
