<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import RowMenu from '@/ui/RowMenu.vue';
import Swatches from '@/ui/Swatches.vue';
import { send, state } from './state.js';

// The profile's workspaces: a list of rows, each with its colour, its name and the number
// of tabs open in it, and a menu to edit or move it. New workspace and Edit open one
// dialog with the name and the colour. Go owns the list and answers with it as it is now;
// it refuses a duplicate name too, but the dialog says so first.

// The colours a workspace starts from; any other is a custom one.
const colors = ['#3470d0', '#d03434', '#4f9e9e', '#e8a317', '#8a4fd0', '#2e9d5b', '#34404f'];

function tabs(w) {
  const n = state.workspaceTabs?.[w.id] || 0;
  return n === 0 ? 'No tabs' : n === 1 ? '1 tab' : `${n} tabs`;
}

// The row menus by workspace id, so a right-click on a row opens its menu.
const menus = {};

function items(w, i) {
  return [
    { label: 'New tab in workspace', run: () => send({ t: 'openInWorkspace', id: w.id }) },
    { label: 'Edit…', run: () => edit(w) },
    { label: 'Move up', run: () => move(w, -1), disabled: i === 0 },
    { label: 'Move down', run: () => move(w, 1), disabled: i === state.workspaces.length - 1 },
  ];
}

// The row moves when Go answers, and the browser drops focus from a button that moves, so
// the moved row's menu button gets it back.
let moved = '';
function move(w, by) {
  moved = w.id;
  send({ t: 'moveWorkspace', id: w.id, by });
}
watch(
  () => state.workspaces,
  async () => {
    if (!moved) return;
    const id = moved;
    moved = '';
    await nextTick();
    if (document.activeElement === document.body) document.querySelector(`[data-workspace="${id}"] .ui-row-menu > button`)?.focus();
  },
);

// The dialog: editing is the workspace being edited, or null for a new one. Enter saves
// from anywhere in it, a swatch too.
const open = ref(false);
const editing = ref(null);
const name = ref('');
const color = ref(colors[0]);
const error = ref('');
const nameInput = ref(null);

function create() {
  editing.value = null;
  name.value = '';
  color.value = colors.find((c) => !state.workspaces.some((w) => w.color === c)) || colors[state.workspaces.length % colors.length];
  show();
}

function edit(w) {
  editing.value = w;
  name.value = w.name;
  color.value = w.color;
  show();
}

async function show() {
  error.value = '';
  open.value = true;
  await nextTick();
  nameInput.value.focus();
  nameInput.value.select();
}

// Names are unique in the list whatever their case, as Go keeps them.
function validate(text) {
  if (!text) return 'A workspace needs a name';
  const same = state.workspaces.find((w) => w !== editing.value && w.name.toLowerCase() === text.toLowerCase());
  return same ? `There's already a workspace called ${same.name}` : '';
}

function onInput() {
  if (error.value) error.value = validate(name.value.trim());
}

function save() {
  const text = name.value.trim();
  error.value = validate(text);
  if (error.value) {
    nameInput.value.focus();
    return;
  }
  if (editing.value) send({ t: 'editWorkspace', id: editing.value.id, text, color: color.value });
  else send({ t: 'createWorkspace', text, color: color.value });
  open.value = false;
}

const title = computed(() => (editing.value ? 'Edit workspace' : 'New workspace'));
</script>

<template>
  <section aria-labelledby="workspaces">
    <div class="heading">
      <h2 id="workspaces">Workspaces</h2>
      <button type="button" class="text-button" @click="create">New workspace…</button>
    </div>
    <div class="card">
      <div v-if="!state.workspaces.length" class="row empty">No workspaces yet</div>
      <div
        v-for="(w, i) in state.workspaces"
        :key="w.id"
        class="row"
        :data-workspace="w.id"
        @contextmenu.prevent="menus[w.id]?.open()"
      >
        <span class="dot" :style="{ background: w.color }" aria-hidden="true"></span>
        <span class="label">
          <span class="name" :title="w.name">{{ w.name }}</span>
          <span class="secondary">{{ tabs(w) }}</span>
        </span>
        <RowMenu :ref="(m) => (menus[w.id] = m)" :items="items(w, i)" :label="`Actions for ${w.name}`" />
      </div>
    </div>
    <Dialog v-if="open" :title="title" @close="open = false">
      <form id="workspace-form" @submit.prevent="save" @keydown.enter.prevent="save">
        <div class="ui-dialog-part">
          <label for="workspace-name">Name</label>
          <input
            id="workspace-name"
            ref="nameInput"
            v-model="name"
            class="text-field"
            :class="{ invalid: error }"
            spellcheck="false"
            :aria-invalid="error ? 'true' : undefined"
            aria-describedby="workspace-name-error"
            @input="onInput"
          />
          <div v-if="error" id="workspace-name-error" class="text-field-error" role="alert">{{ error }}</div>
        </div>
        <div class="ui-dialog-part">
          <label>Colour</label>
          <Swatches v-model="color" :colors="colors" label="Colour" custom />
        </div>
      </form>
      <template #buttons>
        <button type="button" class="text-button" @click="open = false">Cancel</button>
        <button type="submit" form="workspace-form" class="text-button primary">{{ editing ? 'Save' : 'Create' }}</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
/* The heading row is 30px high, as it is with no button. */
.heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 30px;
}

.dot {
  flex: none;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px var(--line);
}

.name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty {
  color: var(--muted);
}

.text-field {
  width: 100%;
}
</style>
