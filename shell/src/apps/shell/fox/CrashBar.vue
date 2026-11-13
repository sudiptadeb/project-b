<script setup>
import { send, state } from '@/sdk/vue.js';

// Says the last run crashed, where its log is and where macOS's report is, until the user
// closes it. Go decides whether there is one.
</script>

<template>
  <div v-if="state.crashLog" v-shell class="notice">
    <div class="bar" role="status">
      <span class="text">Project B crashed last time. The log is at {{ state.crashLog }}, macOS's report at {{ state.crashReport }}</span>
      <button class="icon-button" type="button" aria-label="Close" @click="send({ t: 'dismissCrash' })">
        <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68" /></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* A bar under the navigation bar, between the frame's side bands. It wraps, so both
   paths show whole. */
.notice {
  flex: none;
  padding: 0 var(--frame-width);
  background: var(--frame);
}

.bar {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
  padding: 4px 4px 4px 12px;
  box-sizing: border-box;
  background: var(--bar);
  border-bottom: 1px solid var(--line);
  font-size: 12px;
}

.text {
  flex: 1;
  overflow-wrap: anywhere;
}
</style>
