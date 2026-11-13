<script setup>
import { ref } from 'vue';
import { send, state } from './state.js';

// The profile's workspaces: each one's colour and name, which can be changed, and New
// Workspace, with a name and a colour. Go owns the list and answers with it as it is now.

// Go answers with the name it saved; an empty one goes back to it.
function rename(w, e) {
  const name = e.target.value.trim();
  if (name) send({ t: 'renameWorkspace', id: w.id, text: name });
  e.target.value = w.name;
}

const name = ref('');
const color = ref('#3470d0');

function create() {
  send({ t: 'createWorkspace', text: name.value.trim(), color: color.value });
  name.value = '';
}
</script>

<template>
  <section aria-labelledby="workspaces">
    <h2 id="workspaces">Workspaces</h2>
    <div class="card">
      <div v-for="w in state.workspaces" :key="w.id" class="row">
        <span class="dot" :style="{ background: w.color }" aria-hidden="true"></span>
        <input
          class="field"
          spellcheck="false"
          :aria-label="`Name of ${w.name}`"
          :value="w.name"
          @change="rename(w, $event)"
          @keydown.enter="$event.target.blur()"
        />
      </div>
      <form class="row" @submit.prevent="create">
        <label class="dot pick" title="Colour" :style="{ background: color }">
          <input v-model="color" type="color" aria-label="New workspace's colour" />
        </label>
        <input v-model="name" class="field" spellcheck="false" placeholder="Name" aria-label="New workspace's name" />
        <button type="submit" class="text-button" :disabled="!name.trim()">New Workspace</button>
      </form>
    </div>
  </section>
</template>

<style scoped>
.dot {
  position: relative;
  flex: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.12);
}

/* The colour input fills the dot, invisible, so a click opens the picker. */
.dot.pick {
  cursor: pointer;
}

.dot.pick:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}

.dot.pick input {
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

.field {
  width: 200px;
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  box-sizing: border-box;
  background: var(--surface);
  color: inherit;
  font: inherit;
}

.field:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
