<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue';
import { pushEscape } from '@/ui/escape.js';
// The design system's dialog styles: .ui-dialog, its title and its buttons.
import '@/ui/Dialog.vue';
import { answerDialog, dialogFor, focusPage, send, state } from '@/sdk/vue.js';

// The active tab's page asks something (docs/daily-driver.md): the design system's dialog
// over that page only, so the tabs and the toolbar still work, with Go's title, words and
// buttons, and a field for a prompt. The page waits until it is answered; another tab
// shown hides it, and showing the tab again brings it back, as Go keeps it.
const dialog = computed(() => dialogFor(state, state.active));
// Where the host asking sits in a sign-in's words, set in bold so it reads first; -1 for none.
const hostAt = computed(() => (dialog.value?.host ? dialog.value.message.indexOf(dialog.value.host) : -1));
const text = ref('');
const password = ref('');
const field = ref(null);
const ok = ref(null);
const card = ref(null);
let pop = null;

watch(
  () => dialog.value?.id,
  async (id) => {
    pop?.();
    pop = null;
    if (!id) return;
    text.value = dialog.value.defaultText || '';
    password.value = '';
    pop = pushEscape(() => answer(false));
    send({ t: 'focusShell' });
    await nextTick();
    if (field.value) {
      field.value.focus();
      field.value.select();
    } else {
      ok.value?.focus();
    }
  },
  { immediate: true },
);

onUnmounted(() => pop?.());

// Tab and Shift+Tab go round the card's own controls while the page waits.
function trap(e) {
  const all = [...card.value.querySelectorAll('input, button')];
  const i = all.indexOf(document.activeElement);
  const next = all[(i + (e.shiftKey ? -1 : 1) + all.length) % all.length];
  e.preventDefault();
  next?.focus();
}

function answer(yes) {
  const d = dialog.value;
  if (!d) return;
  answerDialog(d.id, yes, yes ? text.value : '', yes ? password.value : '');
  focusPage();
}
</script>

<template>
  <div v-if="dialog" v-shell class="page-dialog">
    <div :key="dialog.id" ref="card" class="ui-dialog card" role="alertdialog" aria-modal="true" :aria-label="dialog.title" @keydown.tab="trap">
      <h2 class="ui-dialog-title">{{ dialog.title }}</h2>
      <p v-if="dialog.message" class="words">
        <template v-if="hostAt >= 0">{{ dialog.message.slice(0, hostAt) }}<b>{{ dialog.host }}</b>{{ dialog.message.slice(hostAt + dialog.host.length) }}</template>
        <template v-else>{{ dialog.message }}</template>
      </p>
      <input
        v-if="dialog.kind === 'prompt'"
        ref="field"
        v-model="text"
        class="text-field field"
        type="text"
        :aria-label="dialog.message || 'Answer'"
        @keydown.enter.prevent="answer(true)"
      />
      <template v-if="dialog.kind === 'signin'">
        <!-- The realm is the site's own words, so it reads as theirs, not ours. -->
        <p v-if="dialog.realm" class="realm">The site says: “{{ dialog.realm }}”</p>
        <div class="group">
          <label for="pb-sign-in-user">Username</label>
          <input id="pb-sign-in-user" ref="field" v-model="text" class="text-field" type="text" autocomplete="username" spellcheck="false" @keydown.enter.prevent="answer(true)" />
        </div>
        <div class="group">
          <label for="pb-sign-in-password">Password</label>
          <input id="pb-sign-in-password" v-model="password" class="text-field" type="password" autocomplete="current-password" @keydown.enter.prevent="answer(true)" />
        </div>
        <p v-if="dialog.insecure" class="insecure">This connection isn't secure: your password will be sent as plain text.</p>
      </template>
      <div class="ui-dialog-buttons">
        <button v-if="dialog.cancel" type="button" class="text-button" @click="answer(false)">{{ dialog.cancel }}</button>
        <button ref="ok" type="button" class="text-button primary" @click="answer(true)">{{ dialog.ok }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Over the page, which is the shell's while it waits, so no press reaches it; the page
   fades toward the bar's colour, as behind the design system's dialogs. */
.page-dialog {
  position: absolute;
  inset: 0;
  z-index: 5;
  display: grid;
  place-items: start center;
  padding-top: 64px;
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.card {
  display: block;
}

.words {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 50vh;
  overflow: auto;
}

.field {
  width: 100%;
  margin: 0;
  box-sizing: border-box;
}

/* The card's 16px rhythm: the buttons sit 16px under what is above them. */
.card .ui-dialog-buttons {
  margin-top: 16px;
}

.words:last-of-type {
  margin-bottom: 0;
}

.words + .field {
  margin-top: 16px;
}

/* A sign-in: the site's words 4px under ours, 16px to the first field, 12px between the
   two, each label 4px over its field. */
.realm {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.group {
  margin-top: 12px;
}

.realm + .group,
.words + .group {
  margin-top: 16px;
}

.group label {
  display: block;
  margin: 0 0 4px;
}

.group .text-field {
  width: 100%;
  box-sizing: border-box;
}

.insecure {
  margin: 16px 0 0;
  color: var(--danger);
  font-size: 12px;
}
</style>
