<script setup>
// The window's own buttons where the frame is ours, Linux and Windows (docs/design-system.md,
// "The window's own buttons and menus"): minimise, maximise and close at the right end of the
// title bar's row. The Mac keeps its own, which leave their space in the row instead
// (controlSpace), so there this draws nothing. Each runs Go's action, as a menu would.
import { computed } from 'vue';
import { send, state } from '@/sdk/vue.js';

const ours = computed(() => state.platform !== 'mac' && !(state.controls && state.controls.w > 0));
const run = (action) => send({ t: 'run', text: action });
</script>

<template>
  <div v-if="ours" class="window-buttons">
    <button class="icon-button" type="button" aria-label="Minimise" title="Minimise" @click="run('window.minimize')">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3.5 8h9"/></svg>
    </button>
    <button class="icon-button" type="button" aria-label="Maximise" title="Maximise" @click="run('window.zoom')">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1"/></svg>
    </button>
    <button class="icon-button close" type="button" aria-label="Close" title="Close" @click="run('window.close')">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 4l8 8M12 4l-8 8"/></svg>
    </button>
  </div>
</template>

<style scoped>
.window-buttons {
  display: flex;
  flex: none;
  align-items: center;
  margin-left: 8px;
}

.window-buttons .icon-button {
  width: 32px;
  height: 28px;
  color: var(--icon);
}

.window-buttons .close:hover {
  background: var(--danger);
  color: #fff;
}
</style>
