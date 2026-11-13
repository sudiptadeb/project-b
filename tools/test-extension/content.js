// Tells the background script about the page, and leaves its answer on the document
// element, where a test can read it.
chrome.runtime.sendMessage({ type: 'page' }).then((answer) => {
  document.documentElement.dataset.pbTestExtension = JSON.stringify(answer);
});

// The background script's messages to this tab: an answer to one, and a port.
chrome.runtime.onMessage.addListener((message, sender, respond) => {
  if (message.type === 'hello') {
    respond({ href: location.href });
  }
});
chrome.runtime.onConnect.addListener((port) => {
  port.onMessage.addListener((m) => {
    document.documentElement.dataset.pbTabMessages = JSON.stringify(m);
  });
  port.postMessage({ href: location.href });
});

// On a page asking for it, a picture of the tab, as captureVisibleTab takes it.
if (location.search === '?capture') {
  chrome.runtime.sendMessage({ type: 'capture' }).then((answer) => {
    document.documentElement.dataset.pbCapture = JSON.stringify(answer);
  });
}

// On a page asking for it, what the extension saw of a second window.
if (location.search === '?windows') {
  chrome.runtime.sendMessage({ type: 'windows' }).then((answer) => {
    document.documentElement.dataset.pbWindows = JSON.stringify(answer);
  });
}
