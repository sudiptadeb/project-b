<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import { pushEscape } from './escape.js';

// A modal dialog, the one pattern for every question: 360px, a 15px title, the body, and
// the buttons at the bottom right, which the caller puts in the buttons slot. Mount it
// with v-if to open it. Escape and the backdrop's own cancel emit close; when it goes,
// focus goes back to what had it before it opened.
defineProps({ title: String });
const emit = defineEmits(['close']);

const dialog = ref(null);
let before = null;
let pop = null;

onMounted(() => {
  before = document.activeElement;
  dialog.value.showModal();
  pop = pushEscape(() => emit('close'));
});
onUnmounted(() => {
  pop();
  if (before && before.isConnected) before.focus();
});
</script>

<template>
  <dialog ref="dialog" class="ui-dialog" :aria-label="title" @cancel.prevent="emit('close')">
    <h2 class="ui-dialog-title">{{ title }}</h2>
    <slot />
    <div class="ui-dialog-buttons"><slot name="buttons" /></div>
  </dialog>
</template>

<style>
.ui-dialog {
  width: 360px;
  max-width: calc(100% - 32px);
  padding: 20px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
}

/* The page behind fades toward the bar's colour. */
.ui-dialog::backdrop {
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.ui-dialog-title {
  margin: 0 0 16px;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
}

.ui-dialog p {
  margin: 0;
  color: var(--muted);
}

/* A field in a dialog: its label above it, 12px and muted. */
.ui-dialog label {
  display: block;
  margin: 0 0 4px;
  color: var(--muted);
  font-size: 12px;
}

.ui-dialog .ui-dialog-part + .ui-dialog-part {
  margin-top: 16px;
}

.ui-dialog-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}
</style>
