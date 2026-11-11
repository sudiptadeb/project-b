<script setup>
import { send, state } from './state.js';

// Flags: switches the app reads as it starts, saved in the profile by Go. A change waits
// for the next launch, so Relaunch shows while what is saved differs from what runs. The
// build flags say what this build was made with; tools/build.sh sets them.

function setText(flag, e) {
  send({ t: 'setFlag', id: flag.id, text: e.target.value.trim() });
  // Go answers with the flags when it saves; a value it refuses goes back to the saved one.
  e.target.value = flag.value;
}
</script>

<template>
  <div>
    <section aria-labelledby="flags">
      <div class="head">
        <h2 id="flags">Flags</h2>
        <button v-if="state.flags.relaunch" type="button" class="text-button primary" @click="send({ t: 'relaunch' })">Relaunch</button>
      </div>
      <div class="card">
        <label v-for="f in state.flags.flags" :key="f.id" class="row" :class="{ off: !f.built }">
          <span class="label">
            {{ f.name }}
            <span v-if="!f.built" class="note">Not in this build</span>
          </span>
          <input
            v-if="f.kind === 'switch'"
            type="checkbox"
            role="switch"
            class="switch"
            :disabled="!f.built"
            :checked="f.value === 'on'"
            :aria-checked="String(f.value === 'on')"
            @change="send({ t: 'setFlag', id: f.id, text: $event.target.checked ? 'on' : '' })"
          />
          <input
            v-else
            type="text"
            class="field"
            placeholder="Off"
            spellcheck="false"
            :value="f.value"
            @change="setText(f, $event)"
            @keydown.enter="$event.target.blur()"
          />
        </label>
      </div>
    </section>

    <section aria-labelledby="build-flags">
      <h2 id="build-flags">Build</h2>
      <div class="card">
        <div v-for="b in state.flags.builds" :key="b.id" class="row">
          <span class="label">{{ b.name }}</span>
          <span class="value">{{ b.on ? 'In this build' : 'Not in this build' }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 30px;
  margin-top: -6px;
}

.note {
  display: block;
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
}

.value {
  font-size: 13px;
  color: var(--muted);
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

.field::placeholder {
  color: var(--muted);
}

.field:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
