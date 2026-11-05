// A Table viewer's files in windows of their own. A tab dragged past the edge of
// the window, or a pane's button, moves a file to one; a tab dragged back, or the
// button there, returns it. Each window names itself in window.name, which outlives
// a reload, and they talk over a BroadcastChannel: a new window reports to its home,
// the window with the table, which sends it row moves when it was last active.
"use strict";

// The drag data type that carries a tab between windows.
const tabDragType = "application/x-rotari-tab";
// How long a window waits for its home to take a file back.
const dockTimeoutMs = 500;

if (!window.name.startsWith("rotari-"))
  window.name = "rotari-main-" + randomWindowPart();
const windowID = window.name;
// homeWindow is the window with the table, in a window of its own; otherwise null.
let homeWindow = null;
// activeWindow is where row moves go: the window last clicked in, as the table's
// window knows it.
let activeWindow = windowID;
let windowChannel =
  typeof BroadcastChannel === "function"
    ? new BroadcastChannel("rotari-table-viewer")
    : null;
if (windowChannel) {
  windowChannel.onmessage = (event) => handleWindowMessage(event.data);
}
// dockTimer waits for an answer to giving a file back.
let dockTimer = null;

function randomWindowPart() {
  return Math.random().toString(36).slice(2);
}

function postToWindow(to, type, fields) {
  if (windowChannel)
    windowChannel.postMessage(
      Object.assign({ to, from: windowID, type }, fields),
    );
}

function handleWindowMessage(message) {
  if (message.from === windowID) return;
  if (message.to !== windowID && message.to !== "*") return;
  switch (message.type) {
    case "active":
      activeWindow = message.from;
      break;
    case "closed":
      if (activeWindow === message.from) activeWindow = windowID;
      break;
    case "key":
      moveFromWindow(message.key);
      break;
    case "dock":
      takeTab(message.media, message.from, message.tab);
      window.focus();
      break;
    case "tab-moved":
      clearTimeout(dockTimer);
      removeTab(message.tab);
      break;
    case "follow":
      followActiveTab(
        message.table,
        message.row,
        (column) => message.files[column],
      );
      break;
    case "toggle":
      toggleActivePlayer();
      break;
    case "pause":
      pauseAllPlayers();
      break;
  }
}

// noteActiveWindow records that row moves should come here.
function noteActiveWindow() {
  if (homeWindow) postToWindow(homeWindow, "active");
  else activeWindow = windowID;
}

// followsElsewhere says whether row moves belong to another window.
function followsElsewhere() {
  return !homeWindow && activeWindow !== windowID;
}

function removeTab(tabID) {
  const pane = allPanes().find((pane) =>
    pane.tabs.some((tab) => tab.id === tabID),
  );
  if (pane) closeTab(pane, tabID);
}

// takeTab opens a file another window gave up, and tells that window it can let go.
function takeTab(media, from, tabID) {
  openMedia(media);
  noteActiveWindow();
  showPanes();
  postToWindow(from, "tab-moved", { tab: tabID });
}

// droppedOutsideWindow says whether a tab's drag ended on nothing, past the edge
// of this window.
function droppedOutsideWindow(event) {
  if (event.dataTransfer && event.dataTransfer.dropEffect !== "none")
    return false;
  return (
    event.clientX < 0 ||
    event.clientY < 0 ||
    event.clientX >= window.innerWidth ||
    event.clientY >= window.innerHeight
  );
}

