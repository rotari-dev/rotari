// Sticky notes: free-form reminders pinned to the Web UI page they were
// written on, kept by the server in the basedir (/api/stickies).
const stickyColors = ["yellow", "pink", "green", "blue", "orange"];
const stickiesHiddenKey = "rotari-stickies-hidden-pages";
const stickySaveDelayMs = 600;
let stickyPage = "";
/** @type {{id: string, text: string, color: string, x: number, y: number}[]} */
let stickyNotes = [];
let stickySaveTimer = 0;
let stickyDragging = false;
function stickyPageKey() {
  return pageParts().join("/") || "/";
}
/** The pages whose notes this browser has hidden. */
function hiddenStickyPages() {
  try {
    const stored = JSON.parse(localStorage.getItem(stickiesHiddenKey) || "[]");
    return Array.isArray(stored) ? stored : [];
  } catch (error) {
    return [];
  }
}
function stickiesHidden() {
  return hiddenStickyPages().includes(stickyPage);
}
function setStickiesHidden(hidden) {
  const pages = hiddenStickyPages().filter((page) => page !== stickyPage);
  if (hidden) pages.push(stickyPage);
  localStorage.setItem(stickiesHiddenKey, JSON.stringify(pages));
}
function stickyLayer() {
  let layer = document.getElementById("sticky-layer");
  if (!layer) {
    layer = document.createElement("div");
    layer.id = "sticky-layer";
    document.body.appendChild(layer);
  }
  return layer;
}
function newStickyID() {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}
const stickyDefaultWidth = 220;
const stickyDefaultHeight = 160;
const stickyMinWidth = 120;
const stickyMinHeight = 80;
const stickyMaxSize = 2000;
function clampSticky(note) {
  note.x = Math.max(0, Math.min(note.x, window.innerWidth - 60));
  note.y = Math.max(0, Math.min(note.y, window.innerHeight - 40));
}
function stickyErrorOn(id, message) {
  const element = document.querySelector(
    '.sticky-note[data-sticky-id="' + id + '"]',
  );
  if (!element) return;
  element.classList.toggle("sticky-error", message !== "");
  element.title = message;
}
async function saveStickies(page, notes) {
  try {
    const response = await fetch("/api/stickies", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ page, notes }),
    });
    const text = await response.text();
    const message = response.ok ? "" : "Not saved: " + text.trim();
    if (page === stickyPage)
      for (const note of notes) stickyErrorOn(note.id, message);
    return message;
  } catch (error) {
    const message = "Not saved: " + error;
    if (page === stickyPage)
      for (const note of notes) stickyErrorOn(note.id, message);
    return message;
  }
}
function flushStickySave() {
  if (!stickySaveTimer) return;
  clearTimeout(stickySaveTimer);
  stickySaveTimer = 0;
  saveStickies(stickyPage, stickyNotes.map((note) => ({ ...note })));
}
function scheduleStickySave() {
  clearTimeout(stickySaveTimer);
  stickySaveTimer = setTimeout(flushStickySave, stickySaveDelayMs);
}
function renderStickies() {
  const layer = stickyLayer();
  layer.hidden = stickiesHidden();
  const focused = document.activeElement;
  const focusedID = focused?.closest?.(".sticky-note")?.dataset.stickyId;
  layer.textContent = "";
  for (const note of stickyNotes) {
    clampSticky(note);
    const element = document.createElement("div");
    element.className = "sticky-note sticky-" + note.color;
    element.dataset.stickyId = note.id;
    element.style.left = note.x + "px";
    element.style.top = note.y + "px";
    element.style.width = (note.width || stickyDefaultWidth) + "px";
    element.style.height = (note.height || stickyDefaultHeight) + "px";
    const head = document.createElement("div");
    head.className = "sticky-head";
    const color = document.createElement("button");
    color.type = "button";
    color.className = "sticky-color";
    color.title = "Change color";
    color.setAttribute("aria-label", "Change sticky note color");
    color.addEventListener("click", () => {
      note.color =
        stickyColors[(stickyColors.indexOf(note.color) + 1) % stickyColors.length];
      element.className = "sticky-note sticky-" + note.color;
      scheduleStickySave();
    });
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "sticky-remove";
    remove.title = "Delete sticky note";
    remove.setAttribute("aria-label", "Delete sticky note");
    remove.textContent = "×";
    remove.addEventListener("click", () => {
      stickyNotes = stickyNotes.filter((item) => item.id !== note.id);
      element.remove();
      removeStickyRow(note.id);
      scheduleStickySave();
      updateStickiesToggle();
    });
    head.append(color, remove);
    const body = document.createElement("textarea");
    body.className = "sticky-text";
    body.value = note.text;
    body.maxLength = 4000;
    body.setAttribute("aria-label", "Sticky note");
    body.addEventListener("input", () => {
      note.text = body.value;
      scheduleStickySave();
    });
    head.addEventListener("pointerdown", (event) => {
      if (event.target !== head || event.button !== 0) return;
      event.preventDefault();
      stickyDragging = true;
      head.setPointerCapture(event.pointerId);
      const startX = event.clientX - note.x;
      const startY = event.clientY - note.y;
      const move = (moveEvent) => {
        note.x = moveEvent.clientX - startX;
        note.y = moveEvent.clientY - startY;
        clampSticky(note);
        element.style.left = note.x + "px";
        element.style.top = note.y + "px";
      };
      const end = () => {
        head.removeEventListener("pointermove", move);
        head.removeEventListener("pointerup", end);
        head.removeEventListener("pointercancel", end);
        stickyDragging = false;
        scheduleStickySave();
      };
      head.addEventListener("pointermove", move);
      head.addEventListener("pointerup", end);
      head.addEventListener("pointercancel", end);
    });
    const grip = document.createElement("div");
    grip.className = "sticky-grip";
    grip.title = "Resize";
    grip.addEventListener("pointerdown", (event) => {
      if (event.button !== 0) return;
      event.preventDefault();
      const startWidth = element.offsetWidth;
      const startHeight = element.offsetHeight;
      const startX = event.clientX;
      const startY = event.clientY;
      const move = (moveEvent) => {
        note.width = Math.round(
          Math.max(
            stickyMinWidth,
            Math.min(stickyMaxSize, startWidth + moveEvent.clientX - startX),
          ),
        );
        note.height = Math.round(
          Math.max(
            stickyMinHeight,
            Math.min(stickyMaxSize, startHeight + moveEvent.clientY - startY),
          ),
        );
        element.style.width = note.width + "px";
        element.style.height = note.height + "px";
      };
      // Listening on the window keeps the drag going when the pointer leaves
      // the grip or the note.
      const end = () => {
        window.removeEventListener("pointermove", move);
        window.removeEventListener("pointerup", end);
        window.removeEventListener("pointercancel", end);
        scheduleStickySave();
      };
      window.addEventListener("pointermove", move);
      window.addEventListener("pointerup", end);
      window.addEventListener("pointercancel", end);
    });
    element.append(head, body, grip);
    layer.appendChild(element);
    if (note.id === focusedID) body.focus();
  }
  updateStickiesToggle();
}
function updateStickiesToggle() {
  const toggle = document.getElementById("stickies-toggle");
  if (!toggle) return;
  toggle.disabled = stickyNotes.length === 0;
  toggle.textContent = stickiesHidden()
    ? "Show on this page"
    : "Hide on this page";
}
async function loadStickies() {
  const page = stickyPageKey();
  if (page === stickyPage) return;
  flushStickySave();
  stickyPage = page;
  stickyNotes = [];
  renderStickies();
  const data = await fetchWebJSON("/api/stickies?page=" + encodeURIComponent(page));
  if (page !== stickyPage || !data) return;
  stickyNotes = data.notes || [];
  renderStickies();
}
function addSticky() {
  setStickiesHidden(false);
  const offset = (stickyNotes.length % 8) * 24;
  stickyNotes.push({
    id: newStickyID(),
    text: "",
    color: stickyColors[stickyNotes.length % stickyColors.length],
    width: 0,
    height: 0,
    created_at: new Date().toISOString(),
    x: Math.max(16, window.innerWidth - 260) - offset,
    y: 80 + offset,
  });
  renderStickies();
  scheduleStickySave();
  document
    .querySelector(
      '.sticky-note[data-sticky-id="' + stickyNotes.at(-1).id + '"] textarea',
    )
    ?.focus();
}
function toggleStickies() {
  setStickiesHidden(!stickiesHidden());
  renderStickies();
}
/** The first characters of a note, on one line, for the list of all notes. */
function stickySnippet(text) {
  const line = text.replace(/\s+/g, " ").trim();
  if (!line) return "(empty)";
  return line.length > 24 ? line.slice(0, 24) + "…" : line;
}
async function renderStickyList() {
  const list = document.getElementById("stickies-list");
  if (!list) return;
  flushStickySave();
  const data = await fetchWebJSON("/api/stickies");
  list.textContent = "";
  const entries = [];
  for (const entry of data?.pages || [])
    for (const note of entry.notes || [])
      entries.push({ page: entry.page, note });
  entries.sort((a, b) =>
    (b.note.created_at || "").localeCompare(a.note.created_at || ""),
  );
  if (!entries.length) {
    const empty = document.createElement("div");
    empty.className = "stickies-empty";
    empty.textContent = data ? "No sticky notes yet." : "Could not load notes.";
    list.appendChild(empty);
    return;
  }
  for (const { page, note } of entries) {
    const row = document.createElement("div");
    row.className = "stickies-row";
    row.dataset.stickyId = note.id;
    const link = document.createElement("a");
    link.className = "stickies-item";
    link.href = appURL(page === "/" ? "/" : "/" + page);
    const when = document.createElement("span");
    when.className = "stickies-item-time";
    when.textContent = note.created_at ? formatDateTime(note.created_at) : "";
    const text = document.createElement("span");
    text.className = "stickies-item-text";
    text.textContent = stickySnippet(note.text);
    const where = document.createElement("span");
    where.className = "stickies-item-page";
    where.textContent = page;
    link.append(when, text, where);
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "stickies-item-remove";
    remove.title = "Delete sticky note";
    remove.setAttribute("aria-label", "Delete sticky note");
    remove.textContent = "×";
    remove.addEventListener("click", () => deleteStickyFromList(page, note.id));
    row.append(link, remove);
    list.appendChild(row);
  }
}
// Drops a note's row from the open list at once, without waiting for a save.
function removeStickyRow(id) {
  const list = document.getElementById("stickies-list");
  if (!list) return;
  for (const row of list.querySelectorAll(".stickies-row"))
    if (row.dataset.stickyId === id) row.remove();
  if (!list.querySelector(".stickies-row")) {
    const empty = document.createElement("div");
    empty.className = "stickies-empty";
    empty.textContent = "No sticky notes yet.";
    list.appendChild(empty);
  }
}
async function deleteStickyFromList(page, id) {
  removeStickyRow(id);
  if (page === stickyPage) {
    stickyNotes = stickyNotes.filter((item) => item.id !== id);
    renderStickies();
    scheduleStickySave();
    flushStickySave();
  } else {
    const data = await fetchWebJSON(
      "/api/stickies?page=" + encodeURIComponent(page),
    );
    if (!data) return;
    const message = await saveStickies(
      page,
      (data.notes || []).filter((item) => item.id !== id),
    );
    if (message) {
      alert(message);
      renderStickyList();
    }
  }
}
function setStickiesMenuOpen(open) {
  const panel = document.getElementById("stickies-panel");
  const button = document.getElementById("stickies-button");
  if (!panel || !button) return;
  panel.hidden = !open;
  button.setAttribute("aria-expanded", String(open));
  if (open) renderStickyList();
}
// A static export has no server to keep notes.
if (typeof rewriteStaticLinks !== "function") {
  const menu = document.getElementById("stickies-menu");
  if (menu) menu.hidden = false;
  document.getElementById("stickies-button")?.addEventListener("click", () => {
    setStickiesMenuOpen(document.getElementById("stickies-panel").hidden);
  });
  document.getElementById("stickies-add")?.addEventListener("click", () => {
    addSticky();
    setStickiesMenuOpen(false);
  });
  document.getElementById("stickies-toggle")?.addEventListener("click", () => {
    toggleStickies();
    setStickiesMenuOpen(false);
  });
  document.addEventListener("click", (event) => {
    // The path is taken when the click is dispatched, so it still names the
    // menu after the click removed the row that was clicked.
    if (!event.composedPath().includes(document.getElementById("stickies-menu")))
      setStickiesMenuOpen(false);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") setStickiesMenuOpen(false);
  });
  window.addEventListener("pagehide", flushStickySave);
  loadStickies();
  setInterval(loadStickies, 1000);
}
