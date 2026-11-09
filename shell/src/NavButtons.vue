<script setup>
import { send } from './bridge.js';
import { state } from './state.js';

function act(t) {
  send({ t });
  send({ t: 'focusPage' });
}
</script>

<template>
  <div class="nav">
    <button class="back" type="button" aria-label="Back" :disabled="!state.tab.canGoBack" @click="act('back')">
      <svg width="12" height="12" viewBox="0 0 13.5 13.5" fill="currentColor"><path d="M.51 5.71L5.79.21C5.86.14 5.94.09 6.02.05 6.11.01 6.2 0 6.29 0 6.38 0 6.48.01 6.56.05 6.65.09 6.72.14 6.79.21 6.92.35 6.99.53 6.99.73 6.99.92 6.92 1.1 6.79 1.24L1.49 6.75 6.78 12.25C6.91 12.39 6.99 12.58 6.99 12.77 6.99 12.96 6.91 13.15 6.78 13.29 6.72 13.35 6.64 13.41 6.56 13.44 6.47 13.48 6.38 13.5 6.29 13.5 6.19 13.5 6.1 13.48 6.02 13.44 5.93 13.41 5.85 13.35 5.79 13.29L.51 7.78C.35 7.66.22 7.5.13 7.32.04 7.14 0 6.95 0 6.75 0 6.55.04 6.35.13 6.17.22 5.99.35 5.84.51 5.71Z"/><path d="M.6 6.73C.6 6.54.66 6.35.79 6.22.92 6.08 1.11 6 1.3 6H12.72C12.91 6 13.09 6.08 13.22 6.22 13.35 6.35 13.43 6.54 13.43 6.73 13.43 6.91 13.35 7.1 13.22 7.23 13.09 7.37 12.91 7.45 12.72 7.46H1.3C1.11 7.45.93 7.37.8 7.24.66 7.1.6 6.91.6 6.73Z"/></svg>
    </button>
    <button class="forward" type="button" aria-label="Forward" :disabled="!state.tab.canGoForward" @click="act('forward')">
      <svg width="12" height="12" viewBox="0 0 13.6 13.5" fill="currentColor"><path d="M12.83 6.73C12.83 6.54 12.76 6.35 12.63 6.22 12.5 6.08 12.31 6 12.13 6H.7C.51 6 .33 6.08.2 6.22.07 6.35 0 6.54 0 6.73 0 6.91.07 7.1.2 7.23.33 7.37.51 7.45.7 7.46H12.12C12.31 7.45 12.49 7.37 12.62 7.24 12.76 7.1 12.83 6.91 12.83 6.73Z"/><path transform="translate(6.6 0)" d="M6.48 5.71L1.2.21C1.13.14 1.05.09.97.05.88.01.79 0 .7 0 .61 0 .51.01.43.05.34.09.27.14.2.21.07.35 0 .53 0 .73 0 .92.07 1.1.2 1.24L5.49 6.75.2 12.25C.07 12.39 0 12.58 0 12.77 0 12.96.07 13.15.2 13.29.26 13.35.34 13.41.43 13.44.51 13.48.6 13.5.7 13.5.79 13.5.88 13.48.97 13.44 1.05 13.41 1.13 13.35 1.19 13.29L6.48 7.78C6.64 7.66 6.77 7.5 6.86 7.32 6.95 7.14 6.99 6.95 6.99 6.75 6.99 6.55 6.95 6.35 6.86 6.17 6.77 5.99 6.64 5.84 6.48 5.71Z"/></svg>
    </button>
    <button class="reload" type="button" aria-label="Reload" :disabled="!state.tab.url" @click="act('reload')">
      <svg width="12.5" height="12.5" viewBox="0 0 14 14" fill="none"><path d="M12.01 2.85C12.01 2.98 11.99 3.11 11.93 3.23 11.87 3.35 11.79 3.45 11.69 3.53 11.59 3.61 11.47 3.67 11.35 3.7 11.22 3.73 11.09 3.73 10.96 3.71L9.8 3.47C9.67 3.45 9.54 3.5 9.43 3.54 9.33 3.62 9.25 3.73 9.21 3.86 9.2 3.99 9.22 4.12 9.28 4.23 9.37 4.34 9.47 4.41 9.6 4.45L12.64 5.07C12.8 5.08 12.95 5.03 13.08 4.93 13.18 4.81 13.24 4.66 13.23 4.5L12.85 1.44C12.84 1.32 12.78 1.2 12.69 1.12 12.6 1.04 12.48 1 12.36 1 12.21 1.02 12.09 1.09 11.98 1.19 11.9 1.31 11.86 1.45 11.87 1.59L12.01 2.85Z" fill="currentColor" stroke="currentColor" stroke-width=".25"/><path d="M10.97 10.64C10.09 11.6 8.92 12.24 7.63 12.48 6.35 12.72 5.03 12.53 3.86 11.95 2.69 11.37 1.74 10.43 1.15 9.27.56 8.1.36 6.78.59 5.5.81 4.21 1.44 3.03 2.39 2.14 3.34 1.24 4.56.68 5.85.53 7.15.39 8.46.66 9.59 1.32 10.71 1.98 11.6 2.98 12.1 4.19" stroke="currentColor" stroke-linecap="round"/></svg>
    </button>
  </div>
</template>

<style scoped>
.nav {
  position: relative;
  flex: none;
  width: 96px;
  height: 28px;
}

button {
  position: absolute;
  top: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--accent);
  cursor: pointer;
}

button:hover:not(:disabled) {
  background: rgba(0, 0, 0, 0.06);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}

button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}

.back {
  left: 8px;
}

.forward {
  left: 36px;
}

.reload {
  left: 64px;
}
</style>
