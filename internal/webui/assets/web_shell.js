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
// of them. A tab says which shell, and a pane says where it shows, in up to four panes
// as columns of rows. Only shells shown in a pane are connected; the others carry on with no one
// reading, and replay when shown. Moving between projects with the panel open starts nothing: browsing
// must not leave a shell in every project. Nothing loads until the panel first opens; a
// page without the top bar's Terminal button has no terminal to offer.
(() => {
  // The panel can have a window of its own, whose page holds nothing else. Its path is
  // windowPath and then the path of a page of the project it shows.
  const windowPath = "/_terminal/window";
  const inWindow = document.documentElement.hasAttribute("data-terminal-window");
  const openKey = "rotari-terminal-open";
  const heightKey = "rotari-terminal-height";
  const widthKey = "rotari-terminal-width";
  const placeKey = "rotari-terminal-place";
  // The line that marks where the panel is, in a picture of the window.
  const placeLines = { bottom: "M1.5 9.5h13", left: "M5.5 2.5v11", right: "M10.5 2.5v11" };
  // The bar's pictures, drawn in a 16 by 16 box: a string is a path, and an object a rectangle.
  const paneFrame = { x: 1.5, y: 2.5, width: 13, height: 11, rx: 1.5 };
  const icons = {
    clear: ["M2.5 4.5h8", "M2.5 8h8", "M2.5 11.5h4.5", "M10 10l3.5 3.5", "M13.5 10L10 13.5"],
    closePane: [paneFrame, "M6 6l4 4", "M10 6l-4 4"],
    maximize: ["M9.5 2.5h4v4", "M13.5 2.5L9 7", "M6.5 13.5h-4v-4", "M2.5 13.5L7 9"],
    restore: ["M13.5 2.5L9.5 6.5", "M9.5 3v3.5H13", "M2.5 13.5l4-4", "M6.5 13V9.5H3"],
    hide: ["M3.5 12.5h9"],
  };
  // drawIcon replaces what a button shows with a picture. Its label names it for those who
  // cannot see the picture, and its title for those who point at it.
  const drawIcon = (button, shapes) => {
    const svgNS = "http://www.w3.org/2000/svg";
    const icon = document.createElementNS(svgNS, "svg");
    icon.setAttribute("viewBox", "0 0 16 16");
    icon.setAttribute("aria-hidden", "true");
    for (const shape of shapes) {
      const element = document.createElementNS(svgNS, typeof shape === "string" ? "path" : "rect");
      if (typeof shape === "string") element.setAttribute("d", shape);
      else for (const [name, value] of Object.entries(shape)) element.setAttribute(name, String(value));
      icon.append(element);
    }
    button.classList.add("terminal-icon");
    button.replaceChildren(icon);
  };
  const shownKey = "rotari-terminal-shell";
  const layoutKey = "rotari-terminal-layout";
  // What a browser tab's panes showed for each project, which goes to its window and back.
  const isPaneKey = (key) =>
    typeof key === "string" && (key.startsWith(shownKey + ":") || key.startsWith(layoutKey + ":"));
  // A browser tab whose panel is in a window says so here, so that following a link finds
  // the window again rather than taking its shells.
  const windowedKey = "rotari-terminal-windowed";
  // A window says it is there this often, and a page loaded while it was waits this long
  // to hear it before putting the panel back.
  const windowBeat = 1000;
  // The project page this browser tab last had its window follow, so that the window
  // follows the page only when the page moves to another project, and a project chosen
  // in the window stays until then.
  const followedKey = "rotari-terminal-followed";
  const windowWait = 3000;
  // A window takes the panes its page handed it in its address, once.
  if (inWindow && location.hash.length > 1) {
    try {
      const handover = JSON.parse(decodeURIComponent(location.hash.slice(1)));
      for (const [key, value] of Object.entries(handover)) {
        if (isPaneKey(key) && typeof value === "string") sessionStorage.setItem(key, value);
      }
    } catch {}
    history.replaceState(null, "", location.pathname + location.search);
  }
  // A page that asked for a shell in its place, such as a run's directory, opens the window
  // on its project with that place, for the next shell started.
  let placeOverride = "";
  if (inWindow) {
    placeOverride = new URLSearchParams(location.search).get("start") || "";
    if (placeOverride) history.replaceState(null, "", location.pathname);
  }
  const maximizedKey = "rotari-terminal-maximized";
  const fontSizeKey = "rotari-terminal-font-size";
  const defaultFontSize = 13;
  const minFontSize = 8;
  const maxFontSize = 32;
  const shellsPath = "/_terminal/shells";
  // How often a page in view picks up shells started or ended from elsewhere, for its tabs
  // and the count on the Terminal button, with the panel open or not.
  const shellsRefresh = 5000;
  // How often any open page says so, whether its panel is open or in view, which keeps
  // the shells. Browsers slow a background page's timers to about this. It is far under
  // the server's idle limit.
  const aliveInterval = 60000;
  const minHeight = 120;
  const minWidth = 320;
  const minContentWidth = 400;
  const maxPanes = 4;
  const minPaneWidth = 200;
  const minPaneHeight = 100;
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
  let panel, panesBox, groups, tabs, add, splitRightButton, splitDownButton, closePaneButton, placeButton, maximize;
  let shells = [];
  let shellLimit = 0;
  let groupLimit = 0;
  let here = { basedir: "", project: "", dir: "" }; // The page's place.
  let tabsDrawn = "";
  let groupsDrawn = "";
  let startAsked = false; // Someone opened the panel: start a shell if there is none.
  let newAsked = false; // Someone asked for a new shell here.
  let listSerial = 0;
  let endingHere = new Set(); // Shells this page asked to end, which need no telling.
  let terminalWindow = null; // The window the panel moved to, while it is open.
  let windowTimer;
  // Loaded while the panel was in a window, this page waits to hear from it.
  let awaitingWindow = false;
  if (!inWindow) {
    try {
      awaitingWindow = sessionStorage.getItem(windowedKey) === "1";
    } catch {}
  }
  // newPane makes a place to show a shell: one terminal screen, and one connection to
  // the shell it shows. All of a connection's state is its pane's own.
  const newPane = () => ({
    element: null,
    status: null,
    action: null,
    term: null,
    fit: null,
    observer: null,
    viewport: null,
    socket: null,
    shown: null, // The ID of the shell this pane shows.
    pendingSend: null, // A command waiting for this pane to connect to a shell.
    connectSerial: 0,
    waitingToOpen: false,
    retryTimer: undefined,
    retryDelay: 1000,
    // What to do when this tab next becomes visible: a tab nobody is looking at
    // neither connects nor retries. Browsers make connections to a server wait for
    // each other, and old tabs retrying would hold up the tab in use.
    whenVisible: null,
  });
  // What the bar's buttons and sending from the page act on. It exists before the panel
  // does, since a command can wait for the panel to open.
  let focused = newPane();
  // The panes in the order they appear. A shell is never in two of them: a second
  // connection would take it from the first.
  let panes = [focused];
  // Where they are: columns side by side, each holding panes one above another. A share
  // is a part of what holds it, and the shares of its neighbors add up to one.
  let columns = [{ share: 1, rows: [{ share: 1, pane: focused }] }];
  const readingOrder = () => columns.flatMap((column) => column.rows.map((row) => row.pane));
  const handles = [];
  // A panel too narrow for screens side by side stacks everything, and the columns stay as
  // they are for when it has room. At the bottom, its width is the window's.
  const stackedUnder = 720;
  let laidStacked = false;
  const paneShowing = (id) => panes.find((pane) => pane.shown === id);

  // A window's panel is open as long as the window is, which says nothing of the pages'.
  const remember = (open) => {
    if (inWindow) return;
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
  // The page beside a panel needs its sidebar, which someone can resize, and room for the
  // rest. Before the page exists there is no sidebar to measure, so this runs again then.
  const pageWidthNeeded = () => (document.querySelector(".sidebar")?.offsetWidth || 0) + minContentWidth;
  const clampWidth = (width) =>
    Math.max(minWidth, Math.min(width, Math.max(minWidth, innerWidth - pageWidthNeeded())));
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
    const label = maximized ? "Restore" : "Maximize";
    drawIcon(maximize, maximized ? icons.restore : icons.maximize);
    maximize.title = maximized ? "Restore the panel's size" : "Give the panel the whole window";
    maximize.setAttribute("aria-label", label);
    maximize.setAttribute("aria-pressed", String(maximized));
  };
  const toggleMaximized = () => {
    const maximized = !root.classList.contains("terminal-maximized");
    applyMaximized(maximized);
    try {
      localStorage.setItem(maximizedKey, maximized ? "1" : "0");
    } catch {}
    focused.term?.focus();
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
    const { term } = focused;
    term?.focus();
    if (!term) return;
    const size = term.options.fontSize + step;
    if (size < minFontSize || size > maxFontSize) return;
    for (const pane of panes) pane.term.options.fontSize = size;
    // Fewer or more cells fit now, and the shells hear of it through the resize.
    fitPanes();
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
  // Where the panel sits. Its height at the bottom and its width at a side are kept apart,
  // so that moving it and back finds each as it was.
  const chosenPlace = () => {
    try {
      const place = localStorage.getItem(placeKey);
      if (place === "left" || place === "right") return place;
    } catch {}
    return "bottom";
  };
  let chosen = chosenPlace();
  // A page narrower than its layouts expect is the danger of a panel at a side, and the
  // sidebar of a narrow window needs the left edge. The choice stays for a wider window.
  const wideWindow = window.matchMedia?.("(min-width: 1000px)");
  const panelPlace = () => (inWindow || wideWindow?.matches === false ? "bottom" : chosen);
  const panelSizes = {
    height: { key: heightKey, variable: "--terminal-saved-height", clamp: clampHeight },
    width: { key: widthKey, variable: "--terminal-saved-width", clamp: clampWidth },
  };
  const panelAxis = () => (panelPlace() === "bottom" ? "height" : "width");
  const applySize = (axis, size) => {
    // Only an open panel takes room, which web_shell.css decides from this.
    root.style.setProperty(panelSizes[axis].variable, panelSizes[axis].clamp(size) + "px");
  };
  const savedSize = (axis) => {
    try {
      return Number(localStorage.getItem(panelSizes[axis].key)) || 0;
    } catch {
      return 0;
    }
  };
  const currentSize = (axis) => panel?.getBoundingClientRect()[axis] || savedSize(axis);
  const saveSize = (axis) => {
    try {
      localStorage.setItem(panelSizes[axis].key, String(Math.round(currentSize(axis))));
    } catch {}
  };
  // applyPlace puts the panel where it goes, which web_shell.css decides from this. Every
  // screen fits itself to its new room, and no connection changes.
  const applyPlace = () => {
    const place = panelPlace();
    if (place === "bottom") delete root.dataset.terminalPlace;
    else root.dataset.terminalPlace = place;
    if (!panel) return;
    panel.querySelector(".terminal-resizer")?.setAttribute("aria-orientation", place === "bottom" ? "horizontal" : "vertical");
    placeButton.querySelector("path").setAttribute("d", placeLines[place]);
    applyLayout();
  };
  const choosePlace = (place) => {
    chosen = place;
    try {
      localStorage.setItem(placeKey, place);
    } catch {}
    applyPlace();
    focused.term?.focus();
  };
  const applySavedSizes = () => {
    for (const axis of Object.keys(panelSizes)) {
      if (savedSize(axis)) applySize(axis, savedSize(axis));
    }
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
    for (const pane of panes) if (pane.term) pane.term.options.theme = theme();
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
      const place = panelPlace();
      const axis = panelAxis();
      const start = currentSize(axis);
      const { clientX: startX, clientY: startY } = event;
      resizer.onpointermove = ({ clientX: x, clientY: y }) => {
        const moved = { bottom: () => startY - y, left: () => x - startX, right: () => startX - x }[place]();
        applySize(axis, start + moved);
      };
      resizer.onpointerup = resizer.onpointercancel = () => {
        resizer.onpointermove = resizer.onpointerup = resizer.onpointercancel = null;
        saveSize(axis);
      };
    };
    resizer.onkeydown = (event) => {
      const steps = {
        bottom: { ArrowUp: 24, ArrowDown: -24 },
        left: { ArrowRight: 24, ArrowLeft: -24 },
        right: { ArrowLeft: 24, ArrowRight: -24 },
      };
      const step = steps[panelPlace()][event.key];
      if (!step) return;
      event.preventDefault();
      const axis = panelAxis();
      applySize(axis, currentSize(axis) + step);
      saveSize(axis);
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
  // A pane says what it is waiting for or what went wrong, and two panes can differ. It
  // sits over the screen rather than taking room from it: a message coming and going must
  // not resize the shell.
  const showStatus = (pane, message, action, handler) => {
    const { status, action: button } = pane;
    status.textContent = message;
    status.hidden = !message;
    button.textContent = action || "";
    button.hidden = !action;
    button.onclick = handler || null;
    pane.element.querySelector(".terminal-pane-status").hidden = !message && !action;
  };
  const fitPanes = () => {
    for (const pane of panes) pane.fit.fit();
  };
  const send = (pane, data) => {
    if (pane.socket?.readyState === WebSocket.OPEN) pane.socket.send(data);
  };
  // Each project keeps its own last tab and panes, for this browser tab alone.
  const shownStorageKey = () => shownKey + ":" + here.basedir + ":" + here.project;
  const layoutStorageKey = () => layoutKey + ":" + here.basedir + ":" + here.project;
  // While the panel is in a window, the window decides what the panes are.
  const rememberLayout = () => {
    if (terminalWindow || awaitingWindow) return;
    try {
      if (panes[0].shown !== null) sessionStorage.setItem(shownStorageKey(), panes[0].shown);
      if (panes.length > 1) {
        sessionStorage.setItem(
          layoutStorageKey(),
          JSON.stringify({
            columns: columns.map((column) => ({
              share: column.share,
              rows: column.rows.map((row) => ({ share: row.share, shell: row.pane.shown })),
            })),
            focused: focused.shown,
          }),
        );
      } else {
        sessionStorage.removeItem(layoutStorageKey());
      }
      if (inWindow) {
        tellOpener({
          type: "rotari-terminal-saved",
          entries: {
            [shownStorageKey()]: sessionStorage.getItem(shownStorageKey()),
            [layoutStorageKey()]: sessionStorage.getItem(layoutStorageKey()),
          },
        });
      }
    } catch {}
  };
  const rememberedShown = () => {
    try {
      return sessionStorage.getItem(shownStorageKey());
    } catch {
      return null;
    }
  };
  // A list's shares as parts of one. A value someone edited or an older version left must
  // not make a pane vanish.
  const evenShares = (items) => {
    const shares = items.map(({ share }) =>
      Number.isFinite(share) && share > 0 ? Math.max(0.1, Math.min(share, 1)) : 1 / items.length,
    );
    const whole = shares.reduce((sum, share) => sum + share, 0);
    items.forEach((item, index) => (item.share = shares[index] / whole));
  };
  // removeRow takes a row out of a layout and returns who got its room: its neighbor in
  // the column, or the first of the next column to it if it was alone.
  const removeRow = (layout, row) => {
    const column = layout.find((item) => item.rows.includes(row));
    const place = column.rows.indexOf(row);
    if (column.rows.length > 1) {
      column.rows.splice(place, 1);
      const neighbor = column.rows[Math.max(place - 1, 0)];
      neighbor.share += row.share;
      return neighbor;
    }
    const at = layout.indexOf(column);
    layout.splice(at, 1);
    const neighbor = layout[Math.max(at - 1, 0)];
    if (neighbor) neighbor.share += column.share;
    return neighbor?.rows[0];
  };
  // rememberedLayout reads what this project last had, as far as it still makes sense: only
  // shells there are, each once, up to the limit, as if the panes of the rest had closed.
  const rememberedLayout = (available) => {
    let saved;
    try {
      saved = JSON.parse(sessionStorage.getItem(layoutStorageKey()));
    } catch {}
    if (!Array.isArray(saved?.columns)) return null;
    const layout = saved.columns
      .filter((column) => Array.isArray(column?.rows) && column.rows.length)
      .map((column) => ({
        share: column.share,
        rows: column.rows.filter(Boolean).map((row) => ({ share: row.share, shell: row.shell })),
      }))
      .filter((column) => column.rows.length);
    evenShares(layout);
    layout.forEach((column) => evenShares(column.rows));
    const seen = new Set();
    for (const row of layout.flatMap((column) => column.rows)) {
      if (available.some((shell) => shell.id === row.shell) && !seen.has(row.shell) && seen.size < maxPanes) {
        seen.add(row.shell);
      } else {
        removeRow(layout, row);
      }
    }
    return layout.length ? { layout, focused: saved.focused } : null;
  };
  // The page's basedir ID, project, and run, as it would ask for them in a query. The
  // parts are still percent-encoded, so they go in as they are.
  const pagePlace = () => {
    if (placeOverride) return placeOverride;
    const path = inWindow ? location.pathname.slice(windowPath.length) : location.pathname;
    const parts = path.split("/").filter(Boolean);
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
    if (inWindow) document.title = projectName(here) + " - Terminal - rotari";
    return null;
  };
  // The projects with shells, this page's first. Choosing one goes to its page, so that
  // what the panel shows always follows the page. A window goes to its own page for that
  // project, and has the page that opened it go to that project's page.
  const renderGroups = () => {
    const entries = [{ ...here, link: "", count: shellsHere().length }];
    for (const shell of shells) {
      if (sameGroup(shell, here)) continue;
      const entry = entries.find((item) => sameGroup(item, shell));
      if (entry) {
        entry.count++;
        entry.unseen ||= shell.unseen_output;
      } else {
        entries.push({
          basedir: shell.basedir,
          project: shell.project,
          link: shell.link,
          count: 1,
          unseen: shell.unseen_output,
        });
      }
    }
    const label = (entry) => {
      const twin = entries.some((item) => item !== entry && item.project === entry.project);
      return (
        (entry.unseen ? "\u2022 " : "") +
        projectName(entry) +
        (twin && entry.basedir ? " in " + entry.basedir : "") +
        " (" +
        entry.count +
        ")"
      );
    };
    const drawn = JSON.stringify(entries.map((entry) => [label(entry), entry.link]));
    if (drawn === groupsDrawn) return;
    groupsDrawn = drawn;
    groups.replaceChildren(
      ...entries.map((entry) => {
        const option = document.createElement("option");
        option.value = inWindow && entry.link ? windowPath + entry.link : entry.link;
        option.textContent = label(entry);
        return option;
      }),
    );
    groups.value = "";
  };
  // endShells ends several shells after asking once. Those shown go last and the focused
  // pane's last of all, so that moving on from one does not land on one about to end.
  const endShells = async (targets) => {
    if (!targets.length) return;
    const noun = targets.length === 1 ? "shell" : targets.length + " shells";
    const running = targets.length === 1 ? "it" : "them";
    if (!confirm("End " + noun + ", and anything running in " + running + "?")) return;
    const rank = (shell) => (shell.id === focused.shown ? 2 : paneShowing(shell.id) ? 1 : 0);
    const ordered = [...targets].sort((a, b) => rank(a) - rank(b));
    for (const shell of targets) endingHere.add(shell.id);
    for (const shell of ordered) {
      if (paneShowing(shell.id)) await refreshShells();
      let response;
      try {
        response = await serverFetch(shellsPath + "/" + encodeURIComponent(shell.id), { method: "DELETE" });
      } catch {}
      if (!response?.ok && response?.status !== 404) {
        await refusal(focused, response, "The shell did not end", () => endShells(targets));
        return;
      }
    }
    if (!targets.some((shell) => paneShowing(shell.id))) await refreshShells();
  };
  // arrangeShells puts this project's shells in the order given, here at once and then on
  // the server, so that later pages and other browser tabs see it.
  const arrangeShells = async (arranged) => {
    const others = shells.filter((shell) => !sameGroup(shell, here));
    listSerial++;
    shells = [...arranged, ...others];
    renderTabs();
    let response;
    try {
      response = await serverFetch("/_terminal/order", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: arranged.map((shell) => shell.id) }),
      });
    } catch {}
    if (!response?.ok) {
      await refusal(focused, response, "The tabs were not rearranged", () => arrangeShells(arranged));
      await refreshShells();
    }
  };
  const moveShell = (shell, place) => {
    const arranged = shellsHere().filter((item) => item.id !== shell.id);
    arranged.splice(place, 0, shell);
    arrangeShells(arranged);
  };
  // draggingShell is the tab being dragged. Drawing the tabs again would end the drag.
  let draggingShell = null;
  const clearDropMarks = () => {
    for (const tab of tabs.children) tab.classList.remove("terminal-drop-before", "terminal-drop-after");
  };
  const dropsAfter = (event, tab) => {
    const box = tab.getBoundingClientRect();
    return event.clientX > box.left + box.width / 2;
  };
  const makeDraggable = (tab, shell) => {
    tab.draggable = true;
    tab.ondragstart = (event) => {
      draggingShell = shell;
      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "move";
        // Some browsers start no drag without data.
        event.dataTransfer.setData("text/plain", shell.name || String(shell.number));
      }
    };
    tab.ondragover = (event) => {
      if (!draggingShell) return;
      event.preventDefault();
      clearDropMarks();
      tab.classList.add(dropsAfter(event, tab) ? "terminal-drop-after" : "terminal-drop-before");
    };
    tab.ondrop = (event) => {
      event.preventDefault();
      const dragged = draggingShell;
      if (!dragged) return;
      const after = dropsAfter(event, tab);
      draggingShell = null;
      clearDropMarks();
      if (dragged.id === shell.id) return;
      const arranged = shellsHere().filter((item) => item.id !== dragged.id);
      arranged.splice(arranged.findIndex((item) => item.id === shell.id) + (after ? 1 : 0), 0, dragged);
      arrangeShells(arranged);
    };
    tab.ondragend = () => {
      draggingShell = null;
      clearDropMarks();
      renderTabs();
    };
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
      await refusal(focused, response, "The tab was not renamed", () => renameShell(shell));
      return;
    }
    await refreshShells();
  };
  let panelMenu = null;
  const closePanelMenu = () => {
    panelMenu?.remove();
    panelMenu = null;
  };
  // openPanelMenu lists choices by the pointer, or by what opened it for the keyboard. Each
  // is a label, what it does, and whether it can be chosen, and in a list of alternatives
  // whether it is the current one.
  const openPanelMenu = (event, choices) => {
    event.preventDefault();
    closePanelMenu();
    panelMenu = document.createElement("div");
    panelMenu.className = "terminal-tab-menu";
    panelMenu.setAttribute("role", "menu");
    for (const [label, action, enabled, current] of choices) {
      const item = document.createElement("button");
      item.type = "button";
      item.setAttribute("role", current === undefined ? "menuitem" : "menuitemradio");
      if (current !== undefined) item.setAttribute("aria-checked", String(current));
      item.textContent = label;
      item.disabled = !enabled;
      item.onclick = () => {
        closePanelMenu();
        action();
      };
      panelMenu.append(item);
    }
    document.body.append(panelMenu);
    const anchor = event.currentTarget.getBoundingClientRect();
    const size = panelMenu.getBoundingClientRect();
    const x = event.clientX || anchor.left;
    const y = event.clientY || anchor.bottom;
    panelMenu.style.left = Math.max(0, Math.min(x, innerWidth - size.width)) + "px";
    panelMenu.style.top = Math.max(0, Math.min(y, innerHeight - size.height)) + "px";
    panelMenu.querySelector("button:not(:disabled)").focus();
  };
  const openTabMenu = (event, shell) => {
    const inGroup = shellsHere();
    const place = inGroup.findIndex((item) => item.id === shell.id);
    const right = inGroup.slice(place + 1);
    openPanelMenu(event, [
      ["Rename...", () => renameShell(shell), true],
      ["Move left", () => moveShell(shell, place - 1), place > 0],
      ["Move right", () => moveShell(shell, place + 1), right.length > 0],
      ["Close tabs to the right", () => endShells(right), right.length > 0],
      ["Close all tabs", () => endShells(inGroup), true],
    ]);
  };
  // A narrow window has only the bottom to offer, and the menu says why. A window of the
  // panel's own has only the page to go back to.
  const openPlaceMenu = (event) => {
    if (inWindow) {
      openPanelMenu(event, [["Back to page", backToPage, true]]);
      return;
    }
    const wide = wideWindow?.matches !== false;
    openPanelMenu(
      event,
      [
        ["Bottom", "bottom"],
        ["Left", "left"],
        ["Right", "right"],
      ].map(([label, place]) => [
        place === "bottom" || wide ? label : label + " (needs a wider window)",
        () => choosePlace(place),
        place === "bottom" || wide,
        chosen === place,
      ]).concat([["Window", popOut, true, false]]),
    );
  };
  document.addEventListener("pointerdown", (event) => {
    if (panelMenu && !panelMenu.contains(event.target)) closePanelMenu();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && panelMenu) {
      closePanelMenu();
      focused.term?.focus();
    }
  });
  // A tab shows in the focused pane. One already in another pane changes places with
  // what is here, since a shell cannot be in two.
  const chooseTab = (id) => {
    const showing = paneShowing(id);
    if (!showing) show(focused, id);
    else if (showing === focused) focused.term.focus();
    else swapInto(showing);
  };
  // swapInto puts a pane where the focused one is. Only their places change: connecting
  // again would replay both.
  const swapInto = (pane) => {
    const slots = columns.flatMap((column) => column.rows);
    const here = slots.find((slot) => slot.pane === focused);
    const there = slots.find((slot) => slot.pane === pane);
    here.pane = pane;
    there.pane = focused;
    panes = readingOrder();
    focusPane(pane);
    pane.term.focus();
  };
  // showShellCount puts the number of shells, of every project, on the Terminal button. With
  // no answer from the server it shows none, rather than one that may be out of date.
  const showShellCount = (known = true) => {
    const button = document.getElementById("terminal-button");
    if (button) button.textContent = known ? "Terminal (" + shells.length + ")" : "Terminal";
  };
  const renderTabs = () => {
    showShellCount();
    renderGroups();
    const inGroup = shellsHere();
    add.disabled = inGroup.length >= groupLimit || shells.length >= shellLimit;
    updatePaneButtons();
    const drawn = JSON.stringify([
      inGroup.map((shell) => [shell.id, shell.number, shell.name, shell.dir, shell.unseen_output]),
      panes.map((pane) => pane.shown),
      panes.indexOf(focused),
    ]);
    // Drawing them again would take focus from a tab and move nothing.
    if (drawn === tabsDrawn || draggingShell) return;
    tabsDrawn = drawn;
    tabs.replaceChildren(
      ...inGroup.map((shell) => {
        const tab = document.createElement("span");
        tab.className = "terminal-tab";
        tab.setAttribute("role", "presentation");
        // A shown tab is connected, whatever the list said a moment ago.
        const showing = paneShowing(shell.id);
        const elsewhere = Boolean(showing) && showing !== focused;
        const unseen = Boolean(shell.unseen_output) && !showing;
        tab.classList.toggle("terminal-tab-unseen", unseen);
        tab.classList.toggle("terminal-tab-other-pane", elsewhere);
        const label = document.createElement("button");
        label.type = "button";
        label.setAttribute("role", "tab");
        label.setAttribute("aria-selected", String(shell.id === focused.shown));
        label.setAttribute(
          "aria-label",
          "Shell " + shell.number + (shell.name ? ", " + shell.name : "") + (unseen ? ", new output" : "") + (elsewhere ? ", in another pane" : ""),
        );
        label.title = (unseen ? "New output. " : "") + (elsewhere ? "In another pane. " : "") + "Started in " + shell.dir;
        label.textContent = shell.name || String(shell.number);
        label.onclick = () => chooseTab(shell.id);
        const close = document.createElement("button");
        close.type = "button";
        close.className = "terminal-tab-close";
        close.setAttribute("aria-label", "End shell " + shell.number);
        close.textContent = "\u00d7";
        close.onclick = () => endShell(shell);
        tab.oncontextmenu = (event) => openTabMenu(event, shell);
        makeDraggable(tab, shell);
        tab.append(label, close);
        return tab;
      }),
    );
  };
  // A sign-in that has gone cannot be retried away.
  const signInGone = (pane, response) => {
    if (response?.status !== 401 && response?.status !== 403) return false;
    showStatus(pane, "Sign in again to use the terminal.", "Reload", () => location.reload());
    return true;
  };
  const refusal = async (pane, response, what, handler) => {
    if (signInGone(pane, response)) return;
    const reason = response ? (await response.text()).trim() : "the server did not answer";
    showStatus(pane, what + ": " + reason, "Try again", handler);
  };
  const shellName = (shell) => {
    const name = shell.name || "Shell " + shell.number;
    return shell.project ? name + " of " + shell.project : name;
  };
  // noteEndedShells tells of shells that ended out of sight: the shown one's connection
  // says so itself, and a shell someone ended from this page is no news.
  const noteEndedShells = (now) => {
    const ended = shells.filter(
      (shell) =>
        !paneShowing(shell.id) &&
        !endingHere.has(shell.id) &&
        !now.some((other) => other.id === shell.id),
    );
    if (!ended.length) return;
    showNote((ended.length === 1 ? shellName(ended[0]) : ended.length + " shells") + " ended.", 6000);
  };
  // refreshShells updates the tabs, and returns whether it could. A newer change here
  // wins over an older answer, which may not have heard of it.
  const refreshShells = async () => {
    const serial = ++listSerial;
    let response;
    try {
      response = await serverFetch(shellsPath);
    } catch {}
    if (!response?.ok) {
      showShellCount(false);
      return { response };
    }
    const listed = await response.json();
    if (serial === listSerial) {
      if (panel && !terminalWindow) noteEndedShells(listed.shells);
      shells = listed.shells;
      shellLimit = listed.limit;
      groupLimit = listed.group_limit;
      showShellCount();
      if (panel) renderTabs();
    }
    return { ok: true };
  };
  // connect attaches a pane to its shell. Reconnecting by itself passes takeOver false,
  // so it cannot take the shell from a tab in use; only someone opening a page or
  // pressing a button does that.
  const connect = (pane, takeOver = true) => {
    clearTimeout(pane.retryTimer);
    pane.whenVisible = null;
    panel.classList.remove("terminal-empty");
    const previous = pane.socket;
    pane.socket = null;
    previous?.close();
    showStatus(pane, "Connecting...");
    const serial = ++pane.connectSerial;
    pane.waitingToOpen = true;
    // xterm.js parses what it has been given later, and reset leaves that waiting, so a
    // fast program's output would land on the next shell's screen. Wait for it to finish.
    pane.term.write("", () => {
      if (serial !== pane.connectSerial) return;
      pane.waitingToOpen = false;
      open(pane, takeOver);
    });
  };
  const open = (pane, takeOver) => {
    const { term } = pane;
    // Replay brings the screen back, so what is here would appear twice.
    term.reset();
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    const current = new WebSocket(
      scheme + "//" + location.host + "/_terminal/ws?shell=" + encodeURIComponent(pane.shown) + "&rows=" + term.rows + "&cols=" + term.cols + (takeOver ? "" : "&takeover=0"),
    );
    pane.socket = current;
    current.binaryType = "arraybuffer";
    current.onopen = () => {
      if (pane.socket !== current) return;
      pane.retryDelay = 1000;
      // A login shell can take a while to read its profile.
      showStatus(pane, "Waiting for the shell...");
    };
    current.onmessage = (event) => {
      // Closing leaves what was already on its way, which is another shell's output now.
      if (pane.socket !== current) return;
      showStatus(pane, "");
      // After the replay, so that it lands at the end of what is on screen.
      term.write(new Uint8Array(event.data), pane.pendingSend ? () => deliverSend(pane) : undefined);
    };
    current.onclose = (event) => {
      if (pane.socket !== current) return;
      pane.socket = null;
      settleSend(
        pane,
        event.code === closeTakenOver
          ? "Another browser tab is using this shell, so nothing was sent."
          : "The terminal lost its connection, so nothing was sent.",
      );
      if (event.code === closeTakenOver) {
        showStatus(pane, "Another tab has this terminal.", "Use it here", () => connect(pane));
      } else if (event.code === closeExited) {
        shellGone(pane, "The shell ended: " + event.reason + ".");
      } else if (event.code === closeNoShell) {
        shellGone(pane, "The shell has ended.");
      } else if (event.code === closeFailed) {
        showStatus(pane, "The shell could not be shown: " + event.reason, "Try again", () => connect(pane));
      } else {
        reconnect(pane);
      }
    };
  };
  const show = (pane, id, takeOver = true) => {
    pane.shown = id;
    rememberLayout();
    renderTabs();
    connect(pane, takeOver);
    if (pane === focused) pane.term.focus();
  };
  // startShell starts a shell at a pane's size, and returns it, or says why not there.
  const startShell = async (pane, again) => {
    showStatus(pane, "Starting a shell...");
    let response;
    try {
      const place = pagePlace();
      placeOverride = "";
      response = await serverFetch(shellsPath + "?rows=" + pane.term.rows + "&cols=" + pane.term.cols + (place && "&" + place), { method: "POST" });
    } catch {}
    if (!response?.ok) {
      await refusal(pane, response, "The shell did not start", again);
      return null;
    }
    const shell = await response.json();
    listSerial++;
    shells = [...shells, shell];
    return shell;
  };
  const newShell = async (pane) => {
    clearTimeout(pane.retryTimer);
    const shell = await startShell(pane, () => newShell(pane));
    if (shell) show(pane, shell.id);
  };
  // addPane gives a new row in the layout its pane, which gets its screen once placed.
  const addPane = (row) => {
    row.pane = newPane();
    buildPane(row.pane);
    return row.pane;
  };
  // closePane takes a pane away and leaves its shell running, or gone if it has ended.
  const closePane = (pane) => {
    if (panes.length < 2 || !panes.includes(pane)) return;
    clearTimeout(pane.retryTimer);
    pane.whenVisible = null;
    pane.connectSerial++;
    const { socket } = pane;
    pane.socket = null;
    socket?.close();
    settleSend(pane, "That pane closed, so nothing was sent.");
    pane.observer.disconnect();
    pane.term.dispose();
    pane.element.remove();
    const row = columns.flatMap((column) => column.rows).find((item) => item.pane === pane);
    const neighbor = removeRow(columns, row);
    panes = readingOrder();
    if (focused === pane) {
      focused = neighbor.pane;
      focused.term.focus();
    }
    applyLayout();
    rememberLayout();
    renderTabs();
  };
  // withSplit is the layout after splitting a pane, with a row for the new one. Right makes
  // a column of the whole height, beside the pane's column; this goes no deeper.
  const withSplit = (pane, right) => {
    const layout = columns.map((column) => ({ ...column, rows: column.rows.map((row) => ({ ...row })) }));
    const column = layout.find((item) => item.rows.some((row) => row.pane === pane));
    const row = column.rows.find((item) => item.pane === pane);
    if (right) {
      const added = { share: 1, pane: null };
      column.share /= 2;
      layout.splice(layout.indexOf(column) + 1, 0, { share: column.share, rows: [added] });
      return { layout, added };
    }
    const added = { share: row.share / 2, pane: null };
    row.share /= 2;
    column.rows.splice(column.rows.indexOf(row) + 1, 0, added);
    return { layout, added };
  };
  // A box of no size has not been laid out, which says nothing about the room.
  const hasRoom = (layout) => {
    const { width, height } = panesBox.getBoundingClientRect();
    if (!width || !height) return true;
    return layoutGeometry(layout, isStacked()).places.every(
      ({ box }) => box.w * width >= minPaneWidth && box.h * height >= minPaneHeight,
    );
  };
  let splitting = false;
  // splitPane shows another shell beside this one, starting one if there is no other. Nothing
  // is started past the limit, and a pane appears only once it has something to show.
  const splitPane = async (right) => {
    if (splitting || panes.length >= maxPanes) return;
    const pane = focused;
    if (!hasRoom(withSplit(pane, right).layout)) {
      showBriefly(pane, "There is too little room to split this pane.");
      return;
    }
    const inGroup = shellsHere();
    const place = inGroup.findIndex((shell) => shell.id === pane.shown);
    const free = inGroup.filter((shell) => !paneShowing(shell.id));
    let shell = inGroup.slice(place + 1).find((item) => !paneShowing(item.id)) || free[0];
    if (!shell) {
      if (inGroup.length >= groupLimit || shells.length >= shellLimit) {
        showBriefly(pane, "No more shells can start here, so there is nothing to split with.");
        return;
      }
      splitting = true;
      shell = await startShell(pane, () => splitPane(right));
      splitting = false;
      if (!shell) return;
      showStatus(pane, "");
    }
    // The panes may have changed while the shell started, so this splits what is there now.
    if (!panes.includes(pane) || panes.length >= maxPanes) return;
    const { layout, added } = withSplit(pane, right);
    columns = layout;
    const opened = addPane(added);
    panes = readingOrder();
    focused = opened;
    applyLayout();
    // It fits itself on opening, and the others follow through their observers.
    openTerminal(opened);
    show(opened, shell.id);
  };
  // A pane's shell has ended. The screen stays if there is nothing to show instead,
  // and a new shell is a choice: it may have been the last thing wanted.
  const shellGone = async (pane, message) => {
    const before = shellsHere();
    const place = before.findIndex((shell) => shell.id === pane.shown);
    const gone = pane.shown;
    pane.shown = null;
    shells = shells.filter((shell) => shell.id !== gone);
    renderTabs();
    showStatus(pane, message, "New shell", () => newShell(pane));
    const { response, ok } = await refreshShells();
    if (!ok) {
      signInGone(pane, response);
      return;
    }
    if (pane.shown !== null || !panes.includes(pane)) return; // Someone chose meanwhile.
    const left = shellsHere().filter((shell) => !paneShowing(shell.id));
    const next = left[Math.min(Math.max(place, 0), left.length - 1)];
    if (next) show(pane, next.id, false);
    else closePane(pane); // The other panes have all there is. With none, this stays.
  };
  const endShell = async (shell) => {
    if (!confirm("End shell " + shell.number + ", and anything running in it?")) return;
    endingHere.add(shell.id);
    let response;
    try {
      response = await serverFetch(shellsPath + "/" + encodeURIComponent(shell.id), { method: "DELETE" });
    } catch {}
    if (!response?.ok && response?.status !== 404) {
      await refusal(focused, response, "The shell did not end", () => endShell(shell));
      return;
    }
    // A shown shell's connection closes, which moves on to another.
    if (!paneShowing(shell.id)) await refreshShells();
  };
  const noShellMessage = () =>
    (here.project ? "No shell for " + here.project + "." : "No shell here.") +
    " Open the terminal to start one.";
  const settleSend = (pane, problem) => {
    if (!pane.pendingSend) return;
    const { resolve, timer } = pane.pendingSend;
    clearTimeout(timer);
    pane.pendingSend = null;
    resolve(problem);
  };
  // Several lines are safe only where the program takes them as pasted text; anywhere
  // else, each line break would run what came before it. Only the program knows, and it
  // says so by turning on bracketed paste.
  const pasteProblem = (pane, text) =>
    text.includes("\n") && !pane.term.modes.bracketedPasteMode
      ? "What is running in this tab would run each line, so nothing was sent."
      : null;
  const deliverSend = (pane) => {
    if (!pane.pendingSend) return;
    const problem = pasteProblem(pane, pane.pendingSend.text);
    if (!problem) {
      pane.term.paste(pane.pendingSend.text);
      pane.term.focus();
    }
    settleSend(pane, problem);
  };
  const awaitSend = (pane, text, resolve) => {
    pane.pendingSend = {
      text,
      resolve,
      timer: setTimeout(() => settleSend(pane, "The terminal did not connect, so nothing was sent."), 15000),
    };
  };
  const moveSend = (from, to) => {
    if (!from.pendingSend) return;
    const { text, resolve, timer } = from.pendingSend;
    clearTimeout(timer);
    from.pendingSend = null;
    awaitSend(to, text, resolve);
  };
  // restoreLayout brings back the panes this browser tab last had here, with a waiting
  // command going to the one that had focus, fitted before any of them connects.
  const restoreLayout = (pane, { layout, focused: wanted }, takeOver) => {
    const rows = layout.flatMap((column) => column.rows);
    const shellOf = new Map(rows.map((row) => [row, row.shell]));
    rows.forEach((row, place) => {
      if (place === 0) row.pane = pane;
      else addPane(row);
      delete row.shell;
    });
    columns = layout;
    panes = readingOrder();
    const chosen = rows.find((row) => shellOf.get(row) === wanted)?.pane || pane;
    if (chosen !== pane) moveSend(pane, chosen);
    focused = chosen;
    applyLayout();
    for (const opened of panes) if (!opened.term) openTerminal(opened);
    fitPanes();
    for (const row of rows) show(row.pane, shellOf.get(row), takeOver);
  };
  // begin shows what this browser tab last showed for this project, in the same panes, or
  // the project's first shell. It starts one only if someone asked.
  const begin = async (takeOver = true) => {
    const pane = focused;
    pane.whenVisible = null;
    showStatus(pane, "Connecting...");
    // Someone sending a command has not asked to take a shell from another browser tab.
    if (pane.pendingSend) takeOver = false;
    const listFailed = (response) => {
      settleSend(pane, "The shells could not be listed, so nothing was sent.");
      return refusal(pane, response, "The shells could not be listed", () => begin());
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
      await newShell(pane);
      return;
    }
    // Other panes are there only when this runs again on a panel in use.
    const others = panes.length > 1;
    const inGroup = shellsHere().filter((item) => item.id === pane.shown || !paneShowing(item.id));
    const saved = others ? null : rememberedLayout(inGroup);
    const wanted = saved?.layout[0].rows[0].shell ?? rememberedShown();
    const shell = inGroup.find((item) => item.id === wanted) || inGroup[0];
    if (saved && saved.layout.flatMap((column) => column.rows).length > 1) {
      restoreLayout(pane, saved, takeOver);
    } else if (shell) show(pane, shell.id, takeOver);
    else if (others) closePane(pane);
    else if (mayStart) await newShell(pane);
    else {
      settleSend(pane, noShellMessage());
      renderTabs();
      panel.classList.add("terminal-empty");
      showStatus(
        pane,
        here.project ? "No shell for " + here.project + " yet." : "No shell here yet.",
        "Start a shell",
        () => newShell(pane),
      );
    }
  };
  const reconnect = (pane) => {
    if (document.hidden) {
      showStatus(pane, "Disconnected.");
      pane.whenVisible = () => reconnect(pane);
      return;
    }
    showStatus(pane, "Reconnecting...");
    pane.retryTimer = setTimeout(async () => {
      // A refused WebSocket does not say why, so ask with a plain request. After
      // the server restarts the sign-in is gone, and retrying can never succeed.
      let response;
      try {
        response = await serverFetch("/_terminal/xterm-license.txt", { method: "HEAD" });
      } catch {}
      if (!panes.includes(pane) || signInGone(pane, response)) return;
      if (document.hidden) reconnect(pane);
      else if (response?.ok) connect(pane, false);
      else {
        pane.retryDelay = Math.min(pane.retryDelay * 2, 10000);
        reconnect(pane);
      }
    }, pane.retryDelay);
    pane.retryDelay = Math.min(pane.retryDelay * 2, 10000);
  };
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) return;
    const waiting = panes.filter((pane) => pane.whenVisible);
    if (waiting.length) waiting.forEach((pane) => pane.whenVisible());
    else refreshShells();
  });
  // percent writes a part of the panel for a style, and place sets where an element goes,
  // keeping half a handle clear of each edge that has a neighbor.
  const percent = (part) => Math.round(part * 1e6) / 1e4 + "%";
  const inset = (start, hasBefore) => (hasBefore ? "calc(" + percent(start) + " + 3px)" : percent(start));
  const span = (size, edges) => (edges ? "calc(" + percent(size) + " - " + edges * 3 + "px)" : percent(size));
  const place = (element, box, before, after) => {
    element.style.left = inset(box.x, before.x);
    element.style.top = inset(box.y, before.y);
    element.style.width = span(box.w, before.x + after.x);
    element.style.height = span(box.h, before.y + after.y);
  };
  const isStacked = () => {
    const { width } = panesBox.getBoundingClientRect();
    return width > 0 && width < stackedUnder;
  };
  // layoutGeometry works out where everything in a layout goes, as parts of the panes' box.
  const layoutGeometry = (layout, stacked) => {
    const places = [];
    const boundaries = [];
    let across = 0;
    layout.forEach((column, c) => {
      const lastColumn = c === layout.length - 1;
      let down = 0;
      column.rows.forEach((row, r) => {
        const lastRow = r === column.rows.length - 1;
        places.push({
          row,
          box: stacked
            ? { x: 0, y: across + down * column.share, w: 1, h: row.share * column.share }
            : { x: across, y: down, w: column.share, h: row.share },
          before: stacked ? { x: 0, y: Number(c > 0 || r > 0) } : { x: Number(c > 0), y: Number(r > 0) },
          after: stacked ? { x: 0, y: Number(!lastColumn || !lastRow) } : { x: Number(!lastColumn), y: Number(!lastRow) },
        });
        if (!lastRow) {
          const scale = stacked ? column.share : 1;
          const start = (stacked ? across : 0) + down * scale;
          boundaries.push({
            a: row,
            b: column.rows[r + 1],
            horizontal: true,
            start,
            scale,
            at: start + row.share * scale,
            from: stacked ? 0 : across,
            size: stacked ? 1 : column.share,
          });
        }
        down += row.share;
      });
      if (!lastColumn) {
        boundaries.push({
          a: column,
          b: layout[c + 1],
          horizontal: stacked,
          start: across,
          scale: 1,
          at: across + column.share,
          from: 0,
          size: 1,
        });
      }
      across += column.share;
    });
    return { places, boundaries };
  };
  const updatePaneButtons = () => {
    const canSplit = panes.length < maxPanes && focused.shown !== null;
    splitRightButton.disabled = splitDownButton.disabled = !canSplit;
    closePaneButton.disabled = panes.length < 2;
  };
  // applyLayout places the panes and the handles between them, by position alone: moving
  // a screen in the page, as columns of their own elements would need, would make
  // browsers forget its scrolling.
  const applyLayout = () => {
    laidStacked = isStacked();
    const { places, boundaries } = layoutGeometry(columns, laidStacked);
    for (const { row, box, before, after } of places) {
      place(row.pane.element, box, before, after);
      row.pane.element.classList.toggle("terminal-pane-focused", row.pane === focused);
    }
    panesBox.classList.toggle("terminal-panes-split", panes.length > 1);
    while (handles.length > boundaries.length) handles.pop().remove();
    boundaries.forEach((boundary, index) => {
      if (!handles[index]) {
        handles[index] = buildSplitHandle();
        panesBox.append(handles[index]);
      }
      const handle = handles[index];
      handle.boundary = boundary;
      handle.classList.toggle("terminal-split-handle-rows", boundary.horizontal);
      handle.setAttribute("aria-orientation", boundary.horizontal ? "horizontal" : "vertical");
      handle.setAttribute("aria-valuenow", String(Math.round((boundary.a.share / (boundary.a.share + boundary.b.share)) * 100)));
      const line = "calc(" + percent(boundary.at) + " - 3px)";
      handle.style.left = boundary.horizontal ? percent(boundary.from) : line;
      handle.style.top = boundary.horizontal ? line : percent(boundary.from);
      handle.style.width = boundary.horizontal ? percent(boundary.size) : "6px";
      handle.style.height = boundary.horizontal ? "6px" : percent(boundary.size);
    });
    updatePaneButtons();
  };
  // focusPane makes a pane the one the bar and sending act on, following where the keys go.
  const focusPane = (pane) => {
    focused = pane;
    applyLayout();
    rememberLayout();
    renderTabs();
  };
  // moveBoundary gives one neighbor a share, and the other what is left of the two, leaving
  // neither under the smallest pane, unless the two have less room than that between them.
  const moveBoundary = ({ a, b, horizontal, scale }, share) => {
    const both = a.share + b.share;
    const { width, height } = panesBox.getBoundingClientRect();
    const room = (horizontal ? height : width) * scale;
    const smallest = room ? (horizontal ? minPaneHeight : minPaneWidth) / room : 0.05;
    const least = Math.min(smallest, both / 2);
    a.share = Math.max(least, Math.min(share, both - least));
    b.share = both - a.share;
    applyLayout();
  };
  const buildSplitHandle = () => {
    const handle = document.createElement("div");
    handle.className = "terminal-split-handle";
    handle.tabIndex = 0;
    handle.setAttribute("role", "separator");
    handle.setAttribute("aria-label", "Resize the panes");
    handle.onpointerdown = (event) => {
      event.preventDefault();
      handle.setPointerCapture?.(event.pointerId);
      handle.onpointermove = (move) => {
        const box = panesBox.getBoundingClientRect();
        const { boundary } = handle;
        const part = boundary.horizontal ? (move.clientY - box.top) / box.height : (move.clientX - box.left) / box.width;
        if (Number.isFinite(part)) moveBoundary(boundary, (part - boundary.start) / boundary.scale);
      };
      handle.onpointerup = handle.onpointercancel = () => {
        handle.onpointermove = handle.onpointerup = handle.onpointercancel = null;
        rememberLayout();
        focused.term.focus();
      };
    };
    handle.onkeydown = (event) => {
      const step = { ArrowLeft: -0.05, ArrowUp: -0.05, ArrowRight: 0.05, ArrowDown: 0.05 }[event.key];
      if (!step) return;
      event.preventDefault();
      moveBoundary(handle.boundary, handle.boundary.a.share + step);
      rememberLayout();
    };
    return handle;
  };
  const showBriefly = (pane, message) => {
    showStatus(pane, message);
    setTimeout(() => {
      if (pane.status.textContent === message) showStatus(pane, "");
    }, 4000);
  };
  const buildPane = (pane) => {
    pane.element = document.createElement("div");
    pane.element.className = "terminal-pane";
    pane.element.addEventListener("focusin", () => {
      if (focused !== pane) focusPane(pane);
    });
    const notice = document.createElement("div");
    notice.className = "terminal-pane-status";
    notice.hidden = true;
    pane.status = document.createElement("span");
    pane.status.className = "terminal-status";
    pane.status.setAttribute("role", "status");
    pane.status.hidden = true;
    pane.action = document.createElement("button");
    pane.action.type = "button";
    pane.action.className = "terminal-action";
    pane.action.hidden = true;
    notice.append(pane.status, pane.action);
    pane.viewport = document.createElement("div");
    pane.viewport.className = "terminal-viewport";
    pane.element.append(notice, pane.viewport);
    panesBox.append(pane.element);
  };
  const build = () => {
    panel = document.createElement("section");
    panel.id = "terminal-panel";
    panel.setAttribute("aria-label", "Terminal");
    const bar = document.createElement("div");
    bar.className = "terminal-bar";
    const title = document.createElement("span");
    title.className = "terminal-title";
    title.textContent = "Terminal";
    groups = document.createElement("select");
    groups.className = "terminal-groups";
    groups.setAttribute("aria-label", "Project of these shells");
    groups.onchange = () => {
      if (!groups.value) return;
      // A window takes the page that opened it along, as the panel goes to the page.
      if (inWindow) tellOpener({ type: "rotari-terminal-go", path: groups.value.slice(windowPath.length) });
      location.assign(groups.value);
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
    add.onclick = () => newShell(focused);
    add.onpointerenter = () => refreshHere().catch(() => {});
    const spacer = document.createElement("span");
    spacer.className = "terminal-spacer";
    const clear = document.createElement("button");
    clear.type = "button";
    drawIcon(clear, icons.clear);
    clear.setAttribute("aria-label", "Clear");
    clear.title = "Clear this shell's screen and scrollback, here and on the server";
    clear.onclick = () => {
      const pane = focused;
      const { term, socket } = pane;
      term?.focus();
      // Not connected, the server would bring it all back.
      if (socket?.readyState !== WebSocket.OPEN) return;
      // A full-screen program draws its own screen, and would not know it had gone.
      if (term.buffer.active.type === "alternate") {
        showBriefly(pane, "Clear is off while a full-screen program runs.");
        return;
      }
      term.clear();
      send(pane, JSON.stringify({ type: "clear" }));
    };
    const barButton = (text, title, onclick) => {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = text;
      button.title = title;
      button.disabled = true;
      button.onclick = onclick;
      return button;
    };
    const iconButton = (label, shapes, onclick, title = label) => {
      const button = barButton("", title, onclick);
      button.setAttribute("aria-label", label);
      drawIcon(button, shapes);
      return button;
    };
    // The split and place pictures are the Table viewer's panes, for the same choices.
    splitRightButton = iconButton("Split right", [paneFrame, "M8 2.5v11"], () => splitPane(true));
    splitDownButton = iconButton("Split down", [paneFrame, "M1.5 8h13"], () => splitPane(false));
    placeButton = iconButton("Panel position", [paneFrame, placeLines.bottom], openPlaceMenu);
    placeButton.disabled = false;
    closePaneButton = iconButton(
      "Close pane",
      icons.closePane,
      () => {
        closePane(focused);
        focused.term?.focus();
      },
      "Close this pane; its shell keeps running",
    );
    maximize = document.createElement("button");
    maximize.type = "button";
    maximize.onclick = toggleMaximized;
    applyMaximized(rememberedMaximized());
    const hide = document.createElement("button");
    hide.type = "button";
    drawIcon(hide, icons.hide);
    hide.setAttribute("aria-label", "Hide");
    hide.title = "Hide the terminal; its shells keep running";
    hide.onclick = () => window.toggleTerminalPanel();
    bar.append(
      title,
      groups,
      tabs,
      add,
      spacer,
      fontButton("A-", "Smaller text", -1),
      fontButton("A+", "Larger text", 1),
      clear,
      splitRightButton,
      splitDownButton,
      closePaneButton,
      placeButton,
      maximize,
      hide,
    );
    panesBox = document.createElement("div");
    panesBox.className = "terminal-panes";
    buildPane(focused);
    // A window has the whole of itself, with nothing to resize, maximize, or hide it for.
    if (inWindow) {
      for (const button of [maximize, hide]) button.remove();
      panel.append(bar, panesBox);
    } else {
      panel.append(buildResizer(), bar, panesBox);
    }
    document.body.append(panel);
    applyPlace();
    new ResizeObserver(() => {
      if (isStacked() !== laidStacked) applyLayout();
    }).observe(panesBox);
  };
  // openTerminal gives a pane its screen, inside its viewport.
  const openTerminal = (pane) => {
    const { viewport } = pane;
    const term = new window.Terminal({
      cursorBlink: true,
      fontFamily: fontFamily(),
      fontSize: savedFontSize(),
      theme: theme(),
    });
    const fit = new window.FitAddon.FitAddon();
    pane.term = term;
    pane.fit = fit;
    term.loadAddon(fit);
    // A link is only text a program printed, so the new tab gets no hold on this page.
    term.loadAddon(
      new window.WebLinksAddon.WebLinksAddon((_, uri) => window.open(uri, "_blank", "noopener")),
    );
    term.open(viewport);
    fit.fit();
    term.onData((data) => send(pane, new TextEncoder().encode(data)));
    term.onBinary((data) =>
      send(pane, Uint8Array.from(data, (character) => character.charCodeAt(0))),
    );
    term.onResize(({ rows, cols }) =>
      send(pane, JSON.stringify({ type: "resize", rows, cols })),
    );
    // A pane just removed or out of sight measures as nothing.
    pane.observer = new ResizeObserver(() => {
      if (!panel.hidden && viewport.clientWidth) fit.fit();
    });
    pane.observer.observe(viewport);
  };
  const start = async () => {
    build();
    showStatus(focused, "Loading the terminal...");
    try {
      await loadAssets();
    } catch (error) {
      showStatus(focused, error.message);
      settleSend(focused, error.message);
      return;
    }
    openTerminal(focused);
    // The theme can change by the viewer's choice or by the system's.
    new MutationObserver(applyTheme).observe(root, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", applyTheme);
    // A font that arrives late changes the size of every cell.
    document.fonts?.ready.then(() => {
      for (const pane of panes) pane.term.options.fontFamily = fontFamily();
      fitPanes();
    });
    // A page opened in the background waits to be looked at before taking a shell.
    if (document.hidden) {
      showStatus(focused, "Waiting for this tab to be shown.");
      focused.whenVisible = () => begin();
    } else {
      await begin();
    }
    focused.term.focus();
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
          showStatus(focused, "The terminal could not start: " + error.message);
          settleSend(focused, "The terminal could not start: " + error.message);
        });
      return;
    }
    panel.hidden = !open;
    if (open && focused.term) {
      fitPanes();
      focused.term.focus();
      // Opened again with nothing to show, or asked for a new shell.
      if (newAsked || (startAsked && focused.shown === null)) begin();
    }
  };
  // tellOpener sends a window's news to the page that opened it, if it is still there and
  // still this server's.
  const tellOpener = (message) => {
    try {
      if (window.opener && !window.opener.closed) window.opener.postMessage(message, location.origin);
    } catch {}
  };
  // The page's path as far as its project, which a window opens on. A run's page gives
  // its project.
  const projectPath = (path = location.pathname) => {
    const parts = path.split("/").filter(Boolean);
    const kept = [];
    if (parts[0] === "_basedir" && parts[1]) kept.push(...parts.splice(0, 2));
    if (parts[0] === "project" && parts[1]) kept.push(parts[0], parts[1]);
    return "/" + kept.join("/");
  };
  // letGo gives up every pane's connection, saying nothing of it, and closes the panel
  // without remembering that, for the window to take the shells.
  const letGo = () => {
    for (const pane of panes) {
      clearTimeout(pane.retryTimer);
      pane.whenVisible = null;
      pane.connectSerial++;
      const { socket } = pane;
      pane.socket = null;
      socket?.close();
      settleSend(pane, "The terminal moved to its own window, so nothing was sent.");
    }
    root.classList.remove("terminal-open");
    document.getElementById("terminal-button")?.setAttribute("aria-pressed", "false");
    if (panel) panel.hidden = true;
  };
  // A closed window sends no word, so the page looks.
  const watchWindow = () => {
    clearTimeout(windowTimer);
    windowTimer = setTimeout(() => {
      if (!terminalWindow) return;
      if (terminalWindow.closed) windowGone();
      else watchWindow();
    }, 500);
  };
  const useWindow = (opened) => {
    terminalWindow = opened;
    awaitingWindow = false;
    try {
      sessionStorage.setItem(windowedKey, "1");
    } catch {}
    watchWindow();
  };
  const followed = () => {
    try {
      return sessionStorage.getItem(followedKey);
    } catch {
      return null;
    }
  };
  const rememberFollowed = (path) => {
    try {
      sessionStorage.setItem(followedKey, path);
    } catch {}
  };
  // followPage has the window show this page's project, when the page is of a project
  // other than the one the window last followed. A page of no project leaves it.
  const followPage = () => {
    const path = projectPath();
    if (!path.includes("/project/") || path === followed()) return;
    rememberFollowed(path);
    terminalWindow.postMessage({ type: "rotari-terminal-follow", path }, location.origin);
  };
  // popOut moves the panel to a window of its own, with the panes this browser tab has for
  // each project. It opens the window before anything else, since a browser allows one only
  // straight from a click.
  const popOut = () => {
    rememberLayout();
    const handover = {};
    try {
      for (let index = 0; index < sessionStorage.length; index++) {
        const key = sessionStorage.key(index);
        if (isPaneKey(key)) handover[key] = sessionStorage.getItem(key);
      }
    } catch {}
    const address = windowPath + projectPath() + "#" + encodeURIComponent(JSON.stringify(handover));
    const opened = window.open(address, "", "popup,width=900,height=600");
    if (!opened) {
      showBriefly(focused, "The browser blocked the window. Allow pop-ups for this site to use it.");
      focused.term?.focus();
      return;
    }
    letGo();
    useWindow(opened);
    rememberFollowed(projectPath());
  };
  // windowGone puts the panel back where it was, with the panes the window last had for
  // this project.
  const windowGone = () => {
    if (!terminalWindow && !awaitingWindow) return;
    clearTimeout(windowTimer);
    while (panes.length > 1) closePane(panes[panes.length - 1]);
    terminalWindow = null;
    awaitingWindow = false;
    try {
      sessionStorage.removeItem(windowedKey);
    } catch {}
    if (!panel) {
      setOpen(true);
      return;
    }
    setOpen(true);
    begin();
  };
  // Back to page closes the window, and the page that opened it shows the panel again.
  const backToPage = () => {
    if (!window.opener || window.opener.closed) {
      showBriefly(focused, "The page that opened this window is gone, so the terminal stays here.");
      return;
    }
    tellOpener({ type: "rotari-terminal-back" });
    window.close();
  };
  // sendToWindow has the window type text into its focused pane, and returns what stopped
  // it. A pane in another project than the page's is named in sentToProject.
  let sendSerial = 0;
  let sentToProject = "";
  const sendsWaiting = new Map();
  const sendWait = 10000;
  const sendToWindow = (text) => {
    const id = ++sendSerial;
    const answer = new Promise((resolve) => {
      const timer = setTimeout(() => {
        sendsWaiting.delete(id);
        resolve("The terminal's window did not answer, so nothing was sent.");
      }, sendWait);
      sendsWaiting.set(id, (reply) => {
        clearTimeout(timer);
        sendsWaiting.delete(id);
        resolve(reply);
      });
    });
    terminalWindow.postMessage({ type: "rotari-terminal-send", id, text, path: projectPath() }, location.origin);
    terminalWindow.focus();
    return answer;
  };
  // In the window, only the page that opened it sends, from this server.
  addEventListener("message", async (event) => {
    if (!inWindow || event.origin !== location.origin || !window.opener || event.source !== window.opener) return;
    const { type, id, text, path, place } = event.data || {};
    if (type === "rotari-terminal-send" && typeof text === "string") {
      const problem = await sendToPane(focused, text);
      const elsewhere = !problem && typeof path === "string" && projectPath(path) !== projectPath(location.pathname.slice(windowPath.length));
      tellOpener({ type: "rotari-terminal-sent", id, problem, project: elsewhere ? projectName(here) : "" });
    } else if (type === "rotari-terminal-follow" && typeof path === "string") {
      if (!path.startsWith("/") || path.startsWith("//")) return;
      if (projectPath(path) !== projectPath(location.pathname.slice(windowPath.length))) {
        location.assign(windowPath + projectPath(path));
      }
    } else if (type === "rotari-terminal-here" && typeof path === "string" && typeof place === "string") {
      // Only a path on this server: one starting // would be another.
      if (!path.startsWith("/") || path.startsWith("//")) return;
      window.focus();
      if (projectPath(path) === projectPath(location.pathname.slice(windowPath.length))) {
        placeOverride = place;
        newAsked = true;
        begin();
      } else {
        location.assign(windowPath + projectPath(path) + (place ? "?start=" + encodeURIComponent(place) : ""));
      }
    }
  });
  // Only this server's pages, and only the window this page opened, have a say. A page
  // loaded while the panel was in a window takes the first that says it is there.
  addEventListener("message", (event) => {
    if (inWindow || event.origin !== location.origin || typeof event.data?.type !== "string") return;
    const { type } = event.data;
    if (type === "rotari-terminal-window" && awaitingWindow && event.source) {
      useWindow(event.source);
      followPage();
      return;
    }
    if (!terminalWindow || event.source !== terminalWindow) return;
    if (type === "rotari-terminal-saved") {
      try {
        for (const [key, value] of Object.entries(event.data.entries || {})) {
          if (!isPaneKey(key)) continue;
          if (typeof value === "string") sessionStorage.setItem(key, value);
          else sessionStorage.removeItem(key);
        }
      } catch {}
    } else if (type === "rotari-terminal-back") {
      windowGone();
    } else if (type === "rotari-terminal-go") {
      // Only a path on this server: one starting // would be another.
      const { path } = event.data;
      if (typeof path === "string" && path.startsWith("/") && !path.startsWith("//")) {
        rememberFollowed(projectPath(path));
        location.assign(path);
      }
    } else if (type === "rotari-terminal-sent" && sendsWaiting.has(event.data.id)) {
      const { problem, project } = event.data;
      if (!problem && typeof project === "string") sentToProject = project;
      sendsWaiting.get(event.data.id)(typeof problem === "string" ? problem : null);
    }
  });
  window.toggleTerminalPanel = () => {
    if (terminalWindow) {
      terminalWindow.focus();
      return;
    }
    // Opening the panel now would take the shells from a window about to answer.
    if (awaitingWindow) return;
    const open = !root.classList.contains("terminal-open");
    startAsked = open;
    setOpen(open);
  };
  // openTerminalHere starts a new shell in this page's place, such as the run's directory.
  window.openTerminalHere = () => {
    if (terminalWindow) {
      terminalWindow.postMessage({ type: "rotari-terminal-here", path: projectPath(), place: pagePlace() }, location.origin);
      terminalWindow.focus();
      return;
    }
    if (awaitingWindow) return;
    newAsked = true;
    setOpen(true);
  };
  // Before the page is drawn, so a panel left open does not make it jump.
  applySavedSizes();
  applyPlace();
  wideWindow?.addEventListener?.("change", applyPlace);
  if (remembered() && !awaitingWindow) root.classList.add("terminal-open");
  if (rememberedMaximized()) root.classList.add("terminal-maximized");
  addEventListener("resize", applySavedSizes);
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
  // sendToTerminal types a command into the focused pane's shell, without running it,
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
    if (terminalWindow) return sendToWindow(text);
    if (awaitingWindow) return "The terminal's window has not answered yet, so nothing was sent.";
    return sendToPane(focused, text);
  };
  const sendToPane = async (pane, text) => {
    if (pane.pendingSend) return "The terminal is still connecting.";
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
    const showing = pane.term && inGroup.some((shell) => shell.id === pane.shown);
    // Between connect and open, it is only waiting for the old output to finish.
    if (showing && !pane.socket && !pane.waitingToOpen) {
      setOpen(true);
      return "This page is not connected to the shell, so nothing was sent.";
    }
    if (showing && pane.socket?.readyState === WebSocket.OPEN) {
      setOpen(true);
      const problem = pasteProblem(pane, text);
      if (problem) return problem;
      pane.term.paste(text);
      pane.term.focus();
      return null;
    }
    const waiting = new Promise((resolve) => {
      awaitSend(pane, text, resolve);
    });
    setOpen(true);
    // A panel already showing nothing has no shell to wait for until it looks again.
    if (pane.term && !showing) begin();
    return waiting;
  };
  // A value goes in as one word of a command, so one that needs quoting gets it.
  const asWord = (value) =>
    /^[A-Za-z0-9_\/.:=@%+,-]+$/.test(value) ? value : "'" + value.replace(/'/g, "'\\''") + "'";
  let sendNote, sendNoteTimer;
  const showNote = (message, duration) => {
    if (!sendNote) {
      sendNote = document.createElement("div");
      sendNote.className = "terminal-send-note";
      sendNote.setAttribute("role", "status");
      document.body.append(sendNote);
    }
    sendNote.textContent = message;
    sendNote.hidden = false;
    clearTimeout(sendNoteTimer);
    sendNoteTimer = setTimeout(() => (sendNote.hidden = true), duration);
  };
  window.sendToTerminalAndSay = async (text, { word = false, pasted = false } = {}) => {
    sentToProject = "";
    const problem = await window.sendToTerminal(word ? asWord(text) : text, { pasted });
    const sent = sentToProject
      ? "Sent to the terminal's window, in " + sentToProject + ". Nothing runs until you press Enter there."
      : "Sent. Nothing runs until you press Enter in the terminal.";
    showNote(problem || sent, problem ? 6000 : sentToProject ? 4000 : 2500);
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
    const offered = inWindow || document.getElementById("terminal-button");
    if (offered) {
      if (!inWindow) addJobsSendButtons();
      sayAlive();
      setInterval(sayAlive, aliveInterval);
      applySavedSizes();
      // Only its width matters. Its height follows a panel at the bottom, so answering that
      // would put the panel back to its saved height while someone drags it.
      const sidebar = document.querySelector(".sidebar");
      if (sidebar) {
        let measured = sidebar.offsetWidth;
        new ResizeObserver(() => {
          if (sidebar.offsetWidth === measured) return;
          measured = sidebar.offsetWidth;
          if (savedSize("width")) applySize("width", savedSize("width"));
        }).observe(sidebar);
      }
      setInterval(() => {
        if (!document.hidden && !panes.some((pane) => pane.whenVisible)) refreshShells();
      }, shellsRefresh);
    }
    // Opening a window asks for a shell, as opening the panel does.
    if (inWindow) {
      if (placeOverride) newAsked = true;
      else startAsked = true;
      setOpen(true);
      const beat = () => {
        tellOpener({ type: "rotari-terminal-window" });
        setTimeout(beat, windowBeat);
      };
      beat();
    } else if (offered && awaitingWindow) {
      setTimeout(() => {
        if (awaitingWindow) windowGone();
      }, windowWait);
    } else if (offered && remembered()) setOpen(true);
    else {
      root.classList.remove("terminal-open");
      // A page out of sight asks nothing until it is looked at.
      if (offered && !document.hidden) refreshShells();
    }
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
