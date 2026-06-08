// Adds a right-click entry that hands the chosen URL to the native omagrab app
// via the custom "omagrab:" URL scheme (registered by omagrab-url.desktop).

function createMenu() {
  // removeAll first so reloads / service-worker restarts can't hit a
  // "duplicate id" error that would leave the menu uncreated.
  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({
      id: "omagrab",
      title: "⏬ Download with omagrab",
      contexts: ["link", "page", "video", "audio"],
    });
  });
}

chrome.runtime.onInstalled.addListener(createMenu);
chrome.runtime.onStartup.addListener(createMenu);
// also run on every service-worker spin-up, in case the events above were missed
createMenu();

chrome.contextMenus.onClicked.addListener((info, tab) => {
  // Prefer the link the user clicked; otherwise fall back to the page itself
  // (e.g. right-clicking anywhere on a video watch page).
  const url = info.linkUrl || info.pageUrl || (tab && tab.url);
  if (!url || !tab || tab.id == null) return;

  const target = "omagrab:" + encodeURIComponent(url);

  // Setting location.href on the active tab triggers the OS protocol handler
  // without spawning a leftover blank tab. The page itself stays put.
  chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: (u) => { window.location.href = u; },
    args: [target],
  });
});
