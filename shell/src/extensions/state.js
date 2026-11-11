// The extension manager's state, as Go sends it. Each page has its own copy: the shell's
// is the toolbar's buttons and the open popup, the settings page's everything the
// extension manager shows. Both come in the snapshot and in an 'extensions' message.

import { ref } from 'vue';

export const extensions = ref(null);

// extensionsMessage takes what a page's message has about extensions.
export function extensionsMessage(m) {
  if (m.t === 'snapshot' || m.t === 'extensions') extensions.value = m.extensions;
}
