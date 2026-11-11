// Shows what the background script knows, and opens a tab through chrome.tabs.
chrome.runtime.sendMessage({ type: 'state' }).then((state) => {
  document.getElementById('pages').textContent = state.pages;
  for (const tab of state.tabs) {
    const item = document.createElement('li');
    item.textContent = tab.url;
    document.getElementById('tabs').append(item);
  }
});

document.getElementById('open').addEventListener('click', () => {
  chrome.tabs.create({ url: 'https://example.com/' });
});
