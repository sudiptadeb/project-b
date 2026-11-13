<script setup>
import { ref, watch } from 'vue';
import { activeTab, focusPage, send, state } from '@/sdk/vue.js';

// The active tab's address. Typing asks Go for suggestions, listed in a plain panel
// under the field: Up and Down choose one and show it in the field, Enter goes, Escape
// puts the address back and gives the page the keyboard.
const input = ref(null);
const text = ref('');
// What was typed, which the field shows again when the choice goes past either end.
const typed = ref('');
const items = ref([]);
const chosen = ref(-1);
const editing = ref(false);
// The last question asked; an answer to an older one is dropped.
let query = 0;

function close() {
  items.value = [];
  chosen.value = -1;
  query++;
}

// Another tab's address replaces what the field held; the same tab's new address waits
// until the field is left.
watch(
  () => state.active,
  () => {
    close();
    text.value = activeTab.value.url;
  },
);
watch(
  () => activeTab.value.url,
  (url) => {
    if (!editing.value) text.value = url;
  },
);

function changed() {
  typed.value = text.value;
  chosen.value = -1;
  if (text.value.trim() === '') close();
  else send({ t: 'suggest', id: ++query, text: text.value });
}

function move(step) {
  const next = chosen.value + step;
  if (next < 0 || next >= items.value.length) {
    chosen.value = -1;
    text.value = typed.value;
  } else {
    chosen.value = next;
    text.value = items.value[next].url;
  }
}

function go(value) {
  if (value.trim() === '') return;
  send({ t: 'navigate', text: value.trim() });
  input.value.blur();
  focusPage();
}

function key(e) {
  if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && items.value.length) {
    e.preventDefault();
    move(e.key === 'ArrowDown' ? 1 : -1);
  } else if (e.key === 'Enter') {
    go(text.value);
  } else if (e.key === 'Escape') {
    input.value.blur();
    focusPage();
  }
}

// Left, the field shows the active tab's address again.
function left() {
  editing.value = false;
  close();
  text.value = activeTab.value.url;
}

defineExpose({
  focus() {
    input.value.focus();
    input.value.select();
  },
  blur: () => input.value.blur(),
  suggestions(m) {
    if (m.id === query) {
      items.value = m.items;
      chosen.value = -1;
    }
  },
});
</script>

<template>
  <div class="where">
    <input
      ref="input"
      v-model="text"
      class="address"
      aria-label="Address"
      role="combobox"
      aria-autocomplete="list"
      aria-controls="suggestions"
      :aria-expanded="items.length > 0"
      autocomplete="off"
      spellcheck="false"
      @input="changed"
      @keydown="key"
      @focus="editing = true"
      @blur="left"
    />
    <ul v-if="items.length" id="suggestions" v-shell class="suggestions" role="listbox" @mousedown.prevent>
      <li
        v-for="(item, i) in items"
        :key="item.url"
        role="option"
        :aria-selected="i === chosen"
        @click="go(item.url)"
      >
        <span class="title">{{ item.title || item.url }}</span>
        <span v-if="item.title" class="url">{{ item.url }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.where {
  position: relative;
  flex: 1;
  display: flex;
  min-width: 0;
  margin: 0 6px;
}

.address {
  flex: 1;
  min-width: 0;
  height: 28px;
  box-sizing: border-box;
  padding: 0 10px;
  border: 1px solid var(--line);
  background: var(--surface);
  font: 12px 'Space Mono', ui-monospace, monospace;
  outline: none;
}

.address:focus {
  box-shadow: var(--focus-ring);
}

.suggestions {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin: 0;
  padding: 0;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-top: 0;
}

li {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 10px;
  white-space: nowrap;
}

li:hover,
li[aria-selected='true'] {
  background: var(--accent-tint);
}

.title,
.url {
  overflow: hidden;
  text-overflow: ellipsis;
}

.title {
  flex: none;
  max-width: 60%;
}

.url {
  flex: 1;
  color: var(--muted);
  font: 12px 'Space Mono', ui-monospace, monospace;
}
</style>
