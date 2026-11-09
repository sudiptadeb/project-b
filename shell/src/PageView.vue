<script setup>
import { onMounted, ref } from 'vue';
import { setPageElement } from './regions.js';
import { state } from './state.js';

const el = ref(null);
onMounted(() => setPageElement(el.value));
</script>

<template>
  <div ref="el" class="page-view">
    <iframe v-if="state.engine === 'mock'" src="/mock/page" title="Mock page"></iframe>
  </div>
</template>

<style scoped>
/* The shadow paints the frame everywhere except this box, which stays a hole in the shell
   that the page shows through. */
.page-view {
  flex: 1;
  min-height: 0;
  margin: 0 4px 4px;
  box-shadow: 0 0 0 100vmax var(--frame);
}

iframe {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
