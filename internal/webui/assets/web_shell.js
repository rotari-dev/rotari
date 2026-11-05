// The controls of the application and Recent jobs pages name an action in
// data-action (click), data-change-action, or data-toggle-action, with any
// arguments as JSON in data-args. A Content-Security-Policy forbids inline
// handlers, and only these actions can run, so a control cannot call anything
// else on the page.
(() => {
  const actions = {
    cancelRun: (el) => cancelRun(el.dataset.project, el.dataset.run),
    changeLogStream: (el) => changeLogStream(el.value),
    chooseArtifactLaunch: (el, args) => chooseArtifactLaunch(...args, el),
    clearSelectedJobs: () => clearSelectedJobs(),
    closeArtifactLaunch: () => closeArtifactLaunch(),
    closeDrawer: () => closeDrawer(),
    closeOutputModal: () => closeOutputModal(),
    controlSelectedRunJobs: (_, args) => controlSelectedRunJobs(...args),
    copyCommandGuide: (el) => copyCommandGuide(el.closest("pre"), el),
    sendToTerminal: (el) => sendButtonPressed(el),
    sendModalOutput: () => sendModalOutput(),
    sendLogTail: () => sendLogTail(),
    copyIdentityValue: (el) => copyIdentityValue(el),
    copyJobsValue: (el) => copyJobsValue(el),
    copyLogTail: (el) => copyLogTail(el),
    copyModalOutput: (el) => copyModalOutput(el),
    copySelectedJobs: (el, args) => copySelectedJobs(el.dataset.project, el.dataset.run, ...args),
    deleteProject: (el) => deleteProject(el.dataset.project),
    deleteRun: (el) => deleteRun(el.dataset.project, el.dataset.run),
    focusMatrixJob: (_, args) => focusMatrixJob(...args),
    generateLauncherConfig: () => generateLauncherConfig(),
    historySearchAddCondition: () => historySearchAddCondition(),
    historySearchAddScope: () => historySearchAddScope(),
    historySearchBasedirChanged: (el) => historySearchBasedirChanged(el),
    historySearchPage: (_, args) => historySearchPage(...args),
    historySearchProjectChanged: (el) => historySearchProjectChanged(el),
    historySearchRemoveCondition: (el) => historySearchRemoveCondition(el),
    historySearchRemoveScope: (el) => historySearchRemoveScope(el),
    historySearchToggleCustomRange: (el) => historySearchToggleCustomRange(el),
    historySearchUpdateTarget: (el) => historySearchUpdateTarget(el),
    historySearchUpdateValueControl: (el) => historySearchUpdateValueControl(el),
    loadArtifactDirectory: (_, args) => loadArtifactDirectory(...args),
    loadArtifactText: (_, args) => loadArtifactText(...args),
    log: (el) => log(el.dataset.project, el.dataset.logRun, el.dataset.logJob, el.dataset.attempt, el.dataset.logMode),
    moreArtifactTable: () => moreArtifactTable(),
    moreArtifactText: () => moreArtifactText(),
    openArtifact: (_, args) => openArtifact(...args),
    openMatrixCell: (el) => openMatrixCell(el),
    popupMenuToggled: (el) => popupMenuToggled(el),
    refreshWithBasedirs: () => refreshWithBasedirs(),
    removeQueueJob: (el) => removeQueueJob(el.dataset.project, el.dataset.job, el.dataset.label),
    saveQueueJob: (el) => saveQueueJob(el.dataset.project, el.dataset.job, el.closest("tr")),
    selectAllRunJobs: () => selectAllRunJobs(),
    selectFailedJobs: () => selectFailedJobs(),
    selectFailedUnfinishedJobs: () => selectFailedUnfinishedJobs(),
    selectJobAttempt: (_, args) => selectJobAttempt(...args),
    setMatrixAxis: (el) => setMatrixAxis(el),
    setTableColumn: (el) => setTableColumn(el),
    showArtifacts: (_, args) => showArtifacts(...args),
    showConfig: (_, args) => showConfig(...args),
    showDiagnosis: (el) => showDiagnosis(el),
    showJobReport: (el) => showJobReport(el),
    showLaunchLog: (_, args) => showLaunchLog(...args),
    showLaunches: (_, args) => showLaunches(...args),
    showLog: (el) => showLog(el.dataset.project, el.dataset.run, el.dataset.job, el.dataset.attempt, "stdout", el.dataset.logMode),
    showNotes: (el) => showNotes(el),
    showNotificationConfig: () => showNotificationConfig(),
    showOriginalOutput: (el) => showOriginalOutput(el.dataset.run, el.dataset.job, el),
    showPath: (el) => showPath(el.dataset.path),
    showRecoverRun: (el) => showRecoverRun(el.dataset.project, el.dataset.run),
    showRunFiles: (_, args) => showRunFiles(...args),
    showRunReport: (el) => showRunReport(el),
    startArtifactLaunch: (_, args) => startArtifactLaunch(...args),
    stopLaunch: (_, args) => stopLaunch(...args),
    toggleAllJobsNotificationBasedirs: (el) => toggleAllJobsNotificationBasedirs(el),
    toggleAllNotificationBasedirs: (el) => toggleAllNotificationBasedirs(el),
    toggleArtifactMarkdown: () => toggleArtifactMarkdown(),
    toggleDrawer: () => toggleDrawer(),
    toggleJobsNotificationBasedir: (el) => toggleJobsNotificationBasedir(el),
    toggleJobsNotifications: () => toggleJobsNotifications(),
    toggleJobsSidebar: (el) => toggleJobsSidebar(el),
    toggleNotificationBasedir: (el) => toggleNotificationBasedir(el),
    toggleReportRedaction: () => toggleReportRedaction(),
    toggleReportSource: () => toggleReportSource(),
    toggleRunNotifications: () => toggleRunNotifications(),
    toggleSettingsMenu: () => toggleSettingsMenu(),
    toggleTerminalPanel: () => toggleTerminalPanel(),
    openTerminalHere: () => openTerminalHere(),
    toggleSidebarAllProjects: (el) => toggleSidebarAllProjects(el),
    toggleSidebarBasedir: (el) => toggleSidebarBasedir(el),
    toggleSidebarProject: (el) => toggleSidebarProject(el),
    closeOutputModalOnBackdrop: (el, _, event) => {
      if (event.target === el) closeOutputModal();
    },
    reloadPage: () => location.reload(),
    submitNotificationConfig: () =>
      document.getElementById("notification-config-editor").requestSubmit(),
  };
  const run = (element, name, event) => {
    if (!Object.hasOwn(actions, name)) {
      console.error("rotari: unknown page action " + name);
      return;
    }
    actions[name](element, JSON.parse(element.dataset.args || "[]"), event);
  };
  // Clicks run every action from the target outward, as bubbling would.
  document.addEventListener("click", (event) => {
    for (let element = event.target; element; element = element.parentElement) {
      if (element.dataset?.action) run(element, element.dataset.action, event);
    }
  });
  // These are caught on their way down: a toggle event does not bubble.
  for (const [type, key] of [
    ["change", "changeAction"],
    ["toggle", "toggleAction"],
  ]) {
    document.addEventListener(
      type,
      (event) => {
        const name = event.target.dataset?.[key];
        if (name) run(event.target, name, event);
      },
      true,
    );
  }
})();

