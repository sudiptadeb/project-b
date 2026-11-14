<script setup>
import { computed } from 'vue';
import AddressBar from './AddressBar.vue';
import NavButtons from './NavButtons.vue';
import { ExtensionsToolbar } from '@extensions';
import { activeTab, controlSpace as controls, send, state } from '@/sdk/vue.js';
import { tabsOnTop, view } from './state.js';

// With the tabs on the side this row is the title bar: it holds the space for the native
// window controls. With the tabs on top the strip above is the title bar and this row is
// a toolbar. Either way the row moves the window wherever it has no control.
const titleBar = computed(() => !tabsOnTop.value);

function switchLayout() {
  send({ t: 'setTabLayout', text: titleBar.value ? 'horizontal' : 'vertical' });
}
</script>

<template>
  <!-- The bar and the frame's bands around it drag the window in their gaps. It stays the
       shell's too, so in full screen, where nothing drags, a press there reaches no page. -->
  <header v-shell v-drag class="top" :class="{ toolbar: !titleBar }">
    <div v-if="titleBar" class="band top"></div>
    <div class="row">
    <div class="band side"></div>
    <div class="bar">
    <template v-if="titleBar">
      <div class="space" :style="{ width: controls.left }"></div>
      <button
        class="icon-button"
        type="button"
        :aria-label="view.sidebar ? 'Hide sidebar' : 'Show sidebar'"
        :aria-expanded="view.sidebar"
        @click="view.sidebar = !view.sidebar"
      >
        <svg width="14" height="13" viewBox="0 0 14 13" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12.18.75H1.75a1 1 0 0 0-1 1V11a1 1 0 0 0 1 1h10.43a1 1 0 0 0 1-1V1.75a1 1 0 0 0-1-1Z"/><path d="M4.56.75V12"/><path v-if="view.sidebar" d="M4.28.75H.75v11h3.53z" fill="currentColor" stroke="none" opacity=".25"/></svg>
      </button>
    </template>
    <div v-else class="space" style="width: 4px"></div>
    <NavButtons />
    <AddressBar />
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
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
    <!-- The Ulaa preferences design's gear, drawn at the toolbar's 1.5px stroke. -->
    <button class="icon-button settings" type="button" aria-label="Settings" title="Settings" @click="send({ t: 'run', text: 'app.settings' })">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
    </button>
    <div class="space" :style="{ width: titleBar ? controls.right : '8px' }"></div>
    <!-- The page's real load progress. Shown while loading; the width stays so a test can read it. -->
    <div class="progress" :class="{ on: activeTab.loading }" :style="{ width: activeTab.progress * 100 + '%' }"></div>
    </div>
    <div class="band side"></div>
    </div>
  </header>
</template>

<style scoped>
/* The row starts as frame colour, like every tracked component, and the bar is drawn
   on it between the frame's side bands. As the title bar it also carries the frame's
   top band; the curve over its corners is the window's own border, drawn by the
   engine. */
.top {
  position: relative;
  z-index: 1;
  flex: none;
  background: var(--frame);
}

.band.top {
  height: var(--frame-width);
}

.row {
  display: flex;
}

.band.side {
  flex: none;
  width: var(--frame-width);
}

.bar {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  height: 44px;
  background: var(--bar);
}

/* Under the tab strip the bar's corners meet the strip with the tabs' radius. */

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

/* The same 4px apart as the back, forward and reload buttons. */
.settings {
  margin-left: 4px;
}

.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  opacity: 0;
  transition: width var(--motion) var(--ease);
}

.progress.on {
  opacity: 1;
}

</style>
