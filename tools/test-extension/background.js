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
    });
    return true;
  }
  if (message.type === 'state') {
    Promise.all([count(), chrome.tabs.query({})]).then(([pages, tabs]) => {
      respond({ pages, tabs: tabs.map((t) => ({ id: t.id, url: t.url, active: t.active })) });
    });
    return true;
  }
});
