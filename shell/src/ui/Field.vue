<script setup>
import { ref, watch } from 'vue';

// A text field that saves itself: Enter or leaving it commits what was typed, Escape puts
// back the saved value and leaves it. validate(text) returns '' or the error to show
// under it; while there is one, nothing is committed, the typed text stays, and Enter
// keeps focus there. value is what is saved now; the field follows it while it isn't
// being edited.
const props = defineProps({
  value: { type: String, default: '' },
  validate: { type: Function, default: () => '' },
  label: String,
  placeholder: String,
});
const emit = defineEmits(['commit', 'cancel']);

const text = ref(props.value);
const error = ref('');
const input = ref(null);
let editing = false;

watch(
  () => props.value,
  (v) => {
    if (!editing) text.value = v;
  },
);

function onInput() {
  editing = true;
  if (error.value) error.value = props.validate(text.value.trim());
}

// commit says whether what was typed is good: saved, or the same as before.
function commit() {
  const t = text.value.trim();
  error.value = props.validate(t);
  if (error.value) return false;
  editing = false;
  text.value = t;
  if (t !== props.value) emit('commit', t);
  return true;
}

function onEnter() {
  if (commit()) input.value.blur();
}

function onEscape(e) {
  e.preventDefault();
  e.stopPropagation();
  editing = false;
  error.value = '';
  text.value = props.value;
  input.value.blur();
  emit('cancel');
}

defineExpose({ focus: () => input.value.focus(), select: () => input.value.select(), commit });
</script>

<template>
  <div class="ui-field">
    <input
      ref="input"
      v-model="text"
      class="field"
      :class="{ invalid: error }"
      spellcheck="false"
      :aria-label="label"
      :aria-invalid="error ? 'true' : undefined"
      :placeholder="placeholder"
      @input="onInput"
      @keydown.enter.prevent="onEnter"
      @keydown.esc="onEscape"
      @blur="editing && commit()"
    />
    <div v-if="error" class="field-error" role="alert">{{ error }}</div>
  </div>
</template>

<style>
.ui-field {
  min-width: 0;
}

.ui-field .field {
  width: 100%;
}
</style>
