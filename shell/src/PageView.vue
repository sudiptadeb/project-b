<script setup>
import { computed, onMounted, ref } from 'vue';
import UpdateStatus from './UpdateStatus.vue';
import { setPageElement } from './regions.js';
import { state } from './state.js';

const el = ref(null);
onMounted(() => setPageElement(el.value));

// With a sidebar beside it the page's bottom-left corner is an inner corner; without
// one it reaches the window's curve.
const noSidebar = computed(() => state.tabLayout === 'horizontal' || !state.sidebar);
</script>

<template>
  <div class="main">
    <div ref="el" class="page-view">
      <iframe v-if="state.engine === 'mock'" src="/mock/page" title="Mock page"></iframe>
    </div>
    <UpdateStatus />
    <!-- macOS rounds the window itself, so where the page reaches a bottom corner a
         radius-sized square of frame colour, cut by the window's curve, keeps the frame
         one width around the corner. -->
    <div v-if="noSidebar" v-drag class="corner left"></div>
    <div v-drag class="corner right"></div>
  </div>
</template>

<style scoped>
/* The page is a plain rectangle: a hole in the shell with nothing painted on it. The
   update bubble is placed against this box, so it sits in the page's corner in either
   layout. */
.main {
  position: relative;
  flex: 1;
  min-width: 0;
}

.page-view {
  height: 100%;
}

iframe {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}

.corner {
  position: absolute;
  bottom: 0;
  width: var(--inner-radius);
  height: var(--inner-radius);
}

.corner.left {
  left: 0;
  background: radial-gradient(circle at 100% 0, transparent calc(var(--inner-radius) - 0.5px), var(--frame) calc(var(--inner-radius) + 0.5px));
}

.corner.right {
  right: 0;
  background: radial-gradient(circle at 0 0, transparent calc(var(--inner-radius) - 0.5px), var(--frame) calc(var(--inner-radius) + 0.5px));
}
</style>
