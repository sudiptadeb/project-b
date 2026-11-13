<script setup>
import { computed, ref } from 'vue';

// A colour from a row of presets, and with custom set, any colour from the system's
// picker behind the last swatch, which keeps a + on it so it reads as a picker. It is one
// Tab stop; the arrow keys move the choice, as a radio group's do. Colours are '#rrggbb'.
const props = defineProps({ modelValue: String, colors: Array, label: String, custom: Boolean });
const emit = defineEmits(['update:modelValue']);

const group = ref(null);
const isCustom = computed(() => !props.colors.includes(props.modelValue));

function onKey(e) {
  const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
  if (!step || e.target.type === 'color') return;
  e.preventDefault();
  const i = props.colors.indexOf(props.modelValue);
  const next = props.colors[(i + step + props.colors.length) % props.colors.length];
  emit('update:modelValue', next);
  requestAnimationFrame(() => group.value.querySelector('[aria-checked="true"]')?.focus());
}
</script>

<template>
  <div ref="group" class="ui-swatches" role="radiogroup" :aria-label="label" @keydown="onKey">
    <button
      v-for="c in colors"
      :key="c"
      type="button"
      role="radio"
      class="ui-swatch"
      :aria-checked="c === modelValue"
      :aria-label="c"
      :title="c"
      :tabindex="c === modelValue || (isCustom && c === colors[0]) ? 0 : -1"
      :style="{ background: c }"
      @click="emit('update:modelValue', c)"
    ></button>
    <label
      v-if="custom"
      class="ui-swatch custom"
      :class="{ on: isCustom }"
      title="Custom colour"
      :style="isCustom ? { background: modelValue } : null"
    >
      <input type="color" :value="modelValue" aria-label="Custom colour" @input="emit('update:modelValue', $event.target.value)" />
    </label>
  </div>
</template>

<style>
.ui-swatches {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

.ui-swatch {
  position: relative;
  flex: none;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px var(--line);
  cursor: pointer;
}

/* The chosen one: a ring in the text colour, apart from the swatch by the surface. */
.ui-swatch[aria-checked='true'],
.ui-swatch.custom.on {
  box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--text);
}

.ui-swatch:focus-visible,
.ui-swatch.custom:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}

/* The custom swatch: a + on the surface until a custom colour is chosen. */
.ui-swatch.custom {
  display: grid;
  place-items: center;
  background: var(--surface);
}

.ui-swatch.custom::before,
.ui-swatch.custom::after {
  content: '';
  position: absolute;
  width: 10px;
  height: 1.5px;
  border-radius: 1px;
  background: var(--icon);
}

.ui-swatch.custom::after {
  transform: rotate(90deg);
}

.ui-swatch.custom.on::before,
.ui-swatch.custom.on::after {
  background: rgba(255, 255, 255, 0.9);
}

.ui-swatch.custom input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  opacity: 0;
  cursor: pointer;
}

.touch .ui-swatches {
  gap: 16px;
}
</style>
