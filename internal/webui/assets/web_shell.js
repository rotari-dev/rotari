// The page frame shared by the application and Recent jobs pages, inlined
// in <head> of both: the viewer's theme, the Settings menu, the navigation
// drawer on narrow screens, the sidebar's width, and why notifications are
// unavailable.
//
// The theme is "light", "dark", or "system" to follow the OS. It is kept per
// browser and applied before the page is drawn, so a stored choice never
// flashes the other theme. web_tokens.css reads it from data-theme on <html>.
(() => {
  const key = "rotari-theme";
  const choices = ["system", "light", "dark"];
  const stored = () => {
    try {
      const value = localStorage.getItem(key);
      return choices.includes(value) ? value : "system";
    } catch {
      return "system";
    }
  };
  const apply = (choice) => {
    if (choice === "light" || choice === "dark") {
      document.documentElement.dataset.theme = choice;
    } else {
      delete document.documentElement.dataset.theme;
    }
  };
  apply(stored());

  // The Settings menu opens from its button and closes on Escape or a click
  // outside it.
  const setMenuOpen = (open) => {
    const button = document.getElementById("settings-button");
    const panel = document.getElementById("settings-panel");
    if (!button || !panel) return;
    panel.hidden = !open;
    button.setAttribute("aria-expanded", String(open));
  };
  window.toggleSettingsMenu = () =>
    setMenuOpen(document.getElementById("settings-panel")?.hidden ?? false);
  window.closeSettingsMenu = () => setMenuOpen(false);

  // The inbox shows the notifications runs recorded for the selected
  // basedirs, read from /api/notifications on every 2-second poll. Each
  // basedir has its own read and cleared cursors, shared through the server,
  // so reading or clearing moves only those of the basedirs shown; on a
  // read-only server they stay in page memory. Entries seen for the first time after the
  // first poll of a selection are announced and passed to options.desktop;
  // history, including that of a newly selected basedir, is not.
  const zeroCursor = { created_at: "", id: "" };
  const after = (a, b) =>
    a.created_at !== b.created_at ? a.created_at > b.created_at : a.id > b.id;
  const later = (a, b) => (after(a, b) ? a : b);
  let inbox = [];
  let inboxProblems = [];
  let inboxError = "";
  const zeroCursors = { read: zeroCursor, cleared: zeroCursor };
  // Cursors by basedir ID: the server's and the page's own.
  let serverState = {};
  const localState = {};
  const seen = new Set();
  let selectionKey = null;
  let inboxPanel;
  let inboxButton;
  let inboxOptions;
  const cursorsOf = (id) => {
    const server = serverState[id] || zeroCursors;
    const local = localState[id] || zeroCursors;
    const cleared = later(server.cleared, local.cleared);
    return { cleared, read: later(later(server.read, local.read), cleared) };
  };
  // newestByBasedir returns each basedir's newest entry position; entries
  // come newest first.
  const newestByBasedir = (entries) => {
    const newest = {};
    for (const item of entries)
      newest[item.basedir] ??= { created_at: item.created_at, id: item.id };
    return newest;
  };
  // saveInboxState advances the shared cursors of the basedirs in changes,
  // or the page's own when the server cannot save them.
  const saveInboxState = async (changes) => {
    for (const [id, next] of Object.entries(changes)) {
      const local = localState[id] || zeroCursors;
      localState[id] = {
        read: later(local.read, next.read),
        cleared: later(local.cleared, next.cleared),
      };
    }
    try {
      const response = await fetch("/api/notifications/state", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ basedirs: changes }),
      });
      if (response.ok)
        Object.assign(serverState, (await response.json()).basedirs);
    } catch (error) {
      // The page keeps its own cursors until the server is reachable.
    }
  };
  window.initializeNotificationInbox = (options) => {
    const toggle = document.getElementById("notify-toggle");
    if (!toggle || inboxButton) return;
    inboxOptions = options;
    const container = document.createElement("div");
    container.className = "notification-inbox";
    inboxButton = document.createElement("button");
    inboxButton.type = "button";
    inboxButton.textContent = "Inbox (0)";
    inboxButton.setAttribute("aria-expanded", "false");
    inboxButton.setAttribute("aria-controls", "notification-inbox-panel");
    inboxPanel = document.createElement("section");
    inboxPanel.id = "notification-inbox-panel";
    inboxPanel.hidden = true;
    inboxPanel.setAttribute("aria-label", "Notifications");
    const clear = document.createElement("button");
    clear.type = "button";
    clear.textContent = "Clear notifications";
    const list = document.createElement("div");
    list.className = "notification-inbox-list";
    const announcement = document.createElement("span");
    announcement.className = "notification-announcement";
    announcement.setAttribute("role", "status");
    container.append(inboxButton, inboxPanel, announcement);
    inboxPanel.append(clear, list);
    toggle.before(container);
    const visible = () =>
      inbox.filter((item) => after(item, cursorsOf(item.basedir).cleared));
    const markRead = () => {
      const changes = {};
      for (const [id, newest] of Object.entries(newestByBasedir(visible())))
        if (after(newest, cursorsOf(id).read))
          changes[id] = { read: newest, cleared: cursorsOf(id).cleared };
      if (Object.keys(changes).length) saveInboxState(changes);
    };
    const render = () => {
      const entries = visible();
      const unread = entries.filter((item) =>
        after(item, cursorsOf(item.basedir).read),
      ).length;
      inboxButton.textContent = "Inbox (" + unread + ")";
      inboxButton.classList.toggle("notifications-on", unread > 0);
      list.replaceChildren();
      for (const message of [inboxError, ...inboxProblems]) {
        if (!message) continue;
        const note = document.createElement("p");
        note.className = "notification-inbox-error";
        note.textContent = message;
        list.append(note);
      }
      if (!entries.length) list.append("No notifications yet.");
      for (const item of entries) {
        const entry = document.createElement("article");
        const title = document.createElement(item.url ? "a" : "strong");
        title.textContent = item.title;
        if (item.url) title.href = item.url;
        const body = document.createElement("p");
        body.textContent = item.body;
        entry.append(title, body);
        list.append(entry);
      }
    };
    const load = async () => {
      const ids = [...new Set(inboxOptions.basedirIDs())].sort();
      const key = ids.join(",");
      const baseline = key !== selectionKey;
      let data = { entries: [], basedirs: {} };
      if (ids.length) {
        let response;
        try {
          response = await fetch(
            "/api/notifications?" + new URLSearchParams({ basedirs: key }),
            { cache: "no-store" },
          );
        } catch (error) {
          return; // Keep polling after temporary network failures.
        }
        if (!response.ok) {
          inboxError =
            "Cannot load notifications: " + (await response.text()).trim();
          render();
          return;
        }
        data = await response.json();
        if (!Array.isArray(data.entries) || !data.basedirs)
          throw new Error("unexpected response from /api/notifications");
      }
      inboxError = "";
      selectionKey = key;
      serverState = data.basedirs;
      inboxProblems = (data.problems || []).map(
        (problem) => "Unreadable notification record: " + problem,
      );
      const arrived = data.entries.filter((item) => !seen.has(item.id));
      data.entries.forEach((item) => seen.add(item.id));
      inbox = data.entries;
      const announced = baseline
        ? []
        : arrived.filter((item) =>
            after(item, cursorsOf(item.basedir).cleared),
          );
      if (announced.length) {
        announcement.textContent = announced[0].title;
        announced
          .slice()
          .reverse()
          .forEach((item) => inboxOptions.desktop?.(item));
        if (!inboxPanel.hidden) markRead();
      }
      render();
    };
    inboxButton.onclick = () => {
      inboxPanel.hidden = !inboxPanel.hidden;
      inboxButton.setAttribute("aria-expanded", String(!inboxPanel.hidden));
      if (!inboxPanel.hidden) markRead();
      render();
    };
    clear.onclick = () => {
      const changes = {};
      for (const [id, newest] of Object.entries(newestByBasedir(visible())))
        changes[id] = { read: newest, cleared: newest };
      if (Object.keys(changes).length) saveInboxState(changes);
      render();
    };
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && !inboxPanel.hidden) {
        inboxPanel.hidden = true;
        inboxButton.setAttribute("aria-expanded", "false");
        inboxButton.focus();
      }
    });
    const poll = () =>
      load().catch((error) => {
        inboxError = "Cannot load notifications: " + error;
        render();
      });
    // A changed basedir selection is read at once; its history does not
    // announce.
    window.refreshNotificationInbox = poll;
    render();
    poll();
    setInterval(poll, 2000);
  };

  // Below 760px the sidebar is a drawer over the page.
  const setDrawerOpen = (open) => {
    document.body.classList.toggle("drawer-open", open);
    document
      .querySelectorAll(".drawer-toggle")
      .forEach((button) => button.setAttribute("aria-expanded", String(open)));
  };
  window.toggleDrawer = () =>
    setDrawerOpen(!document.body.classList.contains("drawer-open"));
  window.closeDrawer = () => setDrawerOpen(false);

  // The reason the notification toggle cannot be used, or "" when it can.
  // Browsers allow notifications only in a secure context, so a page opened
  // over plain HTTP from a non-loopback host cannot enable them.
  window.notificationUnavailableReason = () => {
    if (window.isSecureContext === false)
      return "Browser notifications need HTTPS or a localhost address. Forward the port with ssh -L and open http://localhost:PORT.";
    if (
      typeof Notification !== "undefined" &&
      Notification.permission === "denied"
    )
      return "Notifications are blocked for this site in the browser's settings.";
    return "";
  };

  const widthKey = "rotari-sidebar-width";
  const clampWidth = (width) => Math.max(190, Math.min(520, width));
  const saveWidth = (width) => {
    try {
      localStorage.setItem(widthKey, String(width));
    } catch {
      // The width then lasts for this page.
    }
  };
  function initSidebarResizer() {
    const sidebar = document.querySelector(".sidebar");
    const handle = document.querySelector(".sidebar-resizer");
    if (!sidebar || !handle) return;
    const applySavedWidth = () => {
      if (window.innerWidth <= 760) {
        sidebar.style.width = "";
        return;
      }
      let saved = 0;
      try {
        saved = Number(localStorage.getItem(widthKey));
      } catch {
        saved = 0;
      }
      if (Number.isFinite(saved) && saved > 0)
        sidebar.style.width = clampWidth(saved) + "px";
    };
    applySavedWidth();
    window.addEventListener("resize", applySavedWidth);
    handle.addEventListener("pointerdown", (event) => {
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = sidebar.getBoundingClientRect().width;
      handle.classList.add("dragging");
      const move = (moveEvent) => {
        const width = clampWidth(startWidth + moveEvent.clientX - startX);
        sidebar.style.width = width + "px";
        saveWidth(width);
      };
      const end = () => {
        handle.classList.remove("dragging");
        window.removeEventListener("pointermove", move);
        window.removeEventListener("pointerup", end);
      };
      window.addEventListener("pointermove", move);
      window.addEventListener("pointerup", end);
    });
    handle.addEventListener("keydown", (event) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      const direction = event.key === "ArrowRight" ? 1 : -1;
      const width = clampWidth(
        sidebar.getBoundingClientRect().width + direction * 12,
      );
      sidebar.style.width = width + "px";
      saveWidth(width);
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    initSidebarResizer();
    document.addEventListener("click", (event) => {
      const menu = document.querySelector(".settings-menu");
      if (menu && !menu.contains(event.target)) setMenuOpen(false);
      // Following a link in the drawer closes it.
      if (event.target.closest && event.target.closest(".sidebar a"))
        setDrawerOpen(false);
    });
    document.addEventListener("keydown", (event) => {
      if (event.key !== "Escape") return;
      setMenuOpen(false);
      setDrawerOpen(false);
    });
    const select = document.getElementById("theme-choice");
    if (!select) return;
    select.value = stored();
    select.addEventListener("change", () => {
      try {
        localStorage.setItem(key, select.value);
      } catch {
        // Storage can be unavailable; the choice then lasts for this page.
      }
      apply(select.value);
    });
    // A choice made in another tab applies here too.
    window.addEventListener("storage", (event) => {
      if (event.key !== key) return;
      select.value = stored();
      apply(select.value);
    });
  });
})();