// The terminal panel along the bottom of the page, with a tab for each shell of the
// page's project, as an editor keeps a window's terminals with its folder. Shells live
// on the server, and every page load lists them, attaches to the tab this browser tab
// last showed for the project, and replays recent output, so following a link ends none
// of them. Only the shown tab is connected; the others carry on with no one reading, and
// replay when shown. Moving between projects with the panel open starts nothing: browsing
// must not leave a shell in every project. Nothing loads until the panel first opens; a
// page without the top bar's Terminal button has no terminal to offer.
(() => {
  const openKey = "rotari-terminal-open";
  const heightKey = "rotari-terminal-height";
  const shownKey = "rotari-terminal-shell";
  const maximizedKey = "rotari-terminal-maximized";
  const fontSizeKey = "rotari-terminal-font-size";
  const defaultFontSize = 13;
  const minFontSize = 8;
  const maxFontSize = 32;
  const shellsPath = "/_terminal/shells";
  // How often a page in view picks up shells started or ended from elsewhere.
  const shellsRefresh = 5000;
  // How often any open page says so, whether its panel is open or in view, which keeps
  // the shells. Browsers slow a background page's timers to about this. It is far under
  // the server's idle limit.
  const aliveInterval = 60000;
  const minHeight = 120;
  // Colours programs ask for by name, and the tokens that hold them.
  const themeTokens = {
    background: "background",
    foreground: "foreground",
    cursor: "cursor",
    selectionBackground: "selection",
    black: "black",
    red: "red",
    green: "green",
    yellow: "yellow",
    blue: "blue",
    magenta: "magenta",
    cyan: "cyan",
    white: "white",
    brightBlack: "bright-black",
    brightRed: "bright-red",
    brightGreen: "bright-green",
    brightYellow: "bright-yellow",
    brightBlue: "bright-blue",
    brightMagenta: "bright-magenta",
    brightCyan: "bright-cyan",
    brightWhite: "bright-white",
  };
  const closeTakenOver = 4001;
  const closeExited = 4002;
  const closeFailed = 4000;
  const closeNoShell = 4005;
  const scriptNonce = document.currentScript?.nonce || "";
  // The terminal belongs to the server, not to a basedir, so its requests skip the
  // application script's wrapper, which puts a mounted basedir's prefix on every path.
  // This script runs first.
  const rootFetch = window.fetch;
  const serverFetch = (...args) => rootFetch.apply(window, args);
  let panel, status, retry, viewport, groups, tabs, add, maximize, term, fit, socket, retryTimer;
  let shells = [];
  let shellLimit = 0;
  let groupLimit = 0;
  let here = { basedir: "", project: "", dir: "" }; // The page's place.
  let tabsDrawn = "";
  let groupsDrawn = "";
  let startAsked = false; // Someone opened the panel: start a shell if there is none.
  let newAsked = false; // Someone asked for a new shell here.
  let pendingSend = null; // A command waiting for this page to connect to a shell.
  let shown = null; // The ID of the shell this page shows.
  let listSerial = 0;
  let retryDelay = 1000;
  // What to do when this tab next becomes visible: a tab nobody is looking at
  // neither connects nor retries. Browsers make connections to a server wait for
  // each other, and old tabs retrying would hold up the tab in use.
  let whenVisible = null;

  const remember = (open) => {
    try {
      localStorage.setItem(openKey, open ? "1" : "0");
    } catch {}
  };
  const remembered = () => {
    try {
      return localStorage.getItem(openKey) === "1";
    } catch {
      return false;
    }
  };
  // The page cannot lose all its room to the panel, nor the panel all of its own.
  const clampHeight = (height) =>
    Math.max(minHeight, Math.min(height, Math.max(minHeight, innerHeight - minHeight)));
  const root = document.documentElement;
  const rememberedMaximized = () => {
    try {
      return localStorage.getItem(maximizedKey) === "1";
    } catch {
      return false;
    }
  };
  const applyMaximized = (maximized) => {
    root.classList.toggle("terminal-maximized", maximized);
    if (!maximize) return;
    maximize.textContent = maximized ? "Restore" : "Maximize";
    maximize.setAttribute("aria-pressed", String(maximized));
  };
  const toggleMaximized = () => {
    const maximized = !root.classList.contains("terminal-maximized");
    applyMaximized(maximized);
    try {
      localStorage.setItem(maximizedKey, maximized ? "1" : "0");
    } catch {}
    term?.focus();
  };
  // A value someone edited or an older version left must not make the shell unreadable.
  const savedFontSize = () => {
    try {
      const size = Number(localStorage.getItem(fontSizeKey));
      if (Number.isInteger(size) && size >= minFontSize && size <= maxFontSize) return size;
    } catch {}
    return defaultFontSize;
  };
  const changeFontSize = (step) => {
    term?.focus();
    if (!term) return;
    const size = term.options.fontSize + step;
    if (size < minFontSize || size > maxFontSize) return;
    term.options.fontSize = size;
    // Fewer or more cells fit now, and the shell hears of it through the resize.
    fit.fit();
    try {
      localStorage.setItem(fontSizeKey, String(size));
    } catch {}
  };
  const fontButton = (text, label, step) => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = text;
    button.title = label;
    button.setAttribute("aria-label", label);
    button.onclick = () => changeFontSize(step);
    return button;
  };
  const applyHeight = (height) => {
    // Only an open panel takes room, which web_shell.css decides from this.
    root.style.setProperty("--terminal-saved-height", clampHeight(height) + "px");
  };
  const savedHeight = () => {
    try {
      return Number(localStorage.getItem(heightKey)) || 0;
    } catch {
      return 0;
    }
  };
  const currentHeight = () => panel?.getBoundingClientRect().height || savedHeight();
  const saveHeight = () => {
    try {
      localStorage.setItem(heightKey, String(Math.round(currentHeight())));
    } catch {}
  };
  const theme = () => {
    const style = getComputedStyle(root);
    const colours = {};
    for (const [name, token] of Object.entries(themeTokens)) {
      const value = style.getPropertyValue("--terminal-" + token).trim();
      if (value) colours[name] = value;
    }
    return colours;
  };
  const applyTheme = () => {
    if (term) term.options.theme = theme();
  };
  const fontFamily = () =>
    getComputedStyle(root).getPropertyValue("--font-mono").trim() || "monospace";
  const buildResizer = () => {
    const resizer = document.createElement("div");
    resizer.className = "terminal-resizer";
    resizer.tabIndex = 0;
    resizer.setAttribute("role", "separator");
    resizer.setAttribute("aria-orientation", "horizontal");
    resizer.setAttribute("aria-label", "Resize the terminal");
    resizer.onpointerdown = (event) => {
      event.preventDefault();
      resizer.setPointerCapture?.(event.pointerId);
      const startY = event.clientY;
      const startHeight = currentHeight();
      resizer.onpointermove = (move) =>
        applyHeight(startHeight + startY - move.clientY);
      resizer.onpointerup = resizer.onpointercancel = () => {
        resizer.onpointermove = resizer.onpointerup = resizer.onpointercancel = null;
        saveHeight();
      };
    };
    resizer.onkeydown = (event) => {
      const step = { ArrowUp: 24, ArrowDown: -24 }[event.key];
      if (!step) return;
      event.preventDefault();
      applyHeight(currentHeight() + step);
      saveHeight();
    };
    return resizer;
  };
  const loadScript = (src) =>
    new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.nonce = scriptNonce;
      script.onload = resolve;
      script.onerror = () => reject(new Error("cannot load " + src));
      document.head.append(script);
    });
  let loaded;
  const loadAssets = () => {
    if (loaded) return loaded;
    const style = document.createElement("link");
    style.rel = "stylesheet";
    style.href = "/_terminal/xterm.css";
    document.head.append(style);
    loaded = (async () => {
      if (!window.Terminal) await loadScript("/_terminal/xterm.js");
      if (!window.FitAddon) await loadScript("/_terminal/xterm-addon-fit.js");
      if (!window.WebLinksAddon) await loadScript("/_terminal/xterm-addon-web-links.js");
    })();
    return loaded;
  };
  const showStatus = (message, action, handler) => {
    status.textContent = message;
    status.hidden = !message;
    retry.textContent = action || "";
    retry.hidden = !action;
    retry.onclick = handler || null;
  };
  const send = (data) => {
    if (socket?.readyState === WebSocket.OPEN) socket.send(data);
  };
  // Each project keeps its own last tab.
  const shownStorageKey = () => shownKey + ":" + here.basedir + ":" + here.project;
  const rememberShown = () => {
    try {
      sessionStorage.setItem(shownStorageKey(), shown);
    } catch {}
  };
  const rememberedShown = () => {
    try {
      return sessionStorage.getItem(shownStorageKey());
    } catch {
      return null;
    }
  };
  // The page's basedir ID, project, and run, as it would ask for them in a query. The
  // parts are still percent-encoded, so they go in as they are.
  const pagePlace = () => {
    const parts = location.pathname.split("/").filter(Boolean);
    const query = [];
    if (parts[0] === "_basedir") {
      query.push("basedir=" + parts[1]);
      parts.splice(0, 2);
    }
    if (parts[0] === "project" && parts[1]) {
      query.push("project=" + parts[1]);
      if (parts[2] === "run" && parts[3]) query.push("run=" + parts[3]);
    }
    return query.join("&");
  };
  const sameGroup = (shell, other) =>
    shell.basedir === other.basedir && shell.project === other.project;
  const shellsHere = () => shells.filter((shell) => sameGroup(shell, here));
  const projectName = (group) => group.project || "No project";
  const refreshHere = async () => {
    const response = await serverFetch("/_terminal/here?" + pagePlace());
    if (!response.ok) return response;
    here = await response.json();
    if (add) add.title = "New shell in " + here.dir;
    return null;
  };
  // The projects with shells, this page's first. Choosing one goes to its page, so that
  // what the panel shows always follows the page.
  const renderGroups = () => {
    const entries = [{ ...here, link: "", count: shellsHere().length }];
    for (const shell of shells) {
      if (sameGroup(shell, here)) continue;
      const entry = entries.find((item) => sameGroup(item, shell));
      if (entry) entry.count++;
      else entries.push({ basedir: shell.basedir, project: shell.project, link: shell.link, count: 1 });
    }
    const label = (entry) => {
      const twin = entries.some((item) => item !== entry && item.project === entry.project);
      return projectName(entry) + (twin && entry.basedir ? " in " + entry.basedir : "") + " (" + entry.count + ")";
    };
    const drawn = JSON.stringify(entries.map((entry) => [label(entry), entry.link]));
    if (drawn === groupsDrawn) return;
    groupsDrawn = drawn;
    groups.replaceChildren(
      ...entries.map((entry) => {
        const option = document.createElement("option");
        option.value = entry.link;
        option.textContent = label(entry);
        return option;
      }),
    );
    groups.value = "";
  };
  // endShells ends several shells after asking once. The shown one goes last, so that
  // moving on from it does not land on one about to end.
  const endShells = async (targets) => {
    if (!targets.length) return;
    const noun = targets.length === 1 ? "shell" : targets.length + " shells";
    const running = targets.length === 1 ? "it" : "them";
    if (!confirm("End " + noun + ", and anything running in " + running + "?")) return;
    const ordered = [...targets].sort((a, b) => (a.id === shown) - (b.id === shown));
    for (const shell of ordered) {
      if (shell.id === shown) await refreshShells();
      let response;
      try {
        response = await serverFetch(shellsPath + "/" + encodeURIComponent(shell.id), { method: "DELETE" });
      } catch {}
      if (!response?.ok && response?.status !== 404) {
        await refusal(response, "The shell did not end", () => endShells(targets));
        return;
      }
    }
    if (!targets.some((shell) => shell.id === shown)) await refreshShells();
  };
  const renameShell = async (shell) => {
    const name = prompt("Name for shell " + shell.number + " (empty for its number):", shell.name || "");
    if (name === null) return;
    let response;
    try {
      response = await serverFetch(shellsPath + "/" + encodeURIComponent(shell.id), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
    } catch {}
    if (!response?.ok) {
      await refusal(response, "The tab was not renamed", () => renameShell(shell));
      return;
    }
    await refreshShells();
  };
  let tabMenu = null;
  const closeTabMenu = () => {
    tabMenu?.remove();
    tabMenu = null;
  };
  const openTabMenu = (event, shell) => {
    event.preventDefault();
    closeTabMenu();
    const inGroup = shellsHere().sort((a, b) => a.number - b.number);
    const right = inGroup.slice(inGroup.findIndex((item) => item.id === shell.id) + 1);
    tabMenu = document.createElement("div");
    tabMenu.className = "terminal-tab-menu";
    tabMenu.setAttribute("role", "menu");
    for (const [label, action, enabled] of [
      ["Rename...", () => renameShell(shell), true],
      ["Close tabs to the right", () => endShells(right), right.length > 0],
      ["Close all tabs", () => endShells(inGroup), true],
    ]) {
      const item = document.createElement("button");
      item.type = "button";
      item.setAttribute("role", "menuitem");
      item.textContent = label;
      item.disabled = !enabled;
      item.onclick = () => {
        closeTabMenu();
        action();
      };
      tabMenu.append(item);
    }
    document.body.append(tabMenu);
    // By the pointer, or by the tab for the keyboard's menu key, and inside the window.
    const anchor = event.currentTarget.getBoundingClientRect();
    const size = tabMenu.getBoundingClientRect();
    const x = event.clientX || anchor.left;
    const y = event.clientY || anchor.bottom;
    tabMenu.style.left = Math.max(0, Math.min(x, innerWidth - size.width)) + "px";
    tabMenu.style.top = Math.max(0, Math.min(y, innerHeight - size.height)) + "px";
    tabMenu.querySelector("button").focus();
  };
  document.addEventListener("pointerdown", (event) => {
    if (tabMenu && !tabMenu.contains(event.target)) closeTabMenu();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && tabMenu) {
      closeTabMenu();
      term?.focus();
    }
  });
  const renderTabs = () => {
    renderGroups();
    const inGroup = shellsHere().sort((a, b) => a.number - b.number);
    add.disabled = inGroup.length >= groupLimit || shells.length >= shellLimit;
    const drawn = JSON.stringify([inGroup.map((shell) => [shell.id, shell.number, shell.name, shell.dir]), shown]);
    // Drawing them again would take focus from a tab and move nothing.
    if (drawn === tabsDrawn) return;
    tabsDrawn = drawn;
    tabs.replaceChildren(
      ...inGroup.map((shell) => {
        const tab = document.createElement("span");
        tab.className = "terminal-tab";
        tab.setAttribute("role", "presentation");
        const label = document.createElement("button");
        label.type = "button";
        label.setAttribute("role", "tab");
        label.setAttribute("aria-selected", String(shell.id === shown));
        label.setAttribute("aria-label", "Shell " + shell.number + (shell.name ? ", " + shell.name : ""));
        label.title = "Started in " + shell.dir;
        label.textContent = shell.name || String(shell.number);
        label.onclick = () => {
          if (shell.id !== shown) show(shell.id);
          else term.focus();
        };
        const close = document.createElement("button");
        close.type = "button";
        close.className = "terminal-tab-close";
        close.setAttribute("aria-label", "End shell " + shell.number);
        close.textContent = "\u00d7";
        close.onclick = () => endShell(shell);
        tab.oncontextmenu = (event) => openTabMenu(event, shell);
        tab.append(label, close);
        return tab;
      }),
    );
  };
  // A sign-in that has gone cannot be retried away.
  const signInGone = (response) => {
    if (response?.status !== 401 && response?.status !== 403) return false;
    showStatus("Sign in again to use the terminal.", "Reload", () => location.reload());
    return true;
  };
  const refusal = async (response, what, handler) => {
    if (signInGone(response)) return;
    const reason = response ? (await response.text()).trim() : "the server did not answer";
    showStatus(what + ": " + reason, "Try again", handler);
  };
  // refreshShells updates the tabs, and returns whether it could. A newer change here
  // wins over an older answer, which may not have heard of it.
  const refreshShells = async () => {
    const serial = ++listSerial;
    let response;
    try {
      response = await serverFetch(shellsPath);
    } catch {}
    if (!response?.ok) return { response };
    const listed = await response.json();
    if (serial === listSerial) {
      shells = listed.shells;
      shellLimit = listed.limit;
      groupLimit = listed.group_limit;
      if (panel) renderTabs();
    }
    return { ok: true };
  };
  // connect attaches to the shown shell. Reconnecting by itself passes takeOver false,
  // so it cannot take the shell from a tab in use; only someone opening a page or
  // pressing a button does that.
  const connect = (takeOver = true) => {
    clearTimeout(retryTimer);
    whenVisible = null;
    panel.classList.remove("terminal-empty");
    const previous = socket;
    socket = null;
    previous?.close();
    showStatus("Connecting...");
    // Replay brings the screen back, so what is here would appear twice.
    term.reset();
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    const current = new WebSocket(
      scheme + "//" + location.host + "/_terminal/ws?shell=" + encodeURIComponent(shown) + "&rows=" + term.rows + "&cols=" + term.cols + (takeOver ? "" : "&takeover=0"),
    );
    socket = current;
    current.binaryType = "arraybuffer";
    current.onopen = () => {
      retryDelay = 1000;
      // A login shell can take a while to read its profile.
      showStatus("Waiting for the shell...");
    };
    current.onmessage = (event) => {
      showStatus("");
      // After the replay, so that it lands at the end of what is on screen.
      term.write(new Uint8Array(event.data), pendingSend ? deliverSend : undefined);
    };
    current.onclose = (event) => {
      if (socket !== current) return;
      socket = null;
      settleSend(
        event.code === closeTakenOver
          ? "Another browser tab is using this shell, so nothing was sent."
          : "The terminal lost its connection, so nothing was sent.",
      );
      if (event.code === closeTakenOver) {
        showStatus("Another tab has this terminal.", "Use it here", () => connect());
      } else if (event.code === closeExited) {
        shellGone("The shell ended: " + event.reason + ".");
      } else if (event.code === closeNoShell) {
        shellGone("The shell has ended.");
      } else if (event.code === closeFailed) {
        showStatus("The shell could not be shown: " + event.reason, "Try again", () => connect());
      } else {
        reconnect();
      }
    };
  };
  const show = (id, takeOver = true) => {
    shown = id;
    rememberShown();
    renderTabs();
    connect(takeOver);
    term.focus();
  };
  const newShell = async () => {
    clearTimeout(retryTimer);
    showStatus("Starting a shell...");
    let response;
    try {
      const place = pagePlace();
      response = await serverFetch(shellsPath + "?rows=" + term.rows + "&cols=" + term.cols + (place && "&" + place), { method: "POST" });
    } catch {}
    if (!response?.ok) {
      await refusal(response, "The shell did not start", newShell);
      return;
    }
    const shell = await response.json();
    listSerial++;
    shells = [...shells, shell].sort((a, b) => a.number - b.number);
    show(shell.id);
  };
  // The shown shell has ended. The screen stays if there is nothing to show instead,
  // and a new shell is a choice: it may have been the last thing wanted.
  const shellGone = async (message) => {
    const before = shellsHere();
    const place = before.findIndex((shell) => shell.id === shown);
    const gone = shown;
    shown = null;
    shells = shells.filter((shell) => shell.id !== gone);
    renderTabs();
    showStatus(message, "New shell", newShell);
    const { response, ok } = await refreshShells();
    if (!ok) {
      signInGone(response);
      return;
    }
    if (shown !== null) return; // Someone chose meanwhile.
    const left = shellsHere();
    const next = left[Math.min(Math.max(place, 0), left.length - 1)];
    if (next) show(next.id, false);
  };
  const endShell = async (shell) => {
    if (!confirm("End shell " + shell.number + ", and anything running in it?")) return;
    let response;
    try {
      response = await serverFetch(shellsPath + "/" + encodeURIComponent(shell.id), { method: "DELETE" });
    } catch {}
    if (!response?.ok && response?.status !== 404) {
      await refusal(response, "The shell did not end", () => endShell(shell));
      return;
    }
    // The shown shell's connection closes, which moves on to another.
    if (shell.id !== shown) await refreshShells();
  };
  const noShellMessage = () =>
    (here.project ? "No shell for " + here.project + "." : "No shell here.") +
    " Open the terminal to start one.";
  const settleSend = (problem) => {
    if (!pendingSend) return;
    const { resolve, timer } = pendingSend;
    clearTimeout(timer);
    pendingSend = null;
    resolve(problem);
  };
  // Several lines are safe only where the program takes them as pasted text; anywhere
  // else, each line break would run what came before it. Only the program knows, and it
  // says so by turning on bracketed paste.
  const pasteProblem = (text) =>
    text.includes("\n") && !term.modes.bracketedPasteMode
      ? "What is running in this tab would run each line, so nothing was sent."
      : null;
  const deliverSend = () => {
    if (!pendingSend) return;
    const problem = pasteProblem(pendingSend.text);
    if (!problem) {
      term.paste(pendingSend.text);
      term.focus();
    }
    settleSend(problem);
  };
  // begin shows what this browser tab last showed for this project, or the project's
  // first shell. It starts one only if someone asked.
  const begin = async (takeOver = true) => {
    whenVisible = null;
    showStatus("Connecting...");
    // Someone sending a command has not asked to take a shell from another browser tab.
    if (pendingSend) takeOver = false;
    const listFailed = (response) => {
      settleSend("The shells could not be listed, so nothing was sent.");
      return refusal(response, "The shells could not be listed", () => begin());
    };
    let refused;
    try {
      refused = await refreshHere();
    } catch {
      await listFailed();
      return;
    }
    if (refused) {
      await listFailed(refused);
      return;
    }
    const { response, ok } = await refreshShells();
    if (!ok) {
      await listFailed(response);
      return;
    }
    const wantsNew = newAsked;
    const mayStart = startAsked;
    newAsked = startAsked = false;
    if (wantsNew) {
      await newShell();
      return;
    }
    const inGroup = shellsHere();
    const wanted = rememberedShown();
    const shell = inGroup.find((item) => item.id === wanted) || inGroup[0];
    if (shell) show(shell.id, takeOver);
    else if (mayStart) await newShell();
    else {
      settleSend(noShellMessage());
      renderTabs();
      panel.classList.add("terminal-empty");
      showStatus(
        here.project ? "No shell for " + here.project + " yet." : "No shell here yet.",
        "Start a shell",
        newShell,
      );
    }
  };
  const reconnect = () => {
    if (document.hidden) {
      showStatus("Disconnected.");
      whenVisible = reconnect;
      return;
    }
    showStatus("Reconnecting...");
    retryTimer = setTimeout(async () => {
      // A refused WebSocket does not say why, so ask with a plain request. After
      // the server restarts the sign-in is gone, and retrying can never succeed.
      let response;
      try {
        response = await serverFetch("/_terminal/xterm-license.txt", { method: "HEAD" });
      } catch {}
      if (signInGone(response)) return;
      if (document.hidden) reconnect();
      else if (response?.ok) connect(false);
      else {
        retryDelay = Math.min(retryDelay * 2, 10000);
        reconnect();
      }
    }, retryDelay);
    retryDelay = Math.min(retryDelay * 2, 10000);
  };
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) return;
    if (whenVisible) whenVisible();
    else if (term && !panel.hidden) refreshShells();
  });
  const build = () => {
    panel = document.createElement("section");
    panel.id = "terminal-panel";
    panel.setAttribute("aria-label", "Terminal");
    const bar = document.createElement("div");
    bar.className = "terminal-bar";
    const title = document.createElement("span");
    title.className = "terminal-title";
    title.textContent = "Terminal";
    status = document.createElement("span");
    status.className = "terminal-status";
    status.setAttribute("role", "status");
    status.hidden = true;
    retry = document.createElement("button");
    retry.type = "button";
    retry.className = "terminal-action";
    retry.hidden = true;
    groups = document.createElement("select");
    groups.className = "terminal-groups";
    groups.setAttribute("aria-label", "Project of these shells");
    groups.onchange = () => {
      if (groups.value) location.assign(groups.value);
    };
    tabs = document.createElement("div");
    tabs.className = "terminal-tabs";
    tabs.setAttribute("role", "tablist");
    tabs.setAttribute("aria-label", "Shells");
    add = document.createElement("button");
    add.type = "button";
    add.className = "terminal-add";
    add.setAttribute("aria-label", "New shell");
    add.textContent = "+";
    add.onclick = () => newShell();
    add.onpointerenter = () => refreshHere().catch(() => {});
    const spacer = document.createElement("span");
    spacer.className = "terminal-spacer";
    const clear = document.createElement("button");
    clear.type = "button";
    clear.textContent = "Clear";
    clear.title = "Clear this shell's screen and scrollback, here and on the server";
    clear.onclick = () => {
      term?.focus();
      // Not connected, the server would bring it all back.
      if (socket?.readyState !== WebSocket.OPEN) return;
      // A full-screen program draws its own screen, and would not know it had gone.
      if (term.buffer.active.type === "alternate") {
        const message = "Clear is off while a full-screen program runs.";
        showStatus(message);
        setTimeout(() => {
          if (status.textContent === message) showStatus("");
        }, 4000);
        return;
      }
      term.clear();
      send(JSON.stringify({ type: "clear" }));
    };
    maximize = document.createElement("button");
    maximize.type = "button";
    maximize.onclick = toggleMaximized;
    applyMaximized(rememberedMaximized());
    const hide = document.createElement("button");
    hide.type = "button";
    hide.textContent = "Hide";
    hide.onclick = () => window.toggleTerminalPanel();
    bar.append(
      title,
      groups,
      tabs,
      add,
      status,
      retry,
      spacer,
      fontButton("A-", "Smaller text", -1),
      fontButton("A+", "Larger text", 1),
      clear,
      maximize,
      hide,
    );
    viewport = document.createElement("div");
    viewport.className = "terminal-viewport";
    panel.append(buildResizer(), bar, viewport);
    document.body.append(panel);
  };
  const start = async () => {
    build();
    showStatus("Loading the terminal...");
    try {
      await loadAssets();
    } catch (error) {
      showStatus(error.message);
      settleSend(error.message);
      return;
    }
    term = new window.Terminal({
      cursorBlink: true,
      fontFamily: fontFamily(),
      fontSize: savedFontSize(),
      theme: theme(),
    });
    fit = new window.FitAddon.FitAddon();
    term.loadAddon(fit);
    // A link is only text a program printed, so the new tab gets no hold on this page.
    term.loadAddon(
      new window.WebLinksAddon.WebLinksAddon((_, uri) => window.open(uri, "_blank", "noopener")),
    );
    term.open(viewport);
    fit.fit();
    term.onData((data) => send(new TextEncoder().encode(data)));
    term.onBinary((data) =>
      send(Uint8Array.from(data, (character) => character.charCodeAt(0))),
    );
    term.onResize(({ rows, cols }) =>
      send(JSON.stringify({ type: "resize", rows, cols })),
    );
    new ResizeObserver(() => {
      if (!panel.hidden) fit.fit();
    }).observe(viewport);
    // The theme can change by the viewer's choice or by the system's.
    new MutationObserver(applyTheme).observe(root, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", applyTheme);
    // A font that arrives late changes the size of every cell.
    document.fonts?.ready.then(() => {
      term.options.fontFamily = fontFamily();
      fit.fit();
    });
    // A page opened in the background waits to be looked at before taking a shell.
    if (document.hidden) {
      showStatus("Waiting for this tab to be shown.");
      whenVisible = () => begin();
    } else {
      await begin();
    }
    term.focus();
    setInterval(() => {
      if (!document.hidden && !panel.hidden && !whenVisible) refreshShells();
    }, shellsRefresh);
  };
  const setOpen = (open) => {
    root.classList.toggle("terminal-open", open);
    document
      .getElementById("terminal-button")
      ?.setAttribute("aria-pressed", String(open));
    remember(open);
    if (!panel) {
      if (open)
        start().catch((error) => {
          console.error(error);
          showStatus("The terminal could not start: " + error.message);
          settleSend("The terminal could not start: " + error.message);
        });
      return;
    }
    panel.hidden = !open;
    if (open && term) {
      fit.fit();
      term.focus();
      // Opened again with nothing to show, or asked for a new shell.
      if (newAsked || (startAsked && shown === null)) begin();
    }
  };
  window.toggleTerminalPanel = () => {
    const open = !root.classList.contains("terminal-open");
    startAsked = open;
    setOpen(open);
  };
  // openTerminalHere starts a new shell in this page's place, such as the run's directory.
  window.openTerminalHere = () => {
    newAsked = true;
    setOpen(true);
  };
  // Before the page is drawn, so a panel left open does not make it jump.
  if (savedHeight()) applyHeight(savedHeight());
  if (remembered()) root.classList.add("terminal-open");
  if (rememberedMaximized()) root.classList.add("terminal-maximized");
  addEventListener("resize", () => {
    if (savedHeight()) applyHeight(savedHeight());
  });
  // pastableText is text with only its lines and tabs left. A log's colours mean nothing
  // at a prompt, and an escape could end a bracketed paste and run what followed it.
  const pastableText = (text) =>
    text
      .replace(/\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)?/g, "")
      .replace(/\u001b\[[0-?]*[ -/]*[@-~]/g, "")
      .replace(/\r\n?/g, "\n")
      .replace(/[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/g, "")
      .replace(/\n+$/, "");
  // Well under what the server takes in one message, in any encoding.
  const pasteLimit = 256 * 1024;
  // sendToTerminal types a command into this project's shown shell, without running it,
  // and returns what stopped it, if anything. It starts no shell and takes none from
  // another browser tab: having a command to hand is no reason for either.
  window.sendToTerminal = async (text, { pasted = false } = {}) => {
    if (pasted) {
      text = pastableText(text);
      if (text.length > pasteLimit) return "This is too long to send, so nothing was sent.";
    } else if (/[\u0000-\u001f\u007f-\u009f]/.test(text)) {
      // A line break would run what came before it, and other control characters, from a
      // value in the state, could do anything a keyboard can.
      return "This command has control characters, so it was not sent.";
    }
    if (!text) return "There is nothing to send.";
    if (pendingSend) return "The terminal is still connecting.";
    let refused, listed;
    try {
      refused = await refreshHere();
      listed = refused ? null : await refreshShells();
    } catch {}
    if (!listed?.ok) {
      const response = refused || listed?.response;
      return response?.status === 401 || response?.status === 403
        ? "Sign in again to use the terminal."
        : "The server did not answer, so nothing was sent.";
    }
    const inGroup = shellsHere();
    if (!inGroup.length) return noShellMessage();
    const showing = term && inGroup.some((shell) => shell.id === shown);
    if (showing && !socket) {
      setOpen(true);
      return "This page is not connected to the shell, so nothing was sent.";
    }
    if (showing && socket.readyState === WebSocket.OPEN) {
      setOpen(true);
      const problem = pasteProblem(text);
      if (problem) return problem;
      term.paste(text);
      term.focus();
      return null;
    }
    const waiting = new Promise((resolve) => {
      pendingSend = {
        text,
        resolve,
        timer: setTimeout(() => settleSend("The terminal did not connect, so nothing was sent."), 15000),
      };
    });
    setOpen(true);
    // A panel already showing nothing has no shell to wait for until it looks again.
    if (term && !showing) begin();
    return waiting;
  };
  // A value goes in as one word of a command, so one that needs quoting gets it.
  const asWord = (value) =>
    /^[A-Za-z0-9_\/.:=@%+,-]+$/.test(value) ? value : "'" + value.replace(/'/g, "'\\''") + "'";
  let sendNote, sendNoteTimer;
  window.sendToTerminalAndSay = async (text, { word = false, pasted = false } = {}) => {
    const problem = await window.sendToTerminal(word ? asWord(text) : text, { pasted });
    if (!sendNote) {
      sendNote = document.createElement("div");
      sendNote.className = "terminal-send-note";
      sendNote.setAttribute("role", "status");
      document.body.append(sendNote);
    }
    sendNote.textContent = problem || "Sent. Nothing runs until you press Enter in the terminal.";
    sendNote.hidden = false;
    clearTimeout(sendNoteTimer);
    sendNoteTimer = setTimeout(() => (sendNote.hidden = true), problem ? 6000 : 2500);
  };
  window.sendButtonPressed = (button) => {
    const { sendText, sendValue } = button.dataset;
    return window.sendToTerminalAndSay(sendText ?? (sendValue || ""), { word: sendText === undefined });
  };
  // The Recent jobs page is drawn on the server, which leaves its copy buttons plain.
  const addJobsSendButtons = () => {
    for (const copy of document.querySelectorAll(".jobs-copy")) {
      const send = copy.cloneNode(true);
      const label = (copy.getAttribute("aria-label") || "").replace(/^Copy /, "");
      send.classList.add("terminal-send");
      send.title = "Send " + label + " to the terminal";
      send.setAttribute("aria-label", send.title);
      // A command is already a command, and anything else is one word of one.
      if (label === "command") send.dataset.sendText = copy.dataset.copyValue;
      else send.dataset.sendValue = copy.dataset.copyValue;
      delete send.dataset.copyValue;
      send.dataset.action = "sendToTerminal";
      const icon = send.querySelector("svg");
      icon.replaceChildren();
      for (const shape of ["M4 12h15", "m13 6 6 6-6 6"]) {
        const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
        path.setAttribute("d", shape);
        icon.append(path);
      }
      copy.after(send);
    }
  };
  const sayAlive = async () => {
    try {
      await serverFetch("/_terminal/alive", { method: "POST" });
    } catch {}
  };
  document.addEventListener("DOMContentLoaded", () => {
    const offered = document.getElementById("terminal-button");
    if (offered) {
      addJobsSendButtons();
      sayAlive();
      setInterval(sayAlive, aliveInterval);
    }
    if (offered && remembered()) setOpen(true);
    else root.classList.remove("terminal-open");
  });
})();

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
    const closeInbox = () => {
      inboxPanel.hidden = true;
      inboxButton.setAttribute("aria-expanded", "false");
    };
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && !inboxPanel.hidden) {
        closeInbox();
        inboxButton.focus();
      }
    });
    document.addEventListener("pointerdown", (event) => {
      if (!inboxPanel.hidden && !container.contains(event.target)) closeInbox();
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
