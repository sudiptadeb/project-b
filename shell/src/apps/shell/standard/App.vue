<script setup>
import { watchEffect } from 'vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabIcons from './TabIcons.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { state } from '@/sdk/vue.js';
import { phone, tabsOnTop, view } from './state.js';

// Every layout shows the same tabs from the same state; only where they sit differs. On a
// phone they are favicons under the toolbar, and the sidebar is a drawer over everything.

// The frame's colour and width and the window's corner radius come from the managers
// and become CSS variables on the root: the shell paints its bands in the colour and
// width the window paints natively, so there is one source for both, and every other
// tone of the shell derives from that colour. The curve over the corners is the window's
// own border, drawn by the engine. Light or dark follows Go's theme pref.
watchEffect(() => {
  document.documentElement.style.setProperty('--frame-width', state.frameWidth + 'px');
  document.documentElement.style.setProperty('--window-radius', state.cornerRadius + 'px');
});
</script>

<!-- Every component the shell tracks starts as frame colour and draws its own surface on
     top; the frame is what shows where nothing was drawn. The page is a plain rectangle
     between them, with the left, right and bottom edges as plain strips of their own. -->
<template>
  <div class="app">
    <TabStrip v-if="tabsOnTop" />
    <TopBar />
    <TabIcons v-if="phone" />
    <div class="body">
      <div class="edge left"></div>
      <Sidebar v-if="!tabsOnTop && !phone" />
      <PageView />
      <div class="edge right"></div>
    </div>
    <div class="edge bottom"></div>
    <!-- A tap outside the drawer closes it, and goes no further. -->
    <div v-if="phone && view.sidebar" v-shell class="drawer" @click.self="view.sidebar = false">
      <Sidebar drawer />
    </div>
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

.drawer {
  position: absolute;
  inset: 0;
  z-index: 3;
}
</style>
