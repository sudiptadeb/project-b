<script setup>
import { computed } from 'vue';
import AddressBar from './AddressBar.vue';
import NavButtons from './NavButtons.vue';
import { activeTab, state } from './state.js';

// The space for the native window controls, on whichever side they are. Empty when
// there are none. The rest of the row is 32px tall, the height of the title bar, so its
// empty parts drag the window.
const controls = computed(() => {
  const c = state.controls;
  if (c.w <= 0) return { left: '8px', right: '8px' };
  const onLeft = c.x + c.w / 2 < window.innerWidth / 2;
  return onLeft ? { left: c.x + c.w + 8 + 'px', right: '8px' } : { left: '8px', right: window.innerWidth - c.x + 8 + 'px' };
});
</script>

<template>
  <header v-shell class="top">
    <div v-drag class="space" :style="{ width: controls.left }"></div>
    <button
      class="icon-button"
      type="button"
      :aria-label="state.sidebar ? 'Hide sidebar' : 'Show sidebar'"
      :aria-expanded="state.sidebar"
      @click="state.sidebar = !state.sidebar"
    >
      <svg width="14" height="13" viewBox="0 0 14 13" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12.18.75H1.75a1 1 0 0 0-1 1V11a1 1 0 0 0 1 1h10.43a1 1 0 0 0 1-1V1.75a1 1 0 0 0-1-1Z"/><path d="M4.56.75V12"/><path v-if="state.sidebar" d="M4.28.75H.75v11h3.53z" fill="currentColor" stroke="none" opacity=".25"/></svg>
    </button>
    <NavButtons />
    <AddressBar />
    <div v-drag class="space" :style="{ width: controls.right }"></div>
    <div class="progress" :class="{ on: activeTab.loading }"></div>
  </header>
</template>

<style scoped>
.top {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  flex: none;
  height: 32px;
  background: var(--bar);
}

.space {
  flex: none;
  height: 100%;
}

.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  width: 0;
  background: var(--accent);
}

.progress.on {
  width: 100%;
  animation: load 1s ease-in-out infinite;
  transform-origin: left;
}

@keyframes load {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}
</style>
