<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { addressField } from './address.js';
import { activeTab, focusPage, send, state, workspaceOf } from '@/sdk/vue.js';

// The active tab's address. Focused, it selects everything and grows a little; typing
// asks Go for suggestions, which open in a panel attached under it, as wide as it. Up and
// Down choose one, Enter or a click goes. Escape closes the panel, then puts the address
// back and gives the keyboard to the page.
const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);

let latestQuery = 0;

const open = computed(() => focused.value && items.value.length > 0);
const workspace = computed(() => workspaceOf(activeTab.value));

function close() {
  items.value = [];
  highlight.value = -1;
  latestQuery++;
}

function onInput() {
  typed.value = text.value;
  highlight.value = -1;
  if (text.value.trim() === '') {
    close();
    return;
  }
  send({ t: 'suggest', id: ++latestQuery, text: text.value });
}

// Moving through the list shows that item in the field. Up from the first item, or down
// from the last, comes back to what the user typed.
function move(delta) {
  const next = highlight.value + delta;
  if (next < 0 || next >= items.value.length) {
    highlight.value = -1;
    text.value = typed.value;
    return;
  }
  highlight.value = next;
  text.value = items.value[next].url;
}

function onKeydown(e) {
  selectOnMouseUp = false;
  switch (e.key) {
    case 'ArrowDown':
      if (open.value) {
        e.preventDefault();
        move(1);
      }
      break;
    case 'ArrowUp':
      if (open.value) {
        e.preventDefault();
        move(-1);
      }
      break;
    case 'Escape':
      if (open.value) {
        e.preventDefault();
        text.value = typed.value;
        close();
      } else {
        text.value = activeTab.value.url;
        input.value.blur();
        focusPage();
      }
      break;
  }
}

function go(target) {
  const value = (target ?? text.value).trim();
  if (value === '') return;
  text.value = value;
  close();
  send({ t: 'navigate', text: value });
  input.value.blur();
  focusPage();
}

// Focusing selects everything. When a click did the focusing, its mouse up would put the
// caret down and clear the selection, so that one mouse up selects again. The first key
// clears this, so later clicks place the caret as usual.
let selectOnMouseUp = false;

function onFocus() {
  focused.value = true;
  typed.value = text.value;
  selectOnMouseUp = true;
  nextTick(() => input.value.select());
}

function onMouseUp(e) {
  if (selectOnMouseUp) {
    e.preventDefault();
    input.value.select();
    selectOnMouseUp = false;
  }
}

function onBlur() {
  focused.value = false;
  close();
  text.value = activeTab.value.url;
}

// The shell page has to hold the keyboard before the field can.
function focusField() {
  send({ t: 'focusShell' });
  input.value.focus();
}

onMounted(() => {
  addressField.suggestions = (id, list) => {
    if (id === latestQuery) {
      items.value = list;
      highlight.value = -1;
    }
  };
  addressField.focus = () => input.value.focus();
});

watch(
  () => activeTab.value.url,
  (url) => {
    if (!focused.value) text.value = url;
  },
);

// A tab that becomes active with nothing in it wants the address field; one with a page
// wants the page.
watch(
  () => state.active,
  () => {
    close();
    text.value = activeTab.value.url;
    if (activeTab.value.url === '') {
      focusField();
    } else {
      input.value.blur();
      focusPage();
    }
  },
);
</script>

<template>
  <form class="field" :class="{ focused, open }" @submit.prevent="go()">
    <input
      ref="input"
      v-model="text"
      aria-label="Address"
      placeholder="Search or enter address"
      role="combobox"
      :aria-expanded="open"
      aria-controls="suggestions"
      :aria-activedescendant="highlight >= 0 ? `suggestion-${highlight}` : undefined"
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
      @input="onInput"
      @keydown="onKeydown"
      @mouseup="onMouseUp"
      @focus="onFocus"
      @blur="onBlur"
    />
    <span v-if="workspace" class="workspace">
      <span class="dot" :style="{ background: workspace.color }"></span>
      {{ workspace.name }}
    </span>
    <ul v-if="open" id="suggestions" v-shell class="list" role="listbox" @mousedown.prevent>
      <li
        v-for="(item, i) in items"
        :id="`suggestion-${i}`"
        :key="item.url"
        role="option"
        :aria-selected="i === highlight"
        :class="{ active: i === highlight }"
        @mouseenter="highlight = i"
        @click="go(item.url)"
      >
        <span class="title">{{ item.title || item.url }}</span>
        <span v-if="item.title" class="dash" aria-hidden="true"></span>
        <span v-if="item.title" class="url">{{ item.url }}</span>
      </li>
    </ul>
  </form>
</template>

<style scoped>
/* A rounded field with a hairline. Focused, it grows past its column by 4px on each side
   and 4px in height, without moving what is beside it, and shows the focus ring. With
   suggestions it floats, and the panel continues it downwards. */
.field {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
  height: 32px;
  margin: 0;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-sizing: border-box;
}

.field.focused {
  height: 36px;
  margin: 0 -4px;
  box-shadow: var(--focus-ring);
}

.field.open {
  border-radius: 8px 8px 0 0;
  box-shadow: var(--shadow-float);
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 13px;
}

.field.focused input {
  padding: 0 16px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

/* The workspace the active tab is in: its colour and its name, at the field's right. */
.workspace {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  max-width: 40%;
  margin-right: 12px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

/* Under the field, as wide as it, one surface with it. */
.list {
  position: absolute;
  top: 100%;
  left: -1px;
  right: -1px;
  z-index: 10;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 0 0 8px 8px;
  box-shadow: var(--shadow-float);
}

li {
  display: flex;
  align-items: center;
  height: 32px;
  padding: 0 12px;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
}

li.active {
  background: var(--accent-tint);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.title {
  flex: none;
  max-width: 60%;
}

.url {
  flex: 1;
  min-width: 0;
  color: var(--muted);
}

.dash {
  flex: none;
  width: 5px;
  height: 1px;
  margin: 0 8px;
  background: var(--muted);
}
</style>
