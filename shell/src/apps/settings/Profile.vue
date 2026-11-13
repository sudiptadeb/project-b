<script setup>
import { ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import Field from '@/ui/Field.vue';
import { send, state } from './state.js';

// This profile: its name in the list, the template it was made from or last reset to, and
// the shell it loads. Reset to template asks which template in a dialog that says what
// goes, then writes that template's values over the profile's settings; Go owns all of it
// and answers with the profile as the list now has it. Another shell loads in every window
// at once.

const nameError = (text) => (text ? '' : 'A profile needs a name');

const resetting = ref(false);
const chosen = ref('');

function askReset() {
  chosen.value = state.profile.template;
  resetting.value = true;
}

function reset() {
  send({ t: 'resetProfile', text: chosen.value });
  resetting.value = false;
}
</script>

<template>
  <section aria-label="Profile">
    <div class="card">
      <div class="row">
        <span class="label">Name</span>
        <Field
          class="name"
          label="Name"
          :value="state.profile.name"
          :validate="nameError"
          @commit="send({ t: 'setProfileName', text: $event })"
        />
      </div>
      <div class="row">
        <span class="label">
          Template
          <span class="secondary">{{ state.profile.template }}</span>
        </span>
        <button type="button" class="text-button" @click="askReset">Reset to template…</button>
      </div>
      <div class="row">
        <label class="label" for="profile-shell">Shell</label>
        <select id="profile-shell" :value="state.profile.shell" @change="send({ t: 'setShell', text: $event.target.value })">
          <option v-for="s in state.profile.shells" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
      </div>
    </div>
    <Dialog v-if="resetting" title="Reset this profile's settings?" @close="resetting = false">
      <div class="ui-dialog-part">
        <label for="profile-template">Template</label>
        <select id="profile-template" v-model="chosen" class="template">
          <option v-for="t in state.profile.templates" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>
      <div class="ui-dialog-part">
        <p>Its settings become {{ chosen }}'s. History, cookies, logins and extensions stay.</p>
      </div>
      <template #buttons>
        <button type="button" class="text-button" @click="resetting = false">Cancel</button>
        <button type="button" class="text-button danger" @click="reset">Reset</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
/* The name's field takes up to 320px of the row and gives way on a narrow page. */
.name {
  flex: 0 1 320px;
}

.template {
  width: 100%;
}
</style>
