<script setup>
import { ref } from 'vue';

// Two to four short choices side by side, one of them on: Theme's System, Light and Dark.
// It is one Tab stop; the arrow keys move the choice, as a radio group's do. options is a
// list of { value, label }.
const props = defineProps({ modelValue: String, options: Array, label: String });
const emit = defineEmits(['update:modelValue']);

const group = ref(null);

function onKey(e) {
  const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
  if (!step) return;
  e.preventDefault();
  const i = props.options.findIndex((o) => o.value === props.modelValue);
  const next = props.options[(i + step + props.options.length) % props.options.length];
  emit('update:modelValue', next.value);
  requestAnimationFrame(() => group.value?.querySelector('[aria-checked="true"]')?.focus());
}
</script>

<template>
  <div ref="group" class="ui-segmented" role="radiogroup" :aria-label="label" @keydown="onKey">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      role="radio"
      :aria-checked="o.value === modelValue"
      :tabindex="o.value === modelValue ? 0 : -1"
      @click="emit('update:modelValue', o.value)"
    >{{ o.label }}</button>
  </div>
</template>

<style>
.ui-segmented {
  flex: none;
  display: inline-flex;
  height: 30px;
  padding: 2px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bar);
}

.ui-segmented button {
  height: 24px;
  padding: 0 12px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.ui-segmented button:hover {
  color: var(--text);
}

.ui-segmented button[aria-checked='true'] {
  background: var(--surface);
  color: var(--text);
  box-shadow: 0 0 0 1px var(--line);
}

.ui-segmented button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.touch .ui-segmented {
  height: 40px;
}

.touch .ui-segmented button {
  height: 34px;
}
</style>
