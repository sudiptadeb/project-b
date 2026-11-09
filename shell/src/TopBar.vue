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
  <header v-shell class="top" :class="{ toolbar: !titleBar }">
    <div class="bar">
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
    <div v-else class="space" style="width: 4px"></div>
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
      <svg v-if="titleBar" width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5"><rect x=".65" y=".65" width="12.7" height="10.7" rx="1.5"/><path d="M.65 3.8h12.7"/></svg>
      <svg v-else width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5"><rect x=".65" y=".65" width="12.7" height="10.7" rx="1.5"/><path d="M4.6.65v10.7"/></svg>
    </button>
    <div v-if="titleBar" v-drag class="space" :style="{ width: controls.right }"></div>
    <div v-else class="space" style="width: 8px"></div>
    <!-- The page's real load progress. Shown while loading; the width stays so a test can read it. -->
    <div class="progress" :class="{ on: activeTab.loading }" :style="{ width: activeTab.progress * 100 + '%' }"></div>
    </div>
  </header>
</template>

<style scoped>
/* The row starts as frame colour, like every tracked component, and the bar is drawn
   on it 4px in from the sides. As the title bar it also carries the frame's top band,
   and the bar's top corners follow the window's curve, one frame width inside it. */
.top {
  position: relative;
  z-index: 1;
  flex: none;
  padding: var(--frame-width) var(--frame-width) 0;
  background: var(--frame);
}

.bar {
  position: relative;
  display: flex;
  align-items: center;
  height: 44px;
  border-radius: var(--inner-radius) var(--inner-radius) 0 0;
  background: var(--bar);
}

/* Under the tab strip the row has no band of its own, and the bar's corners meet the
   strip with the tabs' radius. */
.top.toolbar {
  padding-top: 0;
}

.top.toolbar .bar {
  border-radius: 8px 8px 0 0;
}

.space {
  flex: none;
  height: 100%;
}

.layout {
  margin-left: 12px;
}

.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  opacity: 0;
  transition: width 0.15s ease-out;
}

.progress.on {
  opacity: 1;
}
</style>
