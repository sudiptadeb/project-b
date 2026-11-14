<script setup>
import CrashNotice from './CrashNotice.vue';
import FindBar from './FindBar.vue';
import PageDialog from './PageDialog.vue';
import UpdateStatus from './UpdateStatus.vue';
import { state } from '@/sdk/vue.js';
</script>

<!-- The page hole shows the active tab. -->
<template>
  <div class="main">
    <div v-page="state.active" class="page-view"></div>
    <CrashNotice />
    <FindBar />
    <PageDialog />
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
