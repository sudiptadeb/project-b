<script setup>
import { computed, watchEffect } from 'vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { state } from './state.js';
import { useTheme } from '@/theme.js';

// Both layouts show the same tabs from the same state; only where they sit differs.
const horizontal = computed(() => state.tabLayout === 'horizontal');

// The frame's colour and width and the window's corner radius come from the managers
// and become CSS variables on the root: the shell paints its bands in the colour and
// width the window paints natively, so there is one source for both, and every other
// tone of the shell derives from that colour. The curve over the corners is the window's
// own border, drawn by the engine. Light or dark follows Go's theme pref.
useTheme(state);
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
    <TabStrip v-if="horizontal" />
    <TopBar />
    <div class="body">
      <div class="edge left"></div>
      <Sidebar v-if="!horizontal" />
      <PageView />
      <div class="edge right"></div>
    </div>
    <div class="edge bottom"></div>
  </div>
</template>

<style>
/* Colours from the Ulaa Browser design. The frame's colour and width come from Go with the
   snapshot, the same values the window paints natively, and the frame colour's tones
   (--frame, --accent, --bar, --icon, --accent-soft, --accent-tint) are set from it by
   tones.js. Icons and the sidebar's New Tab take a deeper ink, so the frame is the one
   thing in the full accent. */
:root {
  font: 14px/1.4 'Zoho Puvi', 'Zoho_Puvi', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
  color: var(--text);
  -webkit-font-smoothing: antialiased;
}

/* The root has no background: the page shows through wherever nothing is painted. */
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


/* Every control in the shell is a flat 28px square with a 6px radius. */
.icon-button {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.icon-button:hover:not(:disabled) {
  background: var(--accent-tint);
}

.icon-button:disabled {
  opacity: 0.4;
  cursor: default;
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}
</style>
