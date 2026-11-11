<script setup>
import { send, state } from './state.js';

// Flags, as core/flags/registry.json defines them, each drawn by its type: a switch for a
// boolean, a select for an enum, a field for the rest. A change waits for the next launch,
// so Relaunch shows beside each flag whose saved value isn't the one running.

function set(flag, text) {
  send({ t: 'setFlag', id: flag.id, text });
}

// Go answers with the flags when it saves; a value it refuses goes back to the saved one.
function setField(flag, e) {
  set(flag, e.target.value.trim());
  e.target.value = flag.value;
}
</script>

<template>
  <section aria-labelledby="flags">
    <h2 id="flags">Flags</h2>
    <div class="card">
      <div v-for="f in state.flags" :key="f.id" class="row">
        <label class="label" :for="'flag-' + f.id">
          {{ f.name }}
          <span class="note">{{ f.built ? f.description : 'Not in this build' }}</span>
        </label>
        <button v-if="f.pending" type="button" class="text-button primary" @click="send({ t: 'relaunch' })">Relaunch</button>
        <input
          v-if="f.type === 'boolean'"
          :id="'flag-' + f.id"
          type="checkbox"
          role="switch"
          class="switch"
          :disabled="!f.built"
          :checked="f.value === 'on'"
          :aria-checked="String(f.value === 'on')"
          @change="set(f, $event.target.checked ? 'on' : '')"
        />
        <select v-else-if="f.type === 'enum'" :id="'flag-' + f.id" :disabled="!f.built" :value="f.value" @change="set(f, $event.target.value)">
          <option value="">Off</option>
          <option v-for="c in f.choices" :key="c" :value="c">{{ c }}</option>
        </select>
        <input
          v-else
          :id="'flag-' + f.id"
          :type="f.type === 'number' ? 'number' : 'text'"
          :inputmode="f.type === 'numbers' ? 'numeric' : undefined"
          :min="f.min"
          :max="f.max"
          class="field"
          placeholder="Off"
          spellcheck="false"
          :disabled="!f.built"
          :value="f.value"
          @change="setField(f, $event)"
          @keydown.enter="$event.target.blur()"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.note {
  display: block;
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
}

select {
  width: 200px;
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

.field::-webkit-inner-spin-button {
  appearance: none;
}

.field::placeholder {
  color: var(--muted);
}

.field:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
