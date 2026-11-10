<script setup>
import { onMounted, ref } from 'vue';
import CrashNotice from './CrashNotice.vue';
import Settings from './Settings.vue';
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
    <CrashNotice />
    <UpdateStatus />
    <Settings v-if="state.settings" />
  </div>
</template>

<style scoped>
/* The page is a plain rectangle: a hole in the shell with nothing painted on it. The
   crash notice, the update bubble and the settings panel are placed against this box, so
   each sits in the page's corner in either layout. */
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
