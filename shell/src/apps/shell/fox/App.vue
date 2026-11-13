<script setup>
import { watchEffect } from 'vue';
import CrashBar from './CrashBar.vue';
import NavBar from './NavBar.vue';
import TabStrip from './TabStrip.vue';
import UpdateBar from './UpdateBar.vue';
import { state } from '@/sdk/vue.js';

// The tabs on top, the navigation bar under them, any notice under that, then the page.
// The frame's sides and foot are strips of their own, so nothing is painted under the
// page: it shows through the hole.
watchEffect(() => {
  document.documentElement.style.setProperty('--frame-width', state.frameWidth + 'px');
});
</script>

<template>
  <div class="window">
    <TabStrip />
    <NavBar />
    <CrashBar />
    <UpdateBar />
    <div class="body">
      <div class="edge side"></div>
      <div v-page="state.active" class="page"></div>
      <div class="edge side"></div>
    </div>
    <div class="edge foot"></div>
  </div>
</template>

<style>
.window {
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

.page {
  flex: 1;
  min-width: 0;
}

.edge {
  flex: none;
  background: var(--frame);
}

.edge.side {
  width: var(--frame-width);
}

.edge.foot {
  height: var(--frame-width);
}
</style>