// popOutTab moves a tab to a window of its own, at a place on the screen if given.
// A blocked window leaves the tab where it is, and says so.
function popOutTab(pane, tab, screenX, screenY) {
  const player = playerOfView(tabViews.get(tab.id));
  const media = Object.assign({}, tab.media, {
    time: player ? player.currentTime : undefined,
  });
  const home = homeWindow || windowID;
  const name = "rotari-window-" + randomWindowPart();
  const place =
    screenX === undefined ? "" : ",left=" + screenX + ",top=" + screenY;
  const fragment = new URLSearchParams({
    popout: JSON.stringify(media),
    home,
  });
  const opened = window.open(
    location.pathname + location.search + "#" + fragment,
    name,
    "popup,width=900,height=600" + place,
  );
  if (!opened) {
    showError(
      "The browser blocked a new window; allow pop-ups for this page to move a file out.",
    );
    return;
  }
  // The new window reports to the home itself once it has loaded, but a row move
  // may come first.
  if (!homeWindow) activeWindow = name;
  closeTab(pane, tab.id);
}

// dockTab gives a tab back to the window with the table.
function dockTab(tab) {
  postToWindow(homeWindow, "dock", { media: tab.media, tab: tab.id });
  clearTimeout(dockTimer);
  dockTimer = setTimeout(
    () =>
      showError(
        "The table's window did not answer; it may be closed. This file stays here.",
      ),
    dockTimeoutMs,
  );
}

// foreignTabDrag says whether a drag holds a tab from another window.
function foreignTabDrag(event) {
  return (
    !dragging &&
    event.dataTransfer &&
    [...event.dataTransfer.types].includes(tabDragType)
  );
}

function readTabDrag(event) {
  try {
    const moved = JSON.parse(event.dataTransfer.getData(tabDragType));
    return moved.window === windowID ? null : moved;
  } catch {
    return null;
  }
}

// receiveTab places a tab dragged from another window in a pane at index, or, for
// an edge zone, in a new pane split off that side.
function receiveTab(event, targetID, zone, index) {
  clearDropMarks();
  const moved = readTabDrag(event);
  let target = findPane(targetID);
  if (!moved || !target) return;
  if (zone !== "center") {
    const created = newPane();
    created.wave = target.wave;
    created.spec = target.spec;
    created.waveHeight = target.waveHeight;
    const dir = zone === "left" || zone === "right" ? "row" : "column";
    insertBeside(target, created, dir, zone === "left" || zone === "top");
    target = created;
    index = 0;
  }
  const tab = newTab(moved.media);
  target.tabs.splice(Math.min(index, target.tabs.length), 0, tab);
  target.range = null;
  target.current = tab.id;
  panes.active = target.id;
  noteActiveWindow();
  renderPanes();
  postToWindow(moved.window, "tab-moved", { tab: moved.tab });
}

// A tab from another window dropped anywhere else opens in the active pane, so one
// can come back to a window with no panes open.
document.addEventListener("dragover", (event) => {
  if (event.defaultPrevented || !foreignTabDrag(event)) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = "move";
});
document.addEventListener("drop", (event) => {
  if (event.defaultPrevented || !foreignTabDrag(event)) return;
  event.preventDefault();
  const moved = readTabDrag(event);
  if (moved) takeTab(moved.media, moved.window, moved.tab);
});

// startPopOut shows only the file a window of its own was opened with.
function startPopOut(text, home) {
  document.body.classList.add("tv-popout");
  homeWindow = home;
  window.addEventListener("pagehide", () => postToWindow(homeWindow, "closed"));
  let media;
  try {
    media = JSON.parse(text);
  } catch {
    showError("This window's file could not be read from its address.");
    return;
  }
  document.title = media.ref + " · rotari";
  openMedia(media);
  noteActiveWindow();
  const player = activePlayer();
  if (player && media.time)
    player.addEventListener(
      "loadedmetadata",
      () => (player.currentTime = media.time),
      { once: true },
    );
}

// popOutKey sends the arrow keys to the table, which moves and tells this window
// what to show. Controls keep the keys they use.
function popOutKey(event) {
  if (!event.key.startsWith("Arrow")) return false;
  if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey)
    return false;
  const target = event.target;
  if (
    target.closest &&
    target.closest("input, select, textarea, audio, video, [contenteditable]")
  )
    return false;
  event.preventDefault();
  noteActiveWindow();
  postToWindow(homeWindow, "key", { key: event.key });
  return true;
}
