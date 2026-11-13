// The extension manager's state, as Go sends it to the settings page: everything the
// extension manager shows, in the snapshot and in an 'extensions' message. The shell's
// copy, the toolbar's buttons and the open popup, is in its SDK's state.

import { ref } from 'vue';

export const extensions = ref(null);

// extensionsMessage takes what a page's message has about extensions.
export function extensionsMessage(m) {
  if (m.t === 'snapshot' || m.t === 'extensions') extensions.value = m.extensions;
}
