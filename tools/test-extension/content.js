// Tells the background script about the page, and leaves its answer on the document
// element, where a test can read it.
chrome.runtime.sendMessage({ type: 'page' }).then((answer) => {
  document.documentElement.dataset.pbTestExtension = JSON.stringify(answer);
});
