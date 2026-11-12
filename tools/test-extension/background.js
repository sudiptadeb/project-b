// Counts the pages its content script reports, keeps the count in storage and shows it on
// the badge. Answers the popup and the content script by message.

async function count() {
  return (await chrome.storage.local.get({ pages: 0 })).pages;
}

chrome.runtime.onMessage.addListener((message, sender, respond) => {
  if (message.type === 'page') {
    count().then(async (pages) => {
      pages += 1;
      await chrome.storage.local.set({ pages });
      await chrome.action.setBadgeText({ text: String(pages) });
      respond({ pages, tab: sender.tab.id, version: chrome.runtime.getManifest().version });
      // And messages the tab back, once and over a port, as tabs.sendMessage and
      // tabs.connect do.
      const answer = await chrome.tabs.sendMessage(sender.tab.id, { type: 'hello' });
      const port = chrome.tabs.connect(sender.tab.id, { name: 'test' });
      port.onMessage.addListener((m) => port.postMessage({ type: 'answered', sent: answer.href, got: m.href }));
    });
    return true;
  }
  if (message.type === 'capture') {
    chrome.tabs.captureVisibleTab({ format: 'png' }).then(
      (url) => respond({ url: url.slice(0, 22), length: url.length }),
      (error) => respond({ error: error.message }),
    );
    return true;
  }
  if (message.type === 'state') {
    // A popup opened: the icon becomes one drawn here, a green square.
    const canvas = new OffscreenCanvas(16, 16);
    const context = canvas.getContext('2d');
    context.fillStyle = '#0a0';
    context.fillRect(0, 0, 16, 16);
    chrome.action.setIcon({ imageData: context.getImageData(0, 0, 16, 16) });
    Promise.all([count(), chrome.tabs.query({})]).then(([pages, tabs]) => {
      respond({ pages, tabs: tabs.map((t) => ({ id: t.id, url: t.url, active: t.active })) });
    });
    return true;
  }
});
