<script setup>
import { onMounted, ref } from 'vue';
import UpdateStatus from './UpdateStatus.vue';
import { setPageElement } from './regions.js';
import { state } from './state.js';

const el = ref(null);
onMounted(() => setPageElement(el.value));
</script>

<template>
  <div class="main">
    <div ref="el" class="page-view">
      <iframe v-if="state.engine === 'mock'" src="/mock/page" title="Mock page"></iframe>
    </div>
    <UpdateStatus />
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
</style>
