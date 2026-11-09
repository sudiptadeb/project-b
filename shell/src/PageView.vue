<script setup>
import { onMounted, ref } from 'vue';
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
  </div>
</template>

<style scoped>
/* The frame colour is a border on the right and bottom, and on the left when there is
   no sidebar. Inside it, the page is a hole in the shell with a rounded top-left corner:
   the shadow paints the bar colour everywhere around the hole, clipped to this box. */
.main {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  border: 0 solid var(--frame);
  border-width: 0 4px 4px 0;
}

.main.no-sidebar {
  border-left-width: 4px;
}

.page-view {
  height: 100%;
  border-radius: 8px 0 0 0;
  box-shadow: 0 0 0 100vmax var(--bar);
}

iframe {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: inherit;
}
</style>
