<script setup>
import { computed } from 'vue';

// What stands before a tab's title, in the strip and in the list of all tabs: what went
// wrong with it, else a spinner while it loads, else its favicon, else a blank page. The
// glyphs take the text's colour.
const props = defineProps({ tab: { type: Object, required: true } });

const status = computed(() => {
  if (props.tab.blocked) return 'Blocked';
  if (props.tab.crashed) return 'Crashed';
  if (props.tab.error) return 'Failed';
  return '';
});
</script>

<template>
  <svg v-if="status" class="icon status" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" role="img" :aria-label="status">
    <title>{{ status }}</title>
    <path d="M8 1.75 14.75 13.5H1.25Z" stroke-linejoin="round" />
    <path d="M8 6.25v3.25M8 11.6v.05" />
  </svg>
  <!-- SVG's own animation turns it, so the shell's layout isn't measured every frame
       while a page loads, as it is during a CSS animation. -->
  <svg v-else-if="tab.loading" class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" role="img" aria-label="Loading">
    <path d="M8 1.75A6.25 6.25 0 1 1 1.75 8">
      <animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur="0.9s" repeatCount="indefinite" />
    </path>
  </svg>
  <img v-else-if="tab.favicon" class="icon" :src="tab.favicon" alt="" width="16" height="16" />
  <svg v-else class="icon blank" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linejoin="round" aria-hidden="true">
    <path d="M3.5 1.75h6l3 3v9.5h-9Z" />
    <path d="M9.5 1.75v3h3" />
  </svg>
</template>

<style scoped>
.icon {
  display: block;
  flex: none;
  width: 16px;
  height: 16px;
}

.status {
  color: var(--danger);
}

.blank {
  opacity: 0.7;
}
</style>
