<script setup>
import { nextTick, onUnmounted, ref } from 'vue';
import { send, state } from './state.js';

// This profile: its name in the list, and the template it was made from or last reset to.
// Reset writes the chosen template's values over the profile's settings, after a
// confirmation; Go owns all of it and answers with the profile as the list now has it.

// Go answers with the name it saved; an empty one goes back to it.
function setName(e) {
  const name = e.target.value.trim();
  if (name) send({ t: 'setProfileName', text: name });
  e.target.value = state.profile.name;
}

const chosen = ref(state.profile.template);
const dialog = ref(null);
const confirming = ref(false);

async function confirm() {
  confirming.value = true;
  await nextTick();
  dialog.value.showModal();
  window.addEventListener('keydown', onKey, true);
}

function done(reset) {
  if (reset) send({ t: 'resetProfile', text: chosen.value });
  window.removeEventListener('keydown', onKey, true);
  confirming.value = false;
}

// Escape answers Cancel, and only that: the settings page leaves an Escape it sees
// prevented alone, so this one takes it first.
function onKey(e) {
  if (e.key !== 'Escape') return;
  e.preventDefault();
  done(false);
}
onUnmounted(() => window.removeEventListener('keydown', onKey, true));
</script>

<template>
  <section aria-labelledby="profile">
    <h2 id="profile">Profile</h2>
    <div class="card">
      <div class="row">
        <label class="label" for="profile-name">Name</label>
        <input
          id="profile-name"
          class="field"
          spellcheck="false"
          :value="state.profile.name"
          @change="setName"
          @keydown.enter="$event.target.blur()"
        />
      </div>
      <div class="row">
        <span class="label">
          Template
          <span class="note">{{ state.profile.template }}</span>
        </span>
        <select v-model="chosen" aria-label="Reset to">
          <option v-for="t in state.profile.templates" :key="t" :value="t">{{ t }}</option>
        </select>
        <button type="button" class="text-button" @click="confirm">Reset…</button>
      </div>
    </div>
    <dialog v-if="confirming" ref="dialog" class="confirm" :aria-label="`Reset to ${chosen}`">
      <h2 class="confirm-title">Reset this profile to {{ chosen }}?</h2>
      <p class="confirm-text">Its settings become {{ chosen }}'s. History, cookies, logins and extensions stay.</p>
      <div class="confirm-buttons">
        <button type="button" class="text-button" @click="done(false)">Cancel</button>
        <button type="button" class="text-button primary" @click="done(true)">Reset</button>
      </div>
    </dialog>
  </section>
</template>

<style scoped>
.note {
  display: block;
  font-size: 12px;
  font-weight: 400;
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

.field:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.confirm {
  width: 360px;
  max-width: calc(100% - 32px);
  padding: 20px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
}

/* The page behind fades toward the bar's colour. */
.confirm::backdrop {
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.confirm-title {
  margin: 0;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
}

.confirm-text {
  margin: 16px 0 0;
}

.confirm-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}
</style>
