<script setup>
import { nextTick, ref, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import { send, state } from './state.js';

// The picker: a copy of the app that hasn't chosen its profile shows this in its one
// window. Choosing a profile opens it; New Profile makes one from a template and opens it.
watchEffect(() => paintTheme(state.windowColor, state.theme));

const making = ref(false);
const name = ref('');
const template = ref('');
const nameField = ref(null);

function open(id) {
  send({ t: 'open', id });
}

async function startMaking() {
  making.value = true;
  name.value = '';
  template.value = state.templates[0];
  await nextTick();
  nameField.value.focus();
}

function create() {
  if (!name.value.trim()) return;
  send({ t: 'create', name: name.value.trim(), template: template.value });
}
</script>

<template>
  <div v-if="state.ready" class="picker">
    <h1>Profiles</h1>
    <ul class="card" aria-label="Profiles">
      <li v-for="p in state.profiles" :key="p.id">
        <button type="button" class="row" @click="open(p.id)">
          <span class="dot" :style="{ background: p.color }"></span>
          <span class="name">{{ p.name }}</span>
        </button>
      </li>
    </ul>
    <form v-if="making" class="card new" aria-label="New profile" @submit.prevent="create">
      <input ref="nameField" v-model="name" class="text-field" aria-label="Name" placeholder="Name" @keydown.escape="making = false" />
      <select v-model="template" aria-label="Template">
        <option v-for="t in state.templates" :key="t" :value="t">{{ t }}</option>
      </select>
      <div class="buttons">
        <button type="button" class="text-button" @click="making = false">Cancel</button>
        <button type="submit" class="text-button primary" :disabled="!name.trim()">Create</button>
      </div>
    </form>
    <button v-else type="button" class="text-button new-button" @click="startMaking">New Profile</button>
  </div>
</template>

<style>
body {
  background: var(--bar);
}
</style>

<style scoped>
.picker {
  width: 320px;
  margin: 0 auto;
  padding-top: 72px;
}

h1 {
  margin: 0 0 16px;
  font-size: 16px;
  font-weight: 600;
}

.card {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

li + li {
  border-top: 1px solid var(--line);
}

.row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  height: 44px;
  padding: 0 16px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: inherit;
  font: inherit;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.row:hover {
  background: var(--accent-tint);
}

.row:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex: none;
}

.new {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 16px;
  padding: 16px;
}

.text-field,
select {
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  box-sizing: border-box;
  background: var(--surface);
  color: inherit;
  font: inherit;
}

.text-field::placeholder {
  color: var(--muted);
}

.text-field:focus-visible,
select:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.new-button {
  margin-top: 16px;
}
</style>
