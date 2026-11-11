<script setup>
// A push pin at the toolbar's 1.5px stroke, filled when the button is on the toolbar.
defineProps({ on: Boolean });
</script>

<template>
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3.5h8"/><path d="M10 3.5h4v5.5l3.5 4.5h-11L10 9z" :fill="on ? 'currentColor' : 'none'"/><path d="M12 13.5v7"/></svg>
</template>
