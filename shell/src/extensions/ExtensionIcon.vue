<script setup>
// An extension's icon at one size. A package may have no icon; then, as Chrome does, the
// first letter of its name stands in.
defineProps({ src: String, name: String, size: { type: Number, default: 16 } });
</script>

<template>
  <img v-if="src" class="pbx-icon" :src="src" alt="" :width="size" :height="size" />
  <span v-else class="pbx-icon pbx-letter" aria-hidden="true" :style="{ width: size + 'px', height: size + 'px', fontSize: size * 0.62 + 'px' }">{{ name.slice(0, 1).toUpperCase() }}</span>
</template>

<style scoped>
.pbx-icon {
  flex: none;
  display: block;
}

.pbx-letter {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 3px;
  background: var(--accent-soft);
  color: var(--text);
  font-weight: 600;
  line-height: 1;
}
</style>
