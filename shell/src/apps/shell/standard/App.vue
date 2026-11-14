<script setup>
import { computed, watchEffect } from 'vue';
import FloatMenu from './FloatMenu.vue';
import PageMenu from './PageMenu.vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { setFrame, state } from '@/sdk/vue.js';
import { tabsOnTop } from './state.js';

// Every layout shows the same tabs from the same state; only where they sit differs.

// The window draws a 4px band of the frame colour around its edge, and the shell paints
// its own strips in that colour and width, as CSS variables on the root, so there is one
// source for both; every other tone of the shell derives from that colour. The window's
// corner radius comes from the managers. The curve over the corners is the window's own
// border, drawn by the engine. Light or dark follows Go's theme pref.
// A page in full screen has the whole window: no band, no bars, nothing over it.
const pageFull = computed(() => state.pageFullScreen !== 0 && state.pageFullScreen === state.active);
watchEffect(() => setFrame(pageFull.value ? 0 : 4));
watchEffect(() => {
  document.documentElement.style.setProperty('--window-radius', state.cornerRadius + 'px');
});
</script>

<!-- Every component the shell tracks starts as frame colour and draws its own surface on
     top; the frame is what shows where nothing was drawn. The page is a plain rectangle
     between them, with the left, right and bottom edges as plain strips of their own. -->
<template>
  <div class="app" :class="{ 'page-full': pageFull }">
    <!-- Hidden, not taken away, while a page fills the screen: each keeps what it holds,
         as the address it shows. -->
    <TabStrip v-if="tabsOnTop" v-show="!pageFull" />
    <TopBar v-show="!pageFull" />
    <div class="body">
      <div v-show="!pageFull" class="edge left"></div>
      <Sidebar v-if="!tabsOnTop" v-show="!pageFull" />
      <PageView />
      <div v-show="!pageFull" class="edge right"></div>
    </div>
    <div v-show="!pageFull" class="edge bottom"></div>
    <FloatMenu />
    <PageMenu />
  </div>
</template>

<style>
/* Colours from the Ulaa Browser design. The frame's colour and width come from Go with the
   snapshot, the same values the window paints natively, and the frame colour's tones
   (--frame, --accent, --bar, --icon, --accent-soft, --accent-tint) are set from it by
   tones.js. Icons and the sidebar's New Tab take a deeper ink, so the frame is the one
   thing in the full accent. The root has no background: the page shows through wherever
   nothing is painted. */
.app {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.edge {
  flex: none;
  background: var(--frame);
}

.edge.left,
.edge.right {
  width: var(--frame-width);
}

.edge.bottom {
  height: var(--frame-width);
}

</style>
