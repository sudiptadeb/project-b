// What a shell sends about downloads. What Go sends is state.downloads (shell.js); how
// the list looks is the shell's own. The Save panel is the system's.

import { send } from './channel.js';

// downloadAction does one thing to download id: 'open' or 'reveal' its file once done,
// 'cancel', 'pause' or 'resume' it while it runs, or 'remove' it from the list, which
// leaves the file where it is.
export function downloadAction(id, action) {
  send({ t: 'downloadAction', id, action });
}

// clearDownloads removes every finished download from the list.
export function clearDownloads() {
  send({ t: 'clearDownloads' });
}
