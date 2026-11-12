<script setup>
import { onMounted, ref, watch } from 'vue';
import CrashNotice from './CrashNotice.vue';
import UpdateStatus from './UpdateStatus.vue';
import { measureNow, setPageElement } from './regions.js';
import { state } from './state.js';

// The page hole shows the active tab.
const el = ref(null);
onMounted(() => setPageElement(el.value, () => state.active));
watch(() => state.active, measureNow);
</script>

<template>
  <div class="main">
    <div ref="el" class="page-view"></div>
    <CrashNotice />
    <UpdateStatus />
  </div>
</template>

<style scoped>
/* The page is a plain rectangle: a hole in the shell with nothing painted on it. The
   crash notice and the update bubble are placed against this box, so
   each sits in the page's corner in either layout. */
.main {
  position: relative;
  flex: 1;
  min-width: 0;
}

.page-view {
  height: 100%;
}

</style>
