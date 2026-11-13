<script setup>
import { onMounted, ref } from 'vue';
import { answerExtensionPrompt } from '@/sdk/extensions.js';

// An install or a removal waiting for the user's answer, over the window. Escape answers
// No, and Go takes it away.
defineProps({ prompt: { type: Object, required: true } });
const dialog = ref(null);
onMounted(() => dialog.value.showModal());
</script>

<template>
  <!-- The cover makes the whole window the shell's while it asks, so no press reaches the page. -->
  <div v-shell class="cover">
    <dialog
      ref="dialog"
      class="pbx-prompt"
      :aria-label="`${prompt.remove ? 'Remove' : 'Add'} ${prompt.name}`"
      @cancel.prevent="answerExtensionPrompt(false)"
    >
      <p class="question">{{ prompt.remove ? 'Remove' : 'Add' }} "{{ prompt.name }}"?</p>
      <template v-if="!prompt.remove">
        <p>Version {{ prompt.version }}</p>
        <ul v-if="prompt.permissions?.length">
          <li v-for="permission in prompt.permissions" :key="permission">{{ permission }}</li>
        </ul>
      </template>
      <div class="answers">
        <button @click="answerExtensionPrompt(false)">Cancel</button>
        <button class="primary" @click="answerExtensionPrompt(true)">{{ prompt.remove ? 'Remove' : 'Add extension' }}</button>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.cover {
  position: fixed;
  inset: 0;
}

dialog {
  width: 360px;
  box-sizing: border-box;
  padding: 20px;
  border: 1px solid var(--line);
  background: var(--surface);
  color: var(--text);
  line-height: 1.4;
}

dialog::backdrop {
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

p {
  margin: 0 0 8px;
  color: var(--muted);
}

.question {
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
}

ul {
  margin: 0;
  padding: 8px 12px 8px 28px;
  border: 1px solid var(--line);
}

.answers {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}

button {
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--line);
  background: none;
  font-weight: 500;
}

button:hover {
  background: var(--accent-tint);
}

.primary,
.primary:hover {
  border-color: var(--accent);
  background: var(--accent);
  color: var(--on-accent);
}
</style>
