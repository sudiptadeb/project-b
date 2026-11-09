<script setup>
import { computed } from 'vue';
import AddressBar from './AddressBar.vue';
import NavButtons from './NavButtons.vue';
import { send } from './bridge.js';
import { activeTab, controlSpace as controls, state } from './state.js';

// With the tabs on the side this row is the title bar: it holds the space for the native
// window controls, and its empty parts drag the window. With the tabs on top the strip
// above is the title bar and this row is an ordinary toolbar.
const titleBar = computed(() => state.tabLayout !== 'horizontal');

function switchLayout() {
  send({ t: 'setTabLayout', text: titleBar.value ? 'horizontal' : 'vertical' });
}
</script>

<template>
  <header v-shell class="top" :class="{ 'title-bar': titleBar }">
    <template v-if="titleBar">
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
    </template>
    <NavButtons />
    <AddressBar />
    <button
      class="icon-button layout"
      type="button"
      :aria-label="titleBar ? 'Tabs on top' : 'Tabs on the side'"
      :title="titleBar ? 'Tabs on top' : 'Tabs on the side'"
      @click="switchLayout"
    >
      <!-- Where the tabs will go after the switch: a band on top, or down the side. -->
      <svg v-if="titleBar" width="14" height="13" viewBox="0 0 14 13" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12.18.75H1.75a1 1 0 0 0-1 1V11a1 1 0 0 0 1 1h10.43a1 1 0 0 0 1-1V1.75a1 1 0 0 0-1-1Z"/><path d="M.75 4.5h12.43"/><path d="M4.5.75v3.75M8.5.75v3.75" /></svg>
      <svg v-else width="14" height="13" viewBox="0 0 14 13" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12.18.75H1.75a1 1 0 0 0-1 1V11a1 1 0 0 0 1 1h10.43a1 1 0 0 0 1-1V1.75a1 1 0 0 0-1-1Z"/><path d="M4.56.75V12"/><path d="M1.5 4h2.5M1.5 6.5h2.5M1.5 9h2.5"/></svg>
    </button>
    <div v-if="titleBar" v-drag class="space" :style="{ width: controls.right }"></div>
    <div v-else class="space" style="width: 4px"></div>
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
  border-radius: 8px 8px 0 0;
  background: var(--bar);
}

.title-bar {
  height: 28px;
}

.space {
  flex: none;
  height: 100%;
}

.layout {
  margin-left: 8px;
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
