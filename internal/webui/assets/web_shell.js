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
    copySelectedJobs: (el, args) =>
      copySelectedJobs(el.dataset.project, el.dataset.run, ...args),
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
    historySearchUpdateValueControl: (el) =>
      historySearchUpdateValueControl(el),
    loadArtifactDirectory: (_, args) => loadArtifactDirectory(...args),
    loadArtifactText: (_, args) => loadArtifactText(...args),
    log: (el) =>
      log(
        el.dataset.project,
        el.dataset.logRun,
        el.dataset.logJob,
        el.dataset.attempt,
        el.dataset.logMode,
      ),
    moreArtifactTable: () => moreArtifactTable(),
    moreArtifactText: () => moreArtifactText(),
    openArtifact: (_, args) => openArtifact(...args),
    openMatrixCell: (el) => openMatrixCell(el),
    popupMenuToggled: (el) => popupMenuToggled(el),
    refreshWithBasedirs: () => refreshWithBasedirs(),
    removeQueueJob: (el) =>
      removeQueueJob(el.dataset.project, el.dataset.job, el.dataset.label),
    saveQueueJob: (el) =>
      saveQueueJob(el.dataset.project, el.dataset.job, el.closest("tr")),
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
    showLog: (el) =>
      showLog(
        el.dataset.project,
        el.dataset.run,
        el.dataset.job,
        el.dataset.attempt,
        "stdout",
        el.dataset.logMode,
      ),
    showNotes: (el) => showNotes(el),
    showNotificationConfig: () => showNotificationConfig(),
    showOriginalOutput: (el) =>
      showOriginalOutput(el.dataset.run, el.dataset.job, el),
    showPath: (el) => showPath(el.dataset.path),
    showRecoverRun: (el) => showRecoverRun(el.dataset.project, el.dataset.run),
    showRunFiles: (_, args) => showRunFiles(...args),
    showRunReport: (el) => showRunReport(el),
    startArtifactLaunch: (_, args) => startArtifactLaunch(...args),
    stopLaunch: (_, args) => stopLaunch(...args),
    toggleAllJobsNotificationBasedirs: (el) =>
      toggleAllJobsNotificationBasedirs(el),
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

// copyJobsValue copies the value of a copy button the server draws (writeJobsCopyButton in
// assets.go), on the Recent jobs page and in either page's Settings menu, and shows a
// check mark for a moment; copyJobsValueFallback is for a browser without the clipboard API.
function copyJobsValueFallback(value) {
  const input = document.createElement("textarea");
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.append(input);
  input.select();
  const copied = document.execCommand("copy");
  input.remove();
  if (!copied) throw new Error("Copy failed");
}

async function copyJobsValue(button) {
  const value = button.dataset.copyValue;
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        await navigator.clipboard.writeText(value);
      } catch (error) {
        copyJobsValueFallback(value);
      }
    } else {
      copyJobsValueFallback(value);
    }
  } catch (error) {
    alert(error.message);
    return;
  }
  // Show the same check-mark feedback as the main Web UI, then
  // restore the button.
  const title = button.dataset.copyTitle || button.title;
  button.dataset.copyTitle = title;
  button.classList.add("copied");
  button.title = "Copied!";
  button.setAttribute("aria-label", "Copied!");
  button.innerHTML =
    '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"></path></svg>';
  clearTimeout(button.copyResetTimer);
  button.copyResetTimer = setTimeout(() => {
    button.classList.remove("copied");
    button.title = title;
    button.setAttribute("aria-label", title);
    button.innerHTML =
      '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 9H5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-4"></path><rect x="9" y="4" width="11" height="11" rx="1"></rect></svg>';
  }, 1200);
}

// shellQuote makes a value one word of a shell command, the same in POSIX shells, tcsh,
// and fish: it goes in single quotes, and each ' and \ goes outside them as \'
// and \\, which every one of those shells reads as the character itself. Which shell will
// read it is not known, and could be a program running in it, so the form does not depend
// on one. Control characters stay as they are; sending refuses them. shellWord leaves a
// value that needs no quotes as it is.
function shellQuote(value) {
  return "'" + String(value ?? "").replace(/['\\]/g, "'\\$&'") + "'";
}
// formatBytes says a size in binary units, as the artifact list and the Explorer's upload do.
function formatBytes(size) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return (unit ? value.toFixed(1) : String(value)) + " " + units[unit];
}
function shellWord(value) {
  value = String(value ?? "");
  return /^[A-Za-z0-9_\/.:=@%+,-]+$/.test(value) ? value : shellQuote(value);
}

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
  const inWindow = document.documentElement.hasAttribute(
    "data-terminal-window",
  );
  const openKey = "rotari-terminal-open";
  const heightKey = "rotari-terminal-height";
  const widthKey = "rotari-terminal-width";
  const placeKey = "rotari-terminal-place";
  // The line that marks where the panel is, in a picture of the window.
  const placeLines = {
    bottom: "M1.5 9.5h13",
    left: "M5.5 2.5v11",
    right: "M10.5 2.5v11",
  };
  // The bar's pictures, drawn in a 16 by 16 box: a string is a path, and an object a rectangle.
  const paneFrame = { x: 1.5, y: 2.5, width: 13, height: 11, rx: 1.5 };
  const icons = {
    clear: [
      "M2.5 4.5h8",
      "M2.5 8h8",
      "M2.5 11.5h4.5",
      "M10 10l3.5 3.5",
      "M13.5 10L10 13.5",
    ],
    closePane: [paneFrame, "M6 6l4 4", "M10 6l-4 4"],
    maximize: [
      "M9.5 2.5h4v4",
      "M13.5 2.5L9 7",
      "M6.5 13.5h-4v-4",
      "M2.5 13.5L7 9",
    ],
    restore: [
      "M13.5 2.5L9.5 6.5",
      "M9.5 3v3.5H13",
      "M2.5 13.5l4-4",
      "M6.5 13V9.5H3",
    ],
    hide: ["M3.5 12.5h9"],
    rename: ["M10 3.5l2.5 2.5L6 12.5H3.5V10z", "M8.5 5l2.5 2.5"],
    endShell: ["M4.5 4.5l7 7", "M11.5 4.5l-7 7"],
    // A play mark: run something here.
    launch: ["M5 3.5l7.5 4.5L5 12.5z"],
    overview: [
      { x: 1.5, y: 2.5, width: 5.5, height: 4.5, rx: 1 },
      { x: 9, y: 2.5, width: 5.5, height: 4.5, rx: 1 },
      { x: 1.5, y: 9, width: 5.5, height: 4.5, rx: 1 },
      { x: 9, y: 9, width: 5.5, height: 4.5, rx: 1 },
    ],
  };
  // drawIcon replaces what a button shows with a picture. Its label names it for those who
  // cannot see the picture, and its title for those who point at it.
  const drawIcon = (button, shapes) => {
    const svgNS = "http://www.w3.org/2000/svg";
    const icon = document.createElementNS(svgNS, "svg");
    icon.setAttribute("viewBox", "0 0 16 16");
    icon.setAttribute("aria-hidden", "true");
    for (const shape of shapes) {
      const element = document.createElementNS(
        svgNS,
        typeof shape === "string" ? "path" : "rect",
      );
      if (typeof shape === "string") element.setAttribute("d", shape);
      else
        for (const [name, value] of Object.entries(shape))
          element.setAttribute(name, String(value));
      icon.append(element);
    }
    button.classList.add("terminal-icon");
    button.replaceChildren(icon);
  };
  const shownKey = "rotari-terminal-shell";
  const layoutKey = "rotari-terminal-layout";
  // What a browser tab's panes showed for each project, which goes to its window and back.
  const isPaneKey = (key) =>
    typeof key === "string" &&
    (key.startsWith(shownKey + ":") || key.startsWith(layoutKey + ":"));
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
        if (isPaneKey(key) && typeof value === "string")
          sessionStorage.setItem(key, value);
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
  // Lines each pane keeps: room for the default replay of lines of 26 bytes or more,
  // at about 12 bytes a cell, some 24 MB for a 200-column pane when full.
  const terminalScrollback = 10000;
  const shellsPath = "/_terminal/shells";
  const projectsPath = "/_terminal/projects";
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
  let panel,
    panesBox,
    groups,
    tabs,
    add,
    launchButton,
    splitRightButton,
    splitDownButton,
    closePaneButton,
    placeButton,
    maximize;
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
    heldResize: null, // The last size a drag gave it, for its shell when the drag ends.
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
  const readingOrder = () =>
    columns.flatMap((column) => column.rows.map((row) => row.pane));
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
    Math.max(
      minHeight,
      Math.min(height, Math.max(minHeight, innerHeight - minHeight)),
    );
  // The page beside a panel needs its sidebar, which someone can resize, and room for the
  // rest. Before the page exists there is no sidebar to measure, so this runs again then.
  const pageWidthNeeded = () =>
    (document.querySelector(".sidebar")?.offsetWidth || 0) + minContentWidth;
  const clampWidth = (width) =>
    Math.max(
      minWidth,
      Math.min(width, Math.max(minWidth, innerWidth - pageWidthNeeded())),
    );
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
    maximize.title = maximized
      ? "Restore the panel's size"
      : "Give the panel the whole window";
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
      if (Number.isInteger(size) && size >= minFontSize && size <= maxFontSize)
        return size;
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
  const panelPlace = () =>
    inWindow || wideWindow?.matches === false ? "bottom" : chosen;
  const panelSizes = {
    height: {
      key: heightKey,
      variable: "--terminal-saved-height",
      clamp: clampHeight,
    },
    width: {
      key: widthKey,
      variable: "--terminal-saved-width",
      clamp: clampWidth,
    },
  };
  const panelAxis = () => (panelPlace() === "bottom" ? "height" : "width");
  const applySize = (axis, size) => {
    // Only an open panel takes room, which web_shell.css decides from this.
    root.style.setProperty(
      panelSizes[axis].variable,
      panelSizes[axis].clamp(size) + "px",
    );
  };
  const savedSize = (axis) => {
    try {
      return Number(localStorage.getItem(panelSizes[axis].key)) || 0;
    } catch {
      return 0;
    }
  };
  const currentSize = (axis) =>
    panel?.getBoundingClientRect()[axis] || savedSize(axis);
  const saveSize = (axis) => {
    try {
      localStorage.setItem(
        panelSizes[axis].key,
        String(Math.round(currentSize(axis))),
      );
    } catch {}
  };
  // applyPlace puts the panel where it goes, which web_shell.css decides from this. Every
  // screen fits itself to its new room, and no connection changes.
  const applyPlace = () => {
    const place = panelPlace();
    if (place === "bottom") delete root.dataset.terminalPlace;
    else root.dataset.terminalPlace = place;
    if (!panel) return;
    panel
      .querySelector(".terminal-resizer")
      ?.setAttribute(
        "aria-orientation",
        place === "bottom" ? "horizontal" : "vertical",
      );
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
    getComputedStyle(root).getPropertyValue("--font-mono").trim() ||
    "monospace";
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
      holdResizes();
      const place = panelPlace();
      const axis = panelAxis();
      const start = currentSize(axis);
      const { clientX: startX, clientY: startY } = event;
      resizer.onpointermove = ({ clientX: x, clientY: y }) => {
        const moved = {
          bottom: () => startY - y,
          left: () => x - startX,
          right: () => startX - x,
        }[place]();
        applySize(axis, start + moved);
      };
      resizer.onpointerup = resizer.onpointercancel = () => {
        resizer.onpointermove =
          resizer.onpointerup =
          resizer.onpointercancel =
            null;
        saveSize(axis);
        releaseResizes();
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
      if (!window.WebLinksAddon)
        await loadScript("/_terminal/xterm-addon-web-links.js");
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
    pane.element.querySelector(".terminal-pane-status").hidden =
      !message && !action;
  };
  const fitPanes = () => {
    for (const pane of panes) pane.fit.fit();
  };
  // While an edge is dragged, the screens follow it but their shells are not told: each
  // new size makes a shell draw its prompt again, which a screen just folded to a new width
  // shows as stray lines, and a drag goes through many sizes. Each shell hears of the last
  // one when the drag ends.
  let dragging = false;
  const holdResizes = () => {
    dragging = true;
  };
  const releaseResizes = () => {
    dragging = false;
    for (const pane of panes) {
      const held = pane.heldResize;
      pane.heldResize = null;
      if (held) send(pane, JSON.stringify({ type: "resize", ...held }));
    }
  };
  const send = (pane, data) => {
    if (pane.socket?.readyState === WebSocket.OPEN) pane.socket.send(data);
  };
  // Each project keeps its own last tab and panes, for this browser tab alone.
  const shownStorageKey = () =>
    shownKey + ":" + here.basedir + ":" + here.project;
  const layoutStorageKey = () =>
    layoutKey + ":" + here.basedir + ":" + here.project;
  // While the panel is in a window, the window decides what the panes are.
  const rememberLayout = () => {
    if (terminalWindow || awaitingWindow) return;
    try {
      if (panes[0].shown !== null)
        sessionStorage.setItem(shownStorageKey(), panes[0].shown);
      if (panes.length > 1) {
        sessionStorage.setItem(
          layoutStorageKey(),
          JSON.stringify({
            columns: columns.map((column) => ({
              share: column.share,
              rows: column.rows.map((row) => ({
                share: row.share,
                shell: row.pane.shown,
              })),
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
      Number.isFinite(share) && share > 0
        ? Math.max(0.1, Math.min(share, 1))
        : 1 / items.length,
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
        rows: column.rows
          .filter(Boolean)
          .map((row) => ({ share: row.share, shell: row.shell })),
      }))
      .filter((column) => column.rows.length);
    evenShares(layout);
    layout.forEach((column) => evenShares(column.rows));
    const seen = new Set();
    for (const row of layout.flatMap((column) => column.rows)) {
      if (
        available.some((shell) => shell.id === row.shell) &&
        !seen.has(row.shell) &&
        seen.size < maxPanes
      ) {
        seen.add(row.shell);
      } else {
        removeRow(layout, row);
      }
    }
    return layout.length ? { layout, focused: saved.focused } : null;
  };
  // The page's basedir ID, project, and run, as it would ask for them in a query. The
  // parts are still percent-encoded, so they go in as they are.
  const pagePlace = () =>
    placeOverride ||
    placeOfPath(
      inWindow ? location.pathname.slice(windowPath.length) : location.pathname,
    );
  // pageOfPath is whose shells a page shows, as the application script routes it: a
  // project's page or a run's belongs to the project; Activity and History search, which
  // cover every basedir, to no project (global); and any other page, such as All projects
  // or Recent jobs, to its basedir. mounted is the basedir's ID, empty for the server's own.
  const pageOfPath = (path) => {
    const parts = path.split("/").filter(Boolean);
    let mounted = "";
    if (parts[0] === "_basedir" && parts[1]) mounted = parts.splice(0, 2)[1];
    if (parts[0] === "project" && parts[1])
      return {
        mounted,
        project: parts[1],
        run: parts[2] === "run" ? parts[3] || "" : "",
      };
    if (parts[0] === "activity" || parts[0] === "search")
      return { global: parts[0] };
    return { mounted };
  };
  // placeOfPath is a page path's place: a shell's link gives its group's.
  const placeOfPath = (path) => {
    const page = pageOfPath(path);
    if (page.global) return "";
    // A project's page names the server's own basedir by leaving it out, and a basedir's
    // own page by an empty ID.
    const query =
      page.mounted || !page.project ? ["basedir=" + page.mounted] : [];
    if (page.project) query.push("project=" + page.project);
    if (page.run) query.push("run=" + page.run);
    return query.join("&");
  };
  const sameGroup = (shell, other) =>
    shell.basedir === other.basedir && shell.project === other.project;
  const shellsHere = () => shells.filter((shell) => sameGroup(shell, here));
  // projectName names a group: its project, its basedir for a basedir's own shells, or none.
  const projectName = (group) => group.project || group.basedir || "No project";
  // inBasedir tells apart projects of the same name in different basedirs.
  const inBasedir = (entry, entries) =>
    entry.project &&
    entries.some((item) => item !== entry && item.project === entry.project)
      ? " in " + entry.basedir
      : "";
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
    const label = (entry) =>
      (entry.unseen ? "\u2022 " : "") +
      projectName(entry) +
      inBasedir(entry, entries) +
      " (" +
      entry.count +
      ")";
    const drawn = JSON.stringify(
      entries.map((entry) => [label(entry), entry.link]),
    );
    if (drawn === groupsDrawn) return;
    groupsDrawn = drawn;
    groups.replaceChildren(
      ...entries.map((entry) => {
        const option = document.createElement("option");
        option.value =
          inWindow && entry.link ? windowPath + entry.link : entry.link;
        option.textContent = label(entry);
        return option;
      }),
    );
    groups.value = "";
  };
  // endShells ends several shells after asking once, in the words given or its own, and
  // says whether it did; a refusal is told as renameShell's is. Those shown go last and the
  // focused pane's last of all, so that moving on from one does not land on one about to end.
  const endShells = async (
    targets,
    { refused = paneRefusal, question } = {},
  ) => {
    if (!targets.length) return false;
    const noun = targets.length === 1 ? "shell" : targets.length + " shells";
    if (!(await askToEnd(targets, question || "End " + noun))) return false;
    const rank = (shell) =>
      shell.id === focused.shown ? 2 : paneShowing(shell.id) ? 1 : 0;
    const ordered = [...targets].sort((a, b) => rank(a) - rank(b));
    for (const shell of targets) endingHere.add(shell.id);
    for (const shell of ordered) {
      if (paneShowing(shell.id)) await refreshShells();
      let response;
      try {
        response = await serverFetch(
          shellsPath + "/" + encodeURIComponent(shell.id),
          { method: "DELETE" },
        );
      } catch {}
      if (!response?.ok && response?.status !== 404) {
        await refused(response, "The shell did not end", () =>
          endShells(targets, { refused, question }),
        );
        return false;
      }
    }
    if (!targets.some((shell) => paneShowing(shell.id))) await refreshShells();
    return true;
  };
  // The questions for ending a project's shells, and every shell of the server.
  const projectEndQuestion = (count, name) =>
    "End " +
    (count === 1 ? "the shell" : "the " + count + " shells") +
    " of " +
    name;
  const everyEndQuestion = (count) =>
    "End " +
    (count === 1 ? "the one shell" : "all " + count + " shells") +
    ", of every project";
  // askToEnd asks whether to end shells, saying how many processes run in them and naming a
  // few, as the shells' previews list them now; when it cannot look, it says so and asks.
  const askToEnd = async (targets, question) => {
    const pronoun = targets.length === 1 ? "it" : "them";
    const previews = await Promise.all(
      targets.map(async (shell) => {
        try {
          const response = await serverFetch(
            shellsPath + "/" + encodeURIComponent(shell.id) + "/preview",
          );
          if (response.status === 404) return { processes: [] };
          if (!response.ok)
            return {
              problem:
                (await response.text()).trim() || "status " + response.status,
            };
          const preview = await response.json();
          return Array.isArray(preview.processes)
            ? preview
            : { problem: preview.processes_error || "the server did not say" };
        } catch {
          return { problem: "the server did not answer" };
        }
      }),
    );
    const problem = previews.find((preview) => preview.problem)?.problem;
    const below = previews.flatMap((preview) => preview.processes || []);
    let detail;
    if (problem)
      detail = "What runs in " + pronoun + " is not known: " + problem + ".";
    else if (!below.length) detail = "Nothing runs in " + pronoun + ".";
    else {
      const named = [...new Set(below.map((process) => process.command))].slice(
        0,
        3,
      );
      detail =
        below.length +
        (below.length === 1 ? " process runs" : " processes run") +
        " in " +
        pronoun +
        ": " +
        named.join(", ") +
        (below.length > named.length ? ", and others" : "") +
        ".";
    }
    return confirm(question + "? " + detail);
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
      await refusal(focused, response, "The tabs were not rearranged", () =>
        arrangeShells(arranged),
      );
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
    for (const tab of tabs.children)
      tab.classList.remove("terminal-drop-before", "terminal-drop-after");
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
        event.dataTransfer.setData(
          "text/plain",
          shell.name || String(shell.number),
        );
      }
    };
    tab.ondragover = (event) => {
      if (!draggingShell) return;
      event.preventDefault();
      clearDropMarks();
      tab.classList.add(
        dropsAfter(event, tab) ? "terminal-drop-after" : "terminal-drop-before",
      );
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
      arranged.splice(
        arranged.findIndex((item) => item.id === shell.id) + (after ? 1 : 0),
        0,
        dragged,
      );
      arrangeShells(arranged);
    };
    tab.ondragend = () => {
      draggingShell = null;
      clearDropMarks();
      renderTabs();
    };
  };
  // renameShell and endShell say whether they did it. A refusal is told in the pane with
  // the keys, or as the caller says, with a way to try again.
  const paneRefusal = (response, what, retry) =>
    refusal(focused, response, what, retry);
  const renameShell = async (shell, refused = paneRefusal) => {
    const name = prompt(
      "Name for shell " + shell.number + " (empty for its number):",
      shell.name || "",
    );
    if (name === null) return false;
    let response;
    try {
      response = await serverFetch(
        shellsPath + "/" + encodeURIComponent(shell.id),
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name }),
        },
      );
    } catch {}
    if (!response?.ok) {
      await refused(response, "The tab was not renamed", () =>
        renameShell(shell, refused),
      );
      return false;
    }
    await refreshShells();
    return true;
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
      item.setAttribute(
        "role",
        current === undefined ? "menuitem" : "menuitemradio",
      );
      if (current !== undefined)
        item.setAttribute("aria-checked", String(current));
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
    panelMenu.style.left =
      Math.max(0, Math.min(x, innerWidth - size.width)) + "px";
    panelMenu.style.top =
      Math.max(0, Math.min(y, innerHeight - size.height)) + "px";
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
      [
        "Close all shells of every project",
        () => endShells(shells, { question: everyEndQuestion(shells.length) }),
        shells.length > inGroup.length,
      ],
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
      ]
        .map(([label, place]) => [
          place === "bottom" || wide
            ? label
            : label + " (needs a wider window)",
          () => choosePlace(place),
          place === "bottom" || wide,
          chosen === place,
        ])
        .concat([["Window", popOut, true, false]]),
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
    if (button)
      button.textContent = known
        ? "Terminal (" + shells.length + ")"
        : "Terminal";
  };
  const renderTabs = () => {
    showShellCount();
    renderGroups();
    const inGroup = shellsHere();
    add.disabled = inGroup.length >= groupLimit || shells.length >= shellLimit;
    updatePaneButtons();
    const drawn = JSON.stringify([
      inGroup.map((shell) => [
        shell.id,
        shell.number,
        shell.name,
        shell.dir,
        shell.unseen_output,
        shell.command_finished,
      ]),
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
        // A command that ended out of sight rings a bell, in place of the dot.
        const finished = Boolean(shell.command_finished) && !showing;
        tab.classList.toggle("terminal-tab-unseen", unseen && !finished);
        tab.classList.toggle("terminal-tab-finished", finished);
        tab.classList.toggle("terminal-tab-other-pane", elsewhere);
        const label = document.createElement("button");
        label.type = "button";
        label.setAttribute("role", "tab");
        label.setAttribute("aria-selected", String(shell.id === focused.shown));
        label.setAttribute(
          "aria-label",
          "Shell " +
            shell.number +
            (shell.name ? ", " + shell.name : "") +
            (finished ? ", command finished" : unseen ? ", new output" : "") +
            (elsewhere ? ", in another pane" : ""),
        );
        // The times are worded as the pointer arrives: the list's refresh redraws no tab.
        const titled = () => {
          const times = shellTimes(
            shells.find((item) => item.id === shell.id) || shell,
          );
          label.title =
            (finished ? "A command finished. " : unseen ? "New output. " : "") +
            (elsewhere ? "In another pane. " : "") +
            "Started in " +
            shell.dir +
            (times
              ? " at " +
                times.startedAt +
                ", " +
                times.startedAgo +
                ". " +
                (times.idle ? "Idle " + times.idle + "." : "Active now.")
              : "");
        };
        titled();
        label.onpointerenter = titled;
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
    showStatus(pane, "Sign in again to use the terminal.", "Reload", () =>
      location.reload(),
    );
    return true;
  };
  const refusal = async (pane, response, what, handler) => {
    if (signInGone(pane, response)) return;
    const reason = response
      ? (await response.text()).trim()
      : "the server did not answer";
    showStatus(pane, what + ": " + reason, "Try again", handler);
  };
  const shellName = (shell) => {
    const name = shell.name || "Shell " + shell.number;
    return shell.basedir ? name + " of " + projectName(shell) : name;
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
    showNote(
      (ended.length === 1 ? shellName(ended[0]) : ended.length + " shells") +
        " ended.",
      6000,
    );
  };
  // markSidebarProjects marks each project in the sidebar that has shells, and each
  // basedir's All projects that has shells of its own, which web_shell.css draws as an
  // icon on its row. Links are compared decoded, since the
  // server and the page encode a project's name differently.
  const linkPath = (href) => {
    try {
      return decodeURIComponent(
        new URL(href, location.origin).pathname,
      ).replace(/\/$/, "");
    } catch {
      return "";
    }
  };
  const markSidebarProjects = () => {
    const counts = new Map();
    for (const shell of shells) {
      if (!shell.basedir) continue; // Of no project, which no row stands for.
      const path = linkPath(shell.link);
      counts.set(path, (counts.get(path) || 0) + 1);
    }
    for (const link of document.querySelectorAll(
      "#sidebar-basedirs .sidebar-project-link",
    )) {
      const row = link.closest(".sidebar-project-row");
      if (!row || row.classList.contains("basedir-row")) continue;
      const count = counts.get(linkPath(link.getAttribute("href") || "")) || 0;
      const marked = row.dataset.terminalShells || "";
      if (marked === (count ? String(count) : "")) continue;
      if (count) {
        row.dataset.terminalShells = String(count);
        row.title =
          count + (count === 1 ? " shell" : " shells") + " in the terminal";
      } else {
        delete row.dataset.terminalShells;
        row.removeAttribute("title");
      }
    }
  };
  // The launch dialog starts a registered command, such as TensorBoard, on the directory a
  // shell is in now, as an artifact's Launch does: the commands that accept it, what one
  // would run, then the launch, and a service's Open link once it answers. The server
  // looks the directory up from the shell; the page names only the shell and the command.
  let launchDialog = null;
  let launchSerial = 0;
  const closeLaunchDialog = () => {
    if (!launchDialog) return;
    launchSerial++;
    launchDialog.remove();
    launchDialog = null;
  };
  const makeElement = (tag, className, text) => {
    const made = document.createElement(tag);
    if (className) made.className = className;
    if (text !== undefined) made.textContent = text;
    return made;
  };
  const answerOf = async (response) => {
    if (!response) return { error: "the server did not answer" };
    return response.json().catch(() => ({ error: "HTTP " + response.status }));
  };
  // launchProblem shows a refusal and how to get past it, as the server words both.
  const launchProblem = (body, error, hint) => {
    const shown = [makeElement("p", "terminal-launch-problem", error)];
    if (hint) shown.push(makeElement("p", "terminal-launch-hint", hint));
    body.replaceChildren(...shown);
  };
  const launchButtons = (...buttons) => {
    const row = makeElement("div", "terminal-launch-choices");
    row.append(...buttons.filter(Boolean));
    return row;
  };
  const textButton = (text, onclick) => {
    const button = makeElement("button", "", text);
    button.type = "button";
    button.onclick = onclick;
    return button;
  };
  const launchOpen = (launch) => {
    if (launch.mode !== "service" || launch.state !== "running") return null;
    const link = makeElement("a", "terminal-launch-open", "Open");
    link.href = launch.base_path + "/";
    link.target = "_blank";
    link.rel = "noopener";
    return link;
  };
  const showInLaunches = (id) =>
    typeof window.showLaunches === "function"
      ? textButton("Show in Launches", () => {
          closeLaunchDialog();
          window.showLaunches(id);
        })
      : null;
  const followLaunch = async (serial, body, id, name) => {
    for (;;) {
      let launch;
      try {
        const response = await serverFetch("/api/launches");
        launch = ((await response.json()).launches || []).find(
          (item) => item.id === id,
        );
      } catch (error) {
        if (serial === launchSerial)
          launchProblem(body, "Cannot follow " + name + ": " + error.message);
        return;
      }
      if (serial !== launchSerial || !launch) return;
      if (launch.state === "starting") {
        body.replaceChildren(
          makeElement(
            "p",
            "",
            "Starting " + name + "; waiting for it to answer...",
          ),
          launchButtons(showInLaunches(id)),
        );
        await new Promise((resolve) => setTimeout(resolve, 500));
        continue;
      }
      body.replaceChildren(
        makeElement(
          "p",
          launch.state === "running" ? "" : "terminal-launch-problem",
          launch.state === "running"
            ? name + " is ready."
            : name + " has " + launch.state + ".",
        ),
        launchButtons(launchOpen(launch), showInLaunches(id)),
      );
      return;
    }
  };
  const startShellLaunch = async (serial, body, shell, name, newInstance) => {
    body.replaceChildren(makeElement("p", "", "Starting..."));
    let response;
    try {
      response = await serverFetch("/api/launch", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          shell: shell.id,
          name,
          new_instance: newInstance,
        }),
      });
    } catch {}
    const answer = await answerOf(response);
    if (serial !== launchSerial) return;
    if (!response?.ok) {
      launchProblem(body, answer.error, answer.hint);
      if (answer.launch)
        body.append(launchButtons(showInLaunches(answer.launch.id)));
      return;
    }
    if (answer.reused) {
      body.replaceChildren(
        makeElement("p", "", name + " is already running here."),
        launchButtons(
          launchOpen(answer),
          showInLaunches(answer.id),
          textButton("Start another instance", () =>
            startShellLaunch(serial, body, shell, name, true),
          ),
        ),
      );
      return;
    }
    if (answer.mode === "service") {
      await followLaunch(serial, body, answer.id, name);
      return;
    }
    body.replaceChildren(
      makeElement("p", "", "Started " + name + "."),
      launchButtons(showInLaunches(answer.id)),
    );
  };
  const previewShellLaunch = async (serial, body, shell, name, back) => {
    body.replaceChildren(makeElement("p", "", "Loading..."));
    let response;
    try {
      response = await serverFetch(
        "/api/launch-preview?shell=" +
          encodeURIComponent(shell.id) +
          "&name=" +
          encodeURIComponent(name),
      );
    } catch {}
    const answer = await answerOf(response);
    if (serial !== launchSerial) return;
    if (!response?.ok) {
      launchProblem(body, answer.error, answer.hint);
      body.append(launchButtons(textButton("Back", back)));
      return;
    }
    const labels = answer.labels || {};
    const details = makeElement("dl", "terminal-launch-details");
    for (const [label, value] of [
      ["Command", answer.name],
      ["Target", answer.target + " (" + answer.kind + ")"],
      ["Working directory", answer.cwd],
      ["Mode", answer.mode],
      ["Runs as", answer.user + "@" + answer.host],
      [
        "From",
        (labels.project ? "project " + labels.project + ", " : "") +
          "shell " +
          labels.shell,
      ],
    ]) {
      details.append(
        makeElement("dt", "", label),
        makeElement("dd", "", value),
      );
    }
    const runs = makeElement("dd");
    runs.append(
      makeElement(
        "pre",
        "terminal-launch-command",
        answer.shell + " -c " + answer.command,
      ),
    );
    details.append(makeElement("dt", "", "Runs"), runs);
    const confirmButton = textButton("Launch", () =>
      startShellLaunch(serial, body, shell, name, false),
    );
    confirmButton.className = "terminal-launch-confirm";
    body.replaceChildren(
      details,
      launchButtons(confirmButton, textButton("Back", back)),
    );
    confirmButton.focus();
  };
  const openLaunchDialog = async (shell) => {
    closeOverview();
    closeLaunchDialog();
    const serial = ++launchSerial;
    launchDialog = makeElement("div", "output-modal terminal-launch-modal");
    launchDialog.setAttribute("role", "dialog");
    launchDialog.setAttribute("aria-modal", "true");
    launchDialog.setAttribute("aria-labelledby", "terminal-launch-title");
    const done = () => {
      closeLaunchDialog();
      focused.term?.focus();
    };
    launchDialog.onclick = (event) => {
      if (event.target === launchDialog) done();
    };
    launchDialog.onkeydown = (event) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      done();
    };
    const box = makeElement("div", "output-panel terminal-launch");
    const head = makeElement("header");
    const title = makeElement(
      "strong",
      "",
      "Launch in " + (shell.name || "shell " + shell.number),
    );
    title.id = "terminal-launch-title";
    const actions = makeElement("div", "modal-actions");
    actions.append(textButton("Close", done));
    head.append(title, actions);
    const body = makeElement(
      "div",
      "terminal-launch-body",
      "Finding where the shell is...",
    );
    box.append(head, body);
    launchDialog.append(box);
    document.body.append(launchDialog);
    actions.firstElementChild.focus();
    let response;
    try {
      response = await serverFetch(
        shellsPath + "/" + encodeURIComponent(shell.id) + "/preview",
      );
    } catch {}
    if (serial !== launchSerial) return;
    if (!response?.ok) {
      launchProblem(
        body,
        "Where the shell is could not be found: " +
          (response
            ? (await response.text()).trim()
            : "the server did not answer"),
      );
      return;
    }
    const where = await response.json();
    if (serial !== launchSerial) return;
    if (!where.cwd) {
      launchProblem(
        body,
        "Where the shell is now is not known: " +
          (where.cwd_error || "the server did not say"),
      );
      return;
    }
    title.textContent = "Launch in " + where.cwd;
    const listCommands = async () => {
      body.replaceChildren(makeElement("p", "", "Listing the commands..."));
      let answer;
      let listing;
      try {
        listing = await serverFetch(
          "/api/launchers?kind=directory&name=" +
            encodeURIComponent(
              where.cwd.split("/").filter(Boolean).pop() || "/",
            ),
        );
      } catch {}
      answer = await answerOf(listing);
      if (serial !== launchSerial) return;
      if (!listing?.ok) {
        launchProblem(body, answer.error, answer.hint);
        return;
      }
      // The Web UI's own viewers, such as the Table viewer, run no command, so they are
      // offered without launching: the server pins the shell's directory under a source
      // the viewer's page names, and the page opens in a new tab.
      const viewers = answer.viewers || [];
      const commands = answer.available ? answer.commands || [] : [];
      if (!viewers.length && !commands.length) {
        launchProblem(
          body,
          answer.available
            ? (answer.unmatched || {}).directory ||
                "No registered command accepts a directory."
            : answer.reason,
          answer.available ? undefined : answer.hint,
        );
        return;
      }
      const shown = [];
      if (viewers.length) {
        let pinned;
        let pinnedAnswer;
        try {
          pinned = await serverFetch("/api/tableview-sources", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ shell: shell.id }),
          });
        } catch {}
        pinnedAnswer = await answerOf(pinned);
        if (serial !== launchSerial) return;
        if (!pinned?.ok) {
          shown.push(
            makeElement(
              "p",
              "terminal-launch-problem",
              "Cannot open a viewer here: " + pinnedAnswer.error,
            ),
          );
        } else {
          const list = makeElement(
            "div",
            "terminal-launch-commands terminal-launch-viewers",
          );
          for (const viewer of viewers) {
            const link = makeElement(
              "a",
              "terminal-launch-viewer",
              viewer.name,
            );
            link.href =
              viewer.path +
              "?source=" +
              encodeURIComponent(pinnedAnswer.source);
            link.target = "_blank";
            link.rel = "noopener";
            link.title = "Open " + pinnedAnswer.dir + " in a new tab";
            link.onclick = () => done();
            list.append(link);
          }
          shown.push(
            makeElement(
              "p",
              "terminal-launch-intro",
              "View " + pinnedAnswer.dir + " in this server:",
            ),
            list,
          );
        }
      }
      if (commands.length) {
        const list = makeElement("div", "terminal-launch-commands");
        for (const command of commands) {
          const button = textButton(command.name, () =>
            previewShellLaunch(serial, body, shell, command.name, listCommands),
          );
          button.disabled = Boolean(command.unavailable);
          if (command.unavailable)
            button.title = "Not installed: " + command.unavailable;
          list.append(button);
        }
        shown.push(
          makeElement(
            "p",
            "terminal-launch-intro",
            "Run a registered command on this directory, as you, on " +
              (answer.user || "this user") +
              "@" +
              (answer.host || "this host") +
              ":",
          ),
          list,
        );
      } else if (!answer.available && answer.reason) {
        shown.push(makeElement("p", "terminal-launch-hint", answer.reason));
        if (answer.hint)
          shown.push(makeElement("p", "terminal-launch-hint", answer.hint));
      }
      body.replaceChildren(...shown);
      body
        .querySelector("a.terminal-launch-viewer, button:not(:disabled)")
        ?.focus();
    };
    await listCommands();
  };
  // The overview shows every shell of the server at once, grouped by project with this
  // page's first, to choose one from, in a dialog over the page as a log is. Choosing one
  // closes it. It is a still view, taken when it opens: the list refreshing meanwhile
  // leaves it alone, and Refresh takes it again.
  let overview = null;
  let overviewButton;
  let overviewSerial = 0;
  const closeOverview = () => {
    if (!overview) return;
    overviewSerial++;
    overview.remove();
    overview = null;
    overviewButton?.setAttribute("aria-pressed", "false");
  };
  // preferShell has the focused pane of a shell's project show it when that project's page
  // next begins in this browser tab, in the layout it had.
  const preferShell = (shell) => {
    const suffix = ":" + shell.basedir + ":" + shell.project;
    try {
      sessionStorage.setItem(shownKey + suffix, shell.id);
      const saved = JSON.parse(sessionStorage.getItem(layoutKey + suffix));
      const rows = Array.isArray(saved?.columns)
        ? saved.columns.flatMap((column) =>
            Array.isArray(column?.rows) ? column.rows.filter(Boolean) : [],
          )
        : [];
      if (!rows.length) return;
      if (!rows.some((row) => row.shell === shell.id)) {
        (rows.find((row) => row.shell === saved.focused) || rows[0]).shell =
          shell.id;
      }
      saved.focused = shell.id;
      sessionStorage.setItem(layoutKey + suffix, JSON.stringify(saved));
    } catch {}
  };
  // A shell of this page's project is shown as choosing its tab would; another project's
  // takes the page there, as the project list does.
  const chooseFromOverview = (shell) => {
    closeOverview();
    if (sameGroup(shell, here)) {
      chooseTab(shell.id);
      return;
    }
    preferShell(shell);
    if (inWindow) {
      tellOpener({ type: "rotari-terminal-go", path: shell.link });
      location.assign(windowPath + shell.link);
    } else {
      location.assign(shell.link);
    }
  };
  // Where a shell is now comes from the server for each card. When it cannot say, the card
  // shows where the shell started, and says that is what it is.
  // A refusal in the overview is told at its top, where the cards are, rather than in a
  // pane under the dialog.
  let overviewProblem = null;
  const overviewRefusal = async (response, what) => {
    const reason = response
      ? (await response.text()).trim()
      : "the server did not answer";
    if (!overviewProblem) return;
    overviewProblem.hidden = false;
    overviewProblem.textContent =
      what + ": " + (reason || "status " + response.status);
  };
  // Renaming keeps the card with the keys; ending moves them to the card after it.
  const renameFromOverview = async (shell) => {
    if (await renameShell(shell, overviewRefusal)) drawOverview(shell.id);
  };
  // A project's + starts another shell there, where its page would, and shows it in no
  // pane: the overview draws it as a card, with the keys on it.
  const startFromOverview = async (entry) => {
    let response;
    try {
      const place = placeOfPath(entry.link);
      response = await serverFetch(
        shellsPath +
          "?rows=" +
          (focused.term?.rows || 24) +
          "&cols=" +
          (focused.term?.cols || 80) +
          (place && "&" + place),
        { method: "POST" },
      );
    } catch {}
    if (!response?.ok) {
      await overviewRefusal(response, "The shell did not start");
      return;
    }
    const started = await response.json();
    drawOverview(started.id);
  };
  // drawOverviewNew offers, under the cards, a shell where there is none yet: every basedir
  // the server knows and each of its projects that has no shell, and no project at all,
  // which starts in the home directory.
  const drawOverviewNew = async (serial, listed) => {
    const row = document.createElement("div");
    row.className = "terminal-overview-new";
    const label = document.createElement("label");
    label.textContent = "New shell in ";
    const choice = document.createElement("select");
    choice.setAttribute("aria-label", "Project for a new shell");
    choice.disabled = true;
    label.append(choice);
    const start = document.createElement("button");
    start.type = "button";
    start.textContent = "Start";
    start.disabled = true;
    const said = document.createElement("span");
    said.className = "terminal-overview-new-note";
    said.textContent = "Listing the projects...";
    row.append(label, start, said);
    overview?.firstElementChild.append(row);
    const reason = async (response) =>
      response
        ? (await response.text()).trim() || "status " + response.status
        : "the server did not answer";
    let response;
    try {
      response = await serverFetch("/api/basedirs");
    } catch {}
    if (serial !== overviewSerial) return;
    if (!response?.ok) {
      said.textContent =
        "The projects could not be listed: " + (await reason(response));
      return;
    }
    const basedirs = (await response.json()).basedirs || [];
    const lists = await Promise.all(
      basedirs.map(async (basedir) => {
        let answer;
        try {
          answer = await serverFetch(
            "/api/projects?basedir_id=" + encodeURIComponent(basedir.id),
          );
        } catch {}
        return answer?.ok
          ? { basedir, projects: (await answer.json()).projects || [] }
          : { basedir, problem: await reason(answer) };
      }),
    );
    if (serial !== overviewSerial) return;
    const hasShells = (basedir, project) =>
      listed.some(
        (entry) => entry.basedir === basedir && entry.project === project,
      );
    const option = (text, link) => {
      const item = document.createElement("option");
      item.value = link;
      item.textContent = text;
      return item;
    };
    const options = [];
    if (!hasShells("", ""))
      options.push(option("No project (home directory)", "/activity"));
    for (const { basedir, projects } of lists) {
      const prefix = basedir.current ? "" : "/_basedir/" + basedir.id;
      // The basedir's own shells, which its All projects shows.
      const items = hasShells(basedir.path, "")
        ? []
        : [option("All projects (the basedir)", prefix + "/")];
      for (const project of projects || []) {
        if (!hasShells(basedir.path, project))
          items.push(
            option(project, prefix + "/project/" + encodeURIComponent(project)),
          );
      }
      if (!items.length) continue;
      if (basedirs.length > 1) {
        const group = document.createElement("optgroup");
        group.label = basedir.path;
        group.append(...items);
        options.push(group);
      } else options.push(...items);
    }
    const problems = lists
      .filter((item) => item.problem)
      .map((item) => item.basedir.path + ": " + item.problem);
    choice.replaceChildren(...options);
    const full =
      shells.length >= shellLimit
        ? "at most " + shellLimit + " shells can run at once"
        : "";
    choice.disabled = !options.length;
    start.disabled = !options.length || Boolean(full);
    said.textContent = [
      !options.length && !problems.length
        ? "Every project has a shell; use its + for another."
        : "",
      full ? "No more: " + full + "." : "",
      problems.length ? "Not listed: " + problems.join("; ") : "",
    ]
      .filter(Boolean)
      .join(" ");
    start.onclick = () => startFromOverview({ link: choice.value });
  };
  const endFromOverview = async (shell) => {
    const cards = [
      ...(overview?.querySelectorAll(".terminal-overview-card") || []),
    ];
    const at = cards.findIndex((item) => item.dataset.shell === shell.id);
    const next = cards[at + 1] || cards[at - 1];
    if (await endShell(shell, overviewRefusal))
      drawOverview(next?.dataset.shell);
  };
  const overviewCard = (shell, serial) => {
    const card = document.createElement("button");
    card.type = "button";
    card.className = "terminal-overview-card";
    card.dataset.shell = shell.id;
    const ours = sameGroup(shell, here);
    const current = ours && focused.shown === shell.id;
    const elsewhere = ours && !current && Boolean(paneShowing(shell.id));
    const unseen =
      Boolean(shell.unseen_output) && !(ours && paneShowing(shell.id));
    const finished =
      Boolean(shell.command_finished) && !(ours && paneShowing(shell.id));
    card.classList.toggle("terminal-overview-current", current);
    card.classList.toggle("terminal-overview-unseen", unseen && !finished);
    card.classList.toggle("terminal-overview-finished", finished);
    const name = document.createElement("span");
    name.className = "terminal-overview-name";
    name.textContent = shell.name || "Shell " + shell.number;
    const state = document.createElement("span");
    state.className = "terminal-overview-state";
    state.textContent = [
      current && "shown",
      elsewhere && "in another pane",
      finished ? "command finished" : unseen && "new output",
    ]
      .filter(Boolean)
      .join(", ");
    const top = document.createElement("span");
    top.className = "terminal-overview-top";
    top.append(name, state);
    const dir = document.createElement("span");
    dir.className = "terminal-overview-dir";
    const path = document.createElement("bdi");
    path.textContent = "…";
    dir.append(path);
    // What the shell runs in the foreground, if anything; nothing at its prompt.
    const running = document.createElement("span");
    running.className = "terminal-overview-command";
    running.hidden = true;
    const times = shellTimes(shell);
    const when = document.createElement("span");
    when.className = "terminal-overview-times";
    when.hidden = !times;
    if (times)
      when.textContent =
        "started " +
        times.startedAgo +
        " \u00b7 " +
        (times.idle ? "idle " + times.idle : "active now");
    card.append(top, dir, when);
    card.onclick = () => chooseFromOverview(shell);
    // Under the card, outside its button: what runs in the foreground, and how many
    // processes the shell has started, which open into their tree.
    const run = document.createElement("div");
    run.className = "terminal-overview-run";
    run.hidden = true;
    const count = document.createElement("button");
    count.type = "button";
    count.className = "terminal-overview-processes";
    count.hidden = true;
    count.setAttribute("aria-expanded", "false");
    const tree = document.createElement("ul");
    tree.className = "terminal-overview-process-list";
    tree.hidden = true;
    count.onclick = () => {
      tree.hidden = !tree.hidden;
      count.setAttribute("aria-expanded", String(!tree.hidden));
    };
    run.append(running, count);
    const label = shell.name
      ? "shell " + shell.number + ", " + shell.name
      : "shell " + shell.number;
    const rename = document.createElement("button");
    rename.type = "button";
    rename.setAttribute("aria-label", "Rename " + label);
    rename.title = "Rename this tab";
    drawIcon(rename, icons.rename);
    rename.onclick = () => renameFromOverview(shell);
    const end = document.createElement("button");
    end.type = "button";
    end.setAttribute("aria-label", "End " + label);
    end.title = "End this shell, and the processes running in it";
    drawIcon(end, icons.endShell);
    end.onclick = () => endFromOverview(shell);
    const launchHere = document.createElement("button");
    launchHere.type = "button";
    launchHere.setAttribute("aria-label", "Launch in " + label);
    launchHere.title =
      "Launch a registered command, such as TensorBoard, in the directory this shell is in now";
    drawIcon(launchHere, icons.launch);
    launchHere.onclick = () => openLaunchDialog(shell);
    const actions = document.createElement("span");
    actions.className = "terminal-overview-actions";
    actions.append(launchHere, rename, end);
    const item = document.createElement("div");
    item.className = "terminal-overview-item";
    item.append(card, run, tree, actions);
    const started = (reason) => {
      if (serial !== overviewSerial) return;
      path.textContent = "started in " + shell.dir;
      dir.classList.add("terminal-overview-dir-unknown");
      dir.title = "Where it is now is not known: " + reason;
    };
    serverFetch(shellsPath + "/" + encodeURIComponent(shell.id) + "/preview")
      .then(async (response) => {
        if (!response.ok) {
          started(
            (await response.text()).trim() || "status " + response.status,
          );
          return;
        }
        const preview = await response.json();
        if (serial !== overviewSerial) return;
        if (preview.command) {
          running.hidden = false;
          running.textContent = preview.command;
          running.title = "Running " + preview.command;
        } else if (preview.command_error) {
          running.hidden = false;
          running.textContent = "running program not known";
          running.classList.add("terminal-overview-command-unknown");
          running.title = preview.command_error;
        }
        const below = preview.processes;
        if (Array.isArray(below) && below.length) {
          const noun = below.length === 1 ? " process" : " processes";
          count.hidden = false;
          // With nothing in the foreground, they run in the background.
          count.textContent = running.hidden
            ? below.length + " background" + noun
            : below.length + noun;
          count.title = "Show the processes this shell has started";
          tree.replaceChildren(
            ...below.map((process) => {
              const line = document.createElement("li");
              line.style.paddingLeft = (process.depth - 1) * 14 + "px";
              const pid = document.createElement("span");
              pid.className = "terminal-overview-pid";
              pid.textContent = String(process.pid);
              const command = document.createElement("span");
              command.textContent = process.command;
              line.title = process.command;
              line.append(pid, command);
              return line;
            }),
          );
        } else if (preview.processes_error) {
          count.hidden = false;
          count.disabled = true;
          count.textContent = "processes not known";
          count.title = preview.processes_error;
        }
        run.hidden = running.hidden && count.hidden;
        if (!preview.cwd) {
          started(preview.cwd_error || "the server did not say");
          return;
        }
        path.textContent = preview.cwd;
        dir.title = preview.cwd;
      })
      .catch(() => started("the server did not answer"));
    return item;
  };
  // drawProjectStates says under each project's name what its last run is doing: its
  // lifecycle, when, and its jobs in progress, pending, and failed, counted as its run
  // page's Jobs card counts them. The line opens that run's page. The projects are read
  // once each time the overview is drawn, not with the list of shells: reading runs is
  // far heavier.
  const projectJobGroups = [
    ["run", "in progress"],
    ["pending", "pending"],
    ["bad", "failed"],
  ];
  const drawProjectStates = async (serial, body) => {
    const lines = [
      ...body.querySelectorAll(".terminal-overview-project-state"),
    ].filter((line) => !line.hidden);
    if (!lines.length) return;
    let response;
    try {
      response = await serverFetch(projectsPath);
    } catch {}
    if (serial !== overviewSerial) return;
    if (!response?.ok) {
      const reason = response
        ? (await response.text()).trim() || "status " + response.status
        : "the server did not answer";
      if (serial !== overviewSerial) return;
      for (const line of lines) {
        line.classList.add("terminal-overview-project-state-error");
        line.textContent = "Its state could not be read: " + reason;
      }
      return;
    }
    const listed = await response.json();
    if (serial !== overviewSerial) return;
    if (Number.isFinite(Date.parse(listed.now)))
      serverClock = Date.parse(listed.now) - Date.now();
    for (const line of lines) {
      const project = listed.projects.find(
        (item) =>
          item.basedir === line.dataset.basedir &&
          item.project === line.dataset.project,
      );
      line.replaceChildren();
      if (!project) {
        line.classList.add("terminal-overview-project-state-error");
        line.textContent =
          "Its state could not be read: the server did not list it";
      } else if (project.status_error) {
        line.classList.add("terminal-overview-project-state-error");
        line.textContent =
          "Its state could not be read: " + project.status_error;
      } else if (!project.last_run) {
        line.textContent = "No runs yet";
      } else {
        line.append(projectStateButton(project.last_run, line.dataset.link));
      }
    }
  };
  // The line is a button when the project has a page to go to: a shell of a basedir no
  // longer registered has none, and its line only tells.
  const projectStateButton = (run, link) => {
    const opens = link.includes("/project/");
    const button = document.createElement(opens ? "button" : "span");
    if (opens) button.type = "button";
    button.className =
      "terminal-overview-project-state-run tone-" + (run.tone || "off");
    // An ended run says when it ended, "Last run failed 2h ago"; one still going says
    // when it started, "Last run running, started 12m ago".
    const ago = (stamp) => {
      const age = ageOf(stamp);
      return age === null
        ? ""
        : age < 60000
          ? "just now"
          : spanText(age) + " ago";
    };
    const ended = ago(run.finished_at);
    const started = ago(run.started_at);
    const lifecycle = document.createElement("span");
    lifecycle.className = "terminal-overview-project-state-lifecycle";
    lifecycle.textContent =
      "Last run " +
      run.lifecycle +
      (ended ? " " + ended : started ? ", started " + started : "");
    button.append(lifecycle);
    const counts = [];
    for (const [group, label] of projectJobGroups) {
      const count = (run.jobs && run.jobs[group]) || 0;
      const part = document.createElement("span");
      part.className =
        "terminal-overview-project-state-count" +
        (count ? " tone-" + group : " terminal-overview-project-state-none");
      part.textContent = count + " " + label;
      button.append(" \u00b7 ", part);
      counts.push(part.textContent);
    }
    const runName = run.run_name
      ? run.run_name + " (" + run.run_id + ")"
      : run.run_id;
    if (!opens) {
      button.title = "Run " + runName;
      return button;
    }
    button.title = "Open run " + runName;
    button.setAttribute(
      "aria-label",
      lifecycle.textContent +
        ", " +
        counts.join(", ") +
        ". Open run " +
        runName,
    );
    button.onclick = () => {
      const path = link + "/run/" + encodeURIComponent(run.run_id);
      closeOverview();
      if (inWindow) {
        tellOpener({ type: "rotari-terminal-go", path });
        location.assign(windowPath + path);
      } else {
        location.assign(path);
      }
    };
    return button;
  };
  const drawOverview = async (focusOn) => {
    const serial = ++overviewSerial;
    const head = document.createElement("header");
    const title = document.createElement("strong");
    title.id = "terminal-overview-title";
    title.textContent = "All shells";
    const actions = document.createElement("div");
    actions.className = "modal-actions";
    const refresh = document.createElement("button");
    refresh.type = "button";
    refresh.textContent = "Refresh";
    refresh.onclick = () => drawOverview();
    const close = document.createElement("button");
    close.type = "button";
    close.textContent = "Close";
    close.onclick = () => {
      closeOverview();
      focused.term?.focus();
    };
    const endEvery = document.createElement("button");
    endEvery.type = "button";
    endEvery.textContent = "End all shells";
    endEvery.title = "End every shell of every project, after asking";
    endEvery.disabled = true;
    endEvery.onclick = async () => {
      const every = [...shells];
      if (
        await endShells(every, {
          refused: overviewRefusal,
          question: everyEndQuestion(every.length),
        })
      )
        drawOverview();
    };
    actions.append(endEvery, refresh, close);
    head.append(title, actions);
    overviewProblem = document.createElement("p");
    overviewProblem.className = "terminal-overview-problem";
    overviewProblem.setAttribute("role", "alert");
    overviewProblem.hidden = true;
    const body = document.createElement("div");
    body.className = "terminal-overview-body";
    body.textContent = "Listing the shells...";
    overview.firstElementChild.replaceChildren(head, overviewProblem, body);
    const { ok, response } = await refreshShells();
    if (serial !== overviewSerial) return;
    if (!ok) {
      const reason = response
        ? (await response.text()).trim()
        : "the server did not answer";
      if (serial === overviewSerial)
        body.textContent = "The shells could not be listed: " + reason;
      return;
    }
    const entries = [{ ...here, shells: [] }];
    for (const shell of shells) {
      let entry = entries.find((item) => sameGroup(item, shell));
      if (!entry)
        entries.push(
          (entry = {
            basedir: shell.basedir,
            project: shell.project,
            link: shell.link,
            shells: [],
          }),
        );
      entry.link = shell.link;
      entry.shells.push(shell);
    }
    const listed = entries.filter((entry) => entry.shells.length);
    endEvery.disabled = !shells.length;
    if (!listed.length) {
      body.textContent = "No shells are running.";
      drawOverviewNew(serial, listed);
      return;
    }
    body.replaceChildren(
      ...listed.map((entry) => {
        const group = document.createElement("section");
        group.className = "terminal-overview-group";
        const heading = document.createElement("h3");
        const name = projectName(entry) + inBasedir(entry, listed);
        const named = document.createElement("span");
        named.textContent = name;
        const more = document.createElement("button");
        more.type = "button";
        more.className = "terminal-overview-add";
        more.setAttribute("aria-label", "New shell in " + name);
        more.textContent = "+";
        const full =
          entry.shells.length >= groupLimit
            ? "at most " +
              groupLimit +
              " shells can run for one " +
              (entry.basedir && !entry.project ? "basedir" : "project")
            : shells.length >= shellLimit
              ? "at most " + shellLimit + " shells can run at once"
              : "";
        more.disabled = Boolean(full);
        more.title = full
          ? "No more: " + full
          : "Start another shell here, where its page would, without showing it";
        more.onclick = () => startFromOverview(entry);
        const endAll = document.createElement("button");
        endAll.type = "button";
        endAll.className = "terminal-overview-end-all";
        endAll.textContent = "End all";
        endAll.setAttribute("aria-label", "End all shells of " + name);
        endAll.title = "End every shell of " + name + ", after asking";
        endAll.onclick = async () => {
          if (
            await endShells(entry.shells, {
              refused: overviewRefusal,
              question: projectEndQuestion(entry.shells.length, name),
            })
          )
            drawOverview();
        };
        heading.append(named, more, endAll);
        // What the project's last run is doing, filled in by drawProjectStates. Shells
        // without a project have none.
        const projectState = document.createElement("div");
        projectState.className = "terminal-overview-project-state";
        projectState.dataset.basedir = entry.basedir || "";
        projectState.dataset.project = entry.project || "";
        projectState.dataset.link = entry.link || "";
        projectState.hidden = !entry.project;
        if (entry.project) projectState.textContent = "Reading its runs...";
        const cards = document.createElement("div");
        cards.className = "terminal-overview-cards";
        cards.append(
          ...entry.shells.map((shell) => overviewCard(shell, serial)),
        );
        group.setAttribute("aria-label", name);
        group.append(heading, projectState, cards);
        return group;
      }),
    );
    drawOverviewNew(serial, listed);
    drawProjectStates(serial, body);
    const cardList = [...body.querySelectorAll(".terminal-overview-card")];
    (
      cardList.find((item) => item.dataset.shell === focusOn) ||
      body.querySelector(".terminal-overview-current") ||
      cardList[0]
    )?.focus();
  };
  // Arrows move between cards, up and down to the nearest card of the next row; Enter
  // chooses, as a button does, and Escape goes back to the shell.
  const overviewKeys = (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      closeOverview();
      focused.term?.focus();
      return;
    }
    const cards = [...overview.querySelectorAll(".terminal-overview-card")];
    const at = cards.indexOf(document.activeElement);
    if (at < 0) return;
    // F2 renames the card's shell, and Delete ends it, as in a file list.
    const shell = shells.find((item) => item.id === cards[at].dataset.shell);
    if (shell && (event.key === "F2" || event.key === "Delete")) {
      event.preventDefault();
      if (event.key === "F2") renameFromOverview(shell);
      else endFromOverview(shell);
      return;
    }
    let next;
    if (event.key === "ArrowRight") next = cards[at + 1];
    else if (event.key === "ArrowLeft") next = cards[at - 1];
    else if (event.key === "Home") next = cards[0];
    else if (event.key === "End") next = cards.at(-1);
    else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      const down = event.key === "ArrowDown";
      const from = cards[at].getBoundingClientRect();
      let best = Infinity;
      for (const card of cards) {
        const box = card.getBoundingClientRect();
        const rise = down ? box.top - from.top : from.top - box.top;
        if (rise <= 1) continue;
        const distance = rise * 1000 + Math.abs(box.left - from.left);
        if (distance < best) [best, next] = [distance, card];
      }
    } else return;
    event.preventDefault();
    next?.focus();
  };
  const toggleOverview = () => {
    if (overview) {
      closeOverview();
      focused.term?.focus();
      return;
    }
    // The page's dialog, with the panel's room left to it as a log's is.
    overview = document.createElement("div");
    overview.className = "output-modal terminal-overview-modal";
    overview.setAttribute("role", "dialog");
    overview.setAttribute("aria-modal", "true");
    overview.setAttribute("aria-labelledby", "terminal-overview-title");
    overview.onkeydown = overviewKeys;
    overview.onclick = (event) => {
      if (event.target !== overview) return;
      closeOverview();
      focused.term?.focus();
    };
    const box = document.createElement("div");
    box.className = "output-panel terminal-overview";
    overview.append(box);
    document.body.append(overview);
    overviewButton.setAttribute("aria-pressed", "true");
    drawOverview();
  };
  // A shell's times are the server's, so a page dates them by the server's clock: the list
  // says what time it is there, and serverClock is how far that is from this browser's.
  let serverClock = 0;
  const ageOf = (stamp) => {
    const at = Date.parse(stamp);
    return Number.isFinite(at)
      ? Math.max(0, Date.now() + serverClock - at)
      : null;
  };
  const spanText = (milliseconds) => {
    const minutes = Math.floor(milliseconds / 60000);
    const days = Math.floor(minutes / 1440);
    const hours = Math.floor((minutes % 1440) / 60);
    if (days) return days + "d " + hours + "h";
    if (hours) return hours + "h " + (minutes % 60) + "m";
    return minutes + "m";
  };
  // shellTimes words when a shell started and how long it has been idle, or is null when
  // the list did not say.
  const shellTimes = (shell) => {
    const started = ageOf(shell.started_at);
    const idle = ageOf(shell.active_at);
    if (started === null || idle === null) return null;
    const at = new Date(Date.parse(shell.started_at));
    return {
      startedAgo: started < 60000 ? "just now" : spanText(started) + " ago",
      startedAt:
        String(at.getHours()).padStart(2, "0") +
        ":" +
        String(at.getMinutes()).padStart(2, "0"),
      idle: idle < 60000 ? null : spanText(idle),
    };
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
      if (Number.isFinite(Date.parse(listed.now)))
        serverClock = Date.parse(listed.now) - Date.now();
      shellLimit = listed.limit;
      groupLimit = listed.group_limit;
      showShellCount();
      markSidebarProjects();
      if (panel) {
        renderTabs();
        refreshExplorer();
      }
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
      scheme +
        "//" +
        location.host +
        "/_terminal/ws?shell=" +
        encodeURIComponent(pane.shown) +
        "&rows=" +
        term.rows +
        "&cols=" +
        term.cols +
        (takeOver ? "" : "&takeover=0"),
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
      term.write(
        new Uint8Array(event.data),
        pane.pendingSend ? () => deliverSend(pane) : undefined,
      );
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
        showStatus(pane, "Another tab has this terminal.", "Use it here", () =>
          connect(pane),
        );
      } else if (event.code === closeExited) {
        shellGone(pane, "The shell ended: " + event.reason + ".");
      } else if (event.code === closeNoShell) {
        shellGone(pane, "The shell has ended.");
      } else if (event.code === closeFailed) {
        showStatus(
          pane,
          "The shell could not be shown: " + event.reason,
          "Try again",
          () => connect(pane),
        );
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
    if (pane === focused) {
      pane.term.focus();
      explorerSoon(0);
    }
  };
  // startShell starts a shell at a pane's size, and returns it, or says why not there.
  // A directory given, as the Explorer gives one, is where the shell starts, still of the
  // page's project; the server refuses one that is not a directory rather than starting
  // the shell elsewhere.
  const startShell = async (pane, again, dir = "") => {
    showStatus(pane, "Starting a shell...");
    let response;
    try {
      const place = pagePlace();
      placeOverride = "";
      response = await serverFetch(
        shellsPath +
          "?rows=" +
          pane.term.rows +
          "&cols=" +
          pane.term.cols +
          (place && "&" + place) +
          (dir && "&dir=" + encodeURIComponent(dir)),
        { method: "POST" },
      );
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
  const newShell = async (pane, dir = "") => {
    clearTimeout(pane.retryTimer);
    const shell = await startShell(pane, () => newShell(pane, dir), dir);
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
    const row = columns
      .flatMap((column) => column.rows)
      .find((item) => item.pane === pane);
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
    const layout = columns.map((column) => ({
      ...column,
      rows: column.rows.map((row) => ({ ...row })),
    }));
    const column = layout.find((item) =>
      item.rows.some((row) => row.pane === pane),
    );
    const row = column.rows.find((item) => item.pane === pane);
    if (right) {
      const added = { share: 1, pane: null };
      column.share /= 2;
      layout.splice(layout.indexOf(column) + 1, 0, {
        share: column.share,
        rows: [added],
      });
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
      ({ box }) =>
        box.w * width >= minPaneWidth && box.h * height >= minPaneHeight,
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
    let shell =
      inGroup.slice(place + 1).find((item) => !paneShowing(item.id)) || free[0];
    if (!shell) {
      if (inGroup.length >= groupLimit || shells.length >= shellLimit) {
        showBriefly(
          pane,
          "No more shells can start here, so there is nothing to split with.",
        );
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
  const endShell = async (shell, refused = paneRefusal) => {
    if (
      !(await askToEnd(
        [shell],
        "End " +
          (shell.basedir && !sameGroup(shell, here)
            ? "shell " + shell.number + " of " + projectName(shell)
            : "shell " + shell.number),
      ))
    )
      return false;
    endingHere.add(shell.id);
    let response;
    try {
      response = await serverFetch(
        shellsPath + "/" + encodeURIComponent(shell.id),
        { method: "DELETE" },
      );
    } catch {}
    if (!response?.ok && response?.status !== 404) {
      await refused(response, "The shell did not end", () =>
        endShell(shell, refused),
      );
      return false;
    }
    // A shown shell's connection closes, which moves on to another.
    if (!paneShowing(shell.id)) await refreshShells();
    return true;
  };
  const noShellMessage = () =>
    (here.basedir
      ? "No shell for " + projectName(here) + "."
      : "No shell here.") + " Open the terminal to start one.";
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
      timer: setTimeout(
        () =>
          settleSend(
            pane,
            "The terminal did not connect, so nothing was sent.",
          ),
        15000,
      ),
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
    const chosen =
      rows.find((row) => shellOf.get(row) === wanted)?.pane || pane;
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
      return refusal(pane, response, "The shells could not be listed", () =>
        begin(),
      );
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
    const inGroup = shellsHere().filter(
      (item) => item.id === pane.shown || !paneShowing(item.id),
    );
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
        here.basedir
          ? "No shell for " + projectName(here) + " yet."
          : "No shell here yet.",
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
        response = await serverFetch("/_terminal/xterm-license.txt", {
          method: "HEAD",
        });
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
  const inset = (start, hasBefore) =>
    hasBefore ? "calc(" + percent(start) + " + 3px)" : percent(start);
  const span = (size, edges) =>
    edges ? "calc(" + percent(size) + " - " + edges * 3 + "px)" : percent(size);
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
            ? {
                x: 0,
                y: across + down * column.share,
                w: 1,
                h: row.share * column.share,
              }
            : { x: across, y: down, w: column.share, h: row.share },
          before: stacked
            ? { x: 0, y: Number(c > 0 || r > 0) }
            : { x: Number(c > 0), y: Number(r > 0) },
          after: stacked
            ? { x: 0, y: Number(!lastColumn || !lastRow) }
            : { x: Number(!lastColumn), y: Number(!lastRow) },
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
    launchButton.disabled = focused.shown === null;
  };
  // applyLayout places the panes and the handles between them, by position alone: moving
  // a screen in the page, as columns of their own elements would need, would make
  // browsers forget its scrolling.
  const applyLayout = () => {
    laidStacked = isStacked();
    const { places, boundaries } = layoutGeometry(columns, laidStacked);
    for (const { row, box, before, after } of places) {
      place(row.pane.element, box, before, after);
      row.pane.element.classList.toggle(
        "terminal-pane-focused",
        row.pane === focused,
      );
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
      handle.classList.toggle(
        "terminal-split-handle-rows",
        boundary.horizontal,
      );
      handle.setAttribute(
        "aria-orientation",
        boundary.horizontal ? "horizontal" : "vertical",
      );
      handle.setAttribute(
        "aria-valuenow",
        String(
          Math.round(
            (boundary.a.share / (boundary.a.share + boundary.b.share)) * 100,
          ),
        ),
      );
      const line = "calc(" + percent(boundary.at) + " - 3px)";
      handle.style.left = boundary.horizontal ? percent(boundary.from) : line;
      handle.style.top = boundary.horizontal ? line : percent(boundary.from);
      handle.style.width = boundary.horizontal ? percent(boundary.size) : "6px";
      handle.style.height = boundary.horizontal
        ? "6px"
        : percent(boundary.size);
    });
    updatePaneButtons();
  };
  // focusPane makes a pane the one the bar and sending act on, following where the keys go.
  const focusPane = (pane) => {
    focused = pane;
    applyLayout();
    rememberLayout();
    renderTabs();
    explorerSoon(0);
  };
  // moveBoundary gives one neighbor a share, and the other what is left of the two, leaving
  // neither under the smallest pane, unless the two have less room than that between them.
  const moveBoundary = ({ a, b, horizontal, scale }, share) => {
    const both = a.share + b.share;
    const { width, height } = panesBox.getBoundingClientRect();
    const room = (horizontal ? height : width) * scale;
    const smallest = room
      ? (horizontal ? minPaneHeight : minPaneWidth) / room
      : 0.05;
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
      holdResizes();
      handle.onpointermove = (move) => {
        const box = panesBox.getBoundingClientRect();
        const { boundary } = handle;
        const part = boundary.horizontal
          ? (move.clientY - box.top) / box.height
          : (move.clientX - box.left) / box.width;
        if (Number.isFinite(part))
          moveBoundary(boundary, (part - boundary.start) / boundary.scale);
      };
      handle.onpointerup = handle.onpointercancel = () => {
        handle.onpointermove =
          handle.onpointerup =
          handle.onpointercancel =
            null;
        rememberLayout();
        releaseResizes();
        focused.term.focus();
      };
    };
    handle.onkeydown = (event) => {
      const step = {
        ArrowLeft: -0.05,
        ArrowUp: -0.05,
        ArrowRight: 0.05,
        ArrowDown: 0.05,
      }[event.key];
      if (!step) return;
      event.preventDefault();
      moveBoundary(handle.boundary, handle.boundary.a.share + step);
      rememberLayout();
    };
    return handle;
  };
  // The Explorer: a tree of the files where the focused pane's shell is, in a column at the
  // left of the panel's body, beside the panes wherever the panel is. It is open unless
  // someone closed it, which the browser remembers. While
  // Follow is on, its root is the shell's directory, looked up on the server; Up shows the
  // parent and turns Follow off. It lists only what a folder holds, a folder at a time, and
  // lists again with the panel's shells, shortly after Enter in the focused pane, and when
  // asked. Names are the server's: one that is not UTF-8 is shown but never named back.
  const explorerOpenKey = "rotari-terminal-explorer";
  const explorerWidthKey = "rotari-terminal-explorer-width";
  const explorerPathType = "application/x-rotari-explorer-path";
  const filesPath = "/_terminal/files";
  const explorerDefaultWidth = 260;
  const explorerMinWidth = 160;
  // The panes beside it keep at least this much, however wide it is dragged.
  const explorerPanesRoom = 200;
  const explorer = {
    open: false,
    follow: true,
    root: "", // The directory at the top of the tree.
    shellDir: "", // Where the focused pane's shell was when last asked.
    shellID: null,
    problem: "", // Why the tree shows nothing, such as a pane with no shell.
    expanded: new Set(),
    listings: new Map(), // A directory's answer: { entries, total } or { error }.
    selected: "",
    visible: [], // The rows shown, top to bottom, for the keys.
    serial: 0,
    soon: undefined,
    element: null,
    tree: null,
    title: null,
    where: null,
    footer: null,
    followButton: null,
    toggle: null,
    body: null,
    busy: "", // A change waiting for the server, said in the footer.
    progress: null, // How far an upload has come, from 0 to 1, for the footer's bar.
    clipboard: null, // { path, folder, cut } until pasted.
    dragging: "", // The path of a row being dragged.
    hoverTimer: undefined,
    hoverPath: "", // A closed folder held over while dragging.
  };
  try {
    explorer.open = localStorage.getItem(explorerOpenKey) !== "0";
  } catch {
    explorer.open = true;
  }
  const explorerIcons = {
    explorer: ["M1.5 3.5h4.5l1.5 1.5h6.5v8h-12.5z", "M1.5 6.5h12.5"],
    up: ["M8 13V3", "M4 7l4-4 4 4"],
    follow: [
      "M6.5 9.5l3-3",
      "M7 4.5l1.5-1.5a2.5 2.5 0 0 1 3.5 3.5L10.5 8",
      "M9 11.5L7.5 13A2.5 2.5 0 0 1 4 9.5L5.5 8",
    ],
    refresh: ["M13 8a5 5 0 1 1-1.5-3.5", "M13 2.5V5h-2.5"],
    // Folders closed into one: a box in front of another, with a minus, as editors draw it.
    collapse: [
      "M5.5 3.5V2.5h8v8h-1",
      { x: 2.5, y: 5.5, width: 8, height: 8, rx: 1 },
      "M4.5 9.5h4",
    ],
    send: ["M2.5 8h10", "M9 4.5L12.5 8 9 11.5"],
    open: [
      { x: 2, y: 2.5, width: 12, height: 11, rx: 1 },
      "M2 6h12",
      "M6 6v7.5",
    ],
    chevronRight: ["M6 4l4 4-4 4"],
    chevronDown: ["M4 6l4 4 4-4"],
    folder: ["M1.5 3.5h4.5l1.5 1.5h6.5v8h-12.5z"],
    file: ["M3.5 1.5h6l3 3v10h-9z", "M9.5 1.5v3h3"],
    newFile: ["M3.5 1.5h6l3 3v4", "M3.5 1.5v13h5", "M12 10.5v4", "M10 12.5h4"],
    newFolder: ["M8 12.5H1.5v-9h4.5l1.5 1.5h6.5v3", "M12 9.5v4", "M10 11.5h4"],
  };
  const joinPath = (dir, name) => (dir === "/" ? "/" : dir + "/") + name;
  const parentOf = (path) => path.slice(0, path.lastIndexOf("/")) || "/";
  const baseName = (path) =>
    path === "/" ? "/" : path.slice(path.lastIndexOf("/") + 1);
  const under = (path, dir) =>
    path === dir || path.startsWith(dir === "/" ? "/" : dir + "/");
  // The path that goes into the shell: relative to where the shell is, when it is there,
  // so that it finishes a command typed there, and never with "..".
  const pathFromShell = (path) => {
    const dir = explorer.shellDir;
    if (!dir || !under(path, dir)) return path;
    if (path === dir) return ".";
    return path.slice(dir === "/" ? 1 : dir.length + 1);
  };
  const explorerShell = () =>
    shells.find((shell) => shell.id === focused.shown);
  const explorerShowing = () =>
    explorer.open && panel && !panel.hidden && !document.hidden;
  // A name as text, with each control character as an escape in a mark of its own, so
  // that a line break or an escape sequence in a name shows as what it is.
  const drawName = (target, name) => {
    let plain = "";
    const flush = () => {
      if (plain) target.append(plain);
      plain = "";
    };
    for (const character of name) {
      const code = character.codePointAt(0);
      if (code < 0x20 || (code >= 0x7f && code <= 0x9f)) {
        flush();
        const mark = document.createElement("span");
        mark.className = "terminal-explorer-escape";
        mark.textContent =
          { 9: "\\t", 10: "\\n", 13: "\\r" }[code] ||
          "\\x" + code.toString(16).padStart(2, "0");
        target.append(mark);
      } else plain += character;
    }
    flush();
  };
  const unnamedNote =
    "This name is not valid UTF-8, so the Explorer cannot open, send, or copy it.";
  const listDirectory = async (query) => {
    let response;
    try {
      response = await serverFetch(filesPath + "?" + query);
    } catch {
      return { error: "The server did not answer." };
    }
    if (response.status === 401 || response.status === 403)
      return {
        error: "Sign in again to use the terminal.",
        status: response.status,
      };
    if (!response.ok)
      return {
        error:
          ((await response.text()) || "").trim() || "Status " + response.status,
        status: response.status,
      };
    return response.json();
  };
  // refreshExplorer lists what the tree shows again: the root, looked up from the shell
  // while following, and each folder open in it. A folder that cannot be listed any more
  // closes. Answers that come after a newer refresh started are dropped.
  const refreshExplorer = async () => {
    if (!explorerShowing()) return;
    const serial = ++explorer.serial;
    const shell = explorerShell();
    explorer.shellID = shell?.id ?? null;
    explorer.problem = "";
    if (!shell) {
      explorer.shellDir = "";
      if (explorer.follow) {
        explorer.problem = "This pane shows no shell.";
        renderExplorer();
        return;
      }
    } else if (explorer.follow) {
      const listed = await listDirectory(
        "shell=" + encodeURIComponent(shell.id),
      );
      if (serial !== explorer.serial) return;
      if (listed.error) {
        explorer.problem = listed.error;
        explorer.shellDir = "";
        renderExplorer();
        return;
      }
      explorer.shellDir = listed.dir;
      if (listed.dir !== explorer.root) moveExplorerRoot(listed.dir);
      explorer.listings.set(listed.dir, listed);
    } else {
      let preview;
      try {
        const response = await serverFetch(
          shellsPath + "/" + encodeURIComponent(shell.id) + "/preview",
        );
        if (response.ok) preview = await response.json();
      } catch {}
      if (serial !== explorer.serial) return;
      explorer.shellDir = preview?.cwd || "";
    }
    const wanted = (explorer.follow ? [] : [explorer.root]).concat(
      [...explorer.expanded].filter((path) => under(path, explorer.root)),
    );
    const answers = await Promise.all(
      wanted.map((path) => listDirectory("path=" + encodeURIComponent(path))),
    );
    if (serial !== explorer.serial) return;
    wanted.forEach((path, index) => {
      const answer = answers[index];
      if (answer.error && path !== explorer.root) {
        explorer.expanded.delete(path);
        explorer.listings.delete(path);
      } else explorer.listings.set(path, answer);
    });
    renderExplorer();
  };
  const explorerSoon = (delay = 400) => {
    clearTimeout(explorer.soon);
    explorer.soon = setTimeout(() => refreshExplorer(), delay);
  };
  // moveExplorerRoot shows another directory at the top, keeping the folders open under it.
  const moveExplorerRoot = (dir) => {
    explorer.root = dir;
    for (const path of [...explorer.expanded])
      if (!under(path, dir) || path === dir) explorer.expanded.delete(path);
    for (const path of [...explorer.listings.keys()])
      if (!under(path, dir)) explorer.listings.delete(path);
    if (explorer.selected && !under(explorer.selected, dir))
      explorer.selected = "";
  };
  const setFollow = (follow) => {
    explorer.follow = follow;
    explorer.followButton?.setAttribute("aria-pressed", String(follow));
    refreshExplorer();
  };
  const explorerUp = () => {
    if (!explorer.root || explorer.root === "/") return;
    const parent = parentOf(explorer.root);
    const child = explorer.root;
    explorer.follow = false;
    explorer.followButton?.setAttribute("aria-pressed", "false");
    moveExplorerRoot(parent);
    explorer.expanded.add(child);
    explorer.selected = child;
    refreshExplorer();
  };
  const toggleFolder = (path) => {
    if (explorer.expanded.has(path)) explorer.expanded.delete(path);
    else explorer.expanded.add(path);
    renderExplorer();
    if (explorer.expanded.has(path) && !explorer.listings.has(path))
      listDirectory("path=" + encodeURIComponent(path)).then((answer) => {
        explorer.listings.set(path, answer);
        renderExplorer();
      });
  };
  const openInViewer = async (path) => {
    // A tab opened only after the answer would be a pop-up to the browser, so it opens now.
    const tab = window.open("about:blank", "_blank");
    if (tab) tab.opener = null;
    let response, answer;
    try {
      response = await serverFetch("/api/tableview-sources", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path }),
      });
      answer = await answerOf(response);
    } catch {}
    if (!response?.ok || !answer?.source) {
      tab?.close();
      showNote(
        "Cannot open " +
          path +
          " in the Table viewer: " +
          (answer?.error || "the server did not answer."),
        6000,
      );
      return;
    }
    const address =
      "/tableview/?source=" +
      encodeURIComponent(answer.source) +
      (answer.child ? "&child=" + encodeURIComponent(answer.child) : "");
    // An about:blank tab has no address of its own to resolve a relative one against.
    if (tab) tab.location.href = new URL(address, location.href).href;
    else
      showNote("The browser blocked the new tab for the Table viewer.", 6000);
  };
  const sendExplorerPath = async (path) => {
    await window.sendToTerminalAndSay(pathFromShell(path), { word: true });
  };
  const copyExplorerText = async (text) => {
    try {
      if (navigator.clipboard?.writeText)
        await navigator.clipboard.writeText(text);
      else copyJobsValueFallback(text);
      showNote("Copied " + text, 2500);
    } catch (error) {
      showNote("Copy failed: " + error.message, 6000);
    }
  };
  const rowOf = (path) =>
    [...(explorer.tree?.children || [])].find(
      (row) => row.explorerPath === path,
    );
  const selectRow = (path, focus = true) => {
    explorer.selected = path;
    for (const row of explorer.tree.children) {
      if (!row.explorerPath) continue;
      const selected = row.explorerPath === path;
      row.setAttribute("aria-selected", String(selected));
      row.tabIndex = selected ? 0 : -1;
    }
    if (focus) rowOf(path)?.focus();
  };
  const explorerIconButton = (label, shapes, onclick) => {
    const button = document.createElement("button");
    button.type = "button";
    button.setAttribute("aria-label", label);
    button.title = label;
    drawIcon(button, shapes);
    button.onclick = onclick;
    return button;
  };
  const explorerNoteRow = (level, text, className, button) => {
    const row = document.createElement("div");
    row.className = "terminal-explorer-note " + (className || "");
    row.style.setProperty("--level", String(level));
    row.append(text);
    if (button) row.append(" ", button);
    return row;
  };
  const explorerRow = (entry, dir, level) => {
    const path = joinPath(dir, entry.name);
    const folder =
      entry.kind === "dir" ||
      (entry.kind === "symlink" && entry.target_kind === "dir");
    const named = !entry.not_utf8;
    const open = folder && explorer.expanded.has(path);
    const row = document.createElement("div");
    row.className = "terminal-explorer-row";
    row.explorerPath = path;
    row.setAttribute("role", "treeitem");
    row.setAttribute("aria-level", String(level + 1));
    if (folder) row.setAttribute("aria-expanded", String(open));
    row.setAttribute("aria-selected", String(explorer.selected === path));
    row.tabIndex = explorer.selected === path ? 0 : -1;
    row.style.setProperty("--level", String(level));
    const chevron = document.createElement("span");
    chevron.className = "terminal-explorer-chevron";
    if (folder)
      drawIcon(
        chevron,
        open ? explorerIcons.chevronDown : explorerIcons.chevronRight,
      );
    const icon = document.createElement("span");
    icon.className = "terminal-explorer-kind";
    drawIcon(icon, folder ? explorerIcons.folder : explorerIcons.file);
    const name = document.createElement("span");
    name.className = "terminal-explorer-name";
    drawName(name, entry.name);
    row.append(chevron, icon, name);
    if (entry.kind === "symlink") {
      row.classList.add("terminal-explorer-link");
      const target = document.createElement("span");
      target.className = "terminal-explorer-target";
      target.append("→ ");
      drawName(target, entry.target || "?");
      if (entry.target_kind === "missing") target.append(" (missing)");
      row.append(target);
    }
    if (!named) {
      row.classList.add("terminal-explorer-unnamed");
      row.title = unnamedNote;
    } else {
      row.title = path;
      const actions = document.createElement("span");
      actions.className = "terminal-explorer-actions";
      const send = explorerIconButton(
        "Send the path to the terminal",
        explorerIcons.send,
        (event) => {
          event.stopPropagation();
          sendExplorerPath(path);
        },
      );
      const view = explorerIconButton(
        "Open in the Table viewer",
        explorerIcons.open,
        (event) => {
          event.stopPropagation();
          openInViewer(path);
        },
      );
      for (const button of [send, view]) button.tabIndex = -1;
      actions.append(send, view);
      row.append(actions);
      row.draggable = true;
      row.ondragstart = (event) => {
        event.dataTransfer.setData(explorerPathType, path);
        event.dataTransfer.setData("text/plain", path);
        // Copied into a pane as its path, moved within the tree.
        event.dataTransfer.effectAllowed = "copyMove";
        explorer.dragging = path;
        // Dragged out of the browser, it downloads, where the browser knows how.
        const saved = folder ? baseName(path) + ".zip" : baseName(path);
        if (!saved.includes(":"))
          event.dataTransfer.setData(
            "DownloadURL",
            (folder ? "application/zip" : "application/octet-stream") +
              ":" +
              saved +
              ":" +
              new URL(downloadPath(path), location.href).href,
          );
      };
      row.ondragend = () => {
        explorer.dragging = "";
        clearDragMarks();
      };
      takesDrops(row, folder ? path : dir, folder && !open);
      if (explorer.clipboard?.cut && explorer.clipboard.path === path)
        row.classList.add("terminal-explorer-cut");
    }
    row.onclick = () => {
      selectRow(path);
      if (folder && named) toggleFolder(path);
    };
    row.ondblclick = () => {
      if (!folder && named) openInViewer(path);
    };
    row.oncontextmenu = (event) => {
      selectRow(path);
      if (!named) {
        event.preventDefault();
        showNote(unnamedNote, 6000);
        return;
      }
      openPanelMenu(event, [
        ["Open in the Table viewer", () => openInViewer(path), true],
        // A file's folder, as editors open a terminal for a file.
        [
          "Open in a new terminal tab",
          () => newShell(focused, folder ? path : dir),
          true,
        ],
        ["Send the path to the terminal", () => sendExplorerPath(path), true],
        ["Download", () => downloadExplorerItem(path), true],
        ["Upload...", () => chooseUploads(folder ? path : dir), true],
        ...explorerChangeChoices(path, folder),
        ["Copy the path", () => copyExplorerText(path), true],
        [
          "Copy the path from here",
          () => copyExplorerText(pathFromShell(path)),
          Boolean(explorer.shellDir) && under(path, explorer.shellDir),
        ],
      ]);
    };
    explorer.visible.push({ path, folder, named, parent: dir, level });
    return row;
  };
  // Changes to files: each is one request to the server, which refuses, with the reason,
  // anything that would replace what exists; a refusal shows as a note, and the tree is
  // listed again afterwards either way. Names are asked with prompt and deletions and
  // moves by dragging confirmed with confirm, as the panel does for its tabs and shells.
  // The clipboard is the Explorer's own: what was cut or copied, until pasted.
  const explorerHoverOpen = 700;
  const changeFiles = async (op, body) => {
    let response;
    try {
      response = await serverFetch(filesPath + "/" + op, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    } catch {}
    if (response?.ok) return (await response.json()).path;
    const reason = response
      ? ((await response.text()) || "").trim() || "status " + response.status
      : "the server did not answer";
    return { error: reason };
  };
  // explorerChange runs one change, says why it failed if it did, and lists the tree again,
  // selecting what the change made and opening the folder it went into.
  const explorerChange = async (what, op, body, into) => {
    explorer.busy = what + "...";
    renderExplorer();
    const done = await changeFiles(op, body);
    explorer.busy = "";
    if (typeof done !== "string") {
      showNote(what + " failed: " + done.error, 6000);
      renderExplorer();
      await refreshExplorer();
      return null;
    }
    if (into && into !== explorer.root) explorer.expanded.add(into);
    if (op !== "delete") explorer.selected = done;
    await refreshExplorer();
    return done;
  };
  // Where something new or pasted goes: the selected folder, a selected file's folder, or
  // the top of the tree.
  const explorerTarget = () => {
    const row = explorer.visible.find(
      (item) => item.path === explorer.selected,
    );
    if (!row) return explorer.root;
    return row.folder ? row.path : row.parent;
  };
  const newExplorerItem = async (op, into = explorerTarget()) => {
    if (!into) return;
    const kind = op === "mkdir" ? "folder" : "file";
    const name = prompt("Name of the new " + kind + " in " + into + ":", "");
    if (!name) return;
    await explorerChange(
      "Making " + name,
      op,
      { path: joinPath(into, name) },
      into,
    );
  };
  const renameExplorerItem = async (path) => {
    const name = prompt("New name for " + path + ":", baseName(path));
    if (!name || name === baseName(path)) return;
    await explorerChange("Renaming " + baseName(path), "move", {
      from: path,
      to: joinPath(parentOf(path), name),
    });
  };
  const deleteExplorerItem = async (path, folder) => {
    let question = "Delete " + path + "? This cannot be undone.";
    if (folder) {
      // One listing, for how much it holds directly; its depth is not walked.
      const listed = await listDirectory("path=" + encodeURIComponent(path));
      const held = listed.error
        ? "what it holds"
        : listed.total === 1
          ? "the 1 entry in it"
          : "the " + listed.total + " entries in it";
      question =
        "Delete the folder " +
        path +
        ", " +
        held +
        ", and everything below? This cannot be undone.";
    }
    if (!confirm(question)) return;
    if (explorer.clipboard && under(explorer.clipboard.path, path))
      explorer.clipboard = null;
    await explorerChange("Deleting " + baseName(path), "delete", { path });
  };
  const setExplorerClipboard = (path, folder, cut) => {
    explorer.clipboard = { path, folder, cut };
    renderExplorer();
    showNote(
      (cut ? "Cut " : "Copied ") + path + ". Paste it into a folder.",
      2500,
    );
  };
  const pasteExplorerItem = async (into = explorerTarget()) => {
    const held = explorer.clipboard;
    if (!held || !into) return;
    const to = joinPath(into, baseName(held.path));
    if (held.cut) {
      const moved = await explorerChange(
        "Moving " + baseName(held.path),
        "move",
        { from: held.path, to },
        into,
      );
      if (moved) explorer.clipboard = null;
      renderExplorer();
      return;
    }
    await explorerChange(
      "Copying " + baseName(held.path),
      "copy",
      { from: held.path, to, unique: parentOf(held.path) === into },
      into,
    );
  };
  // moveByDrag moves what was dragged into a folder, after asking.
  const moveByDrag = async (path, into) => {
    if (!movableInto(path, into)) return;
    if (!confirm("Move " + path + " into " + into + "?")) return;
    await explorerChange(
      "Moving " + baseName(path),
      "move",
      { from: path, to: joinPath(into, baseName(path)) },
      into,
    );
  };
  // A folder can take what is dragged unless it is already there, or is it or below it.
  const movableInto = (path, into) =>
    Boolean(path && into) && parentOf(path) !== into && !under(into, path);
  const clearDragMarks = () => {
    clearTimeout(explorer.hoverTimer);
    explorer.hoverTimer = undefined;
    explorer.hoverPath = "";
    for (const marked of explorer.tree?.querySelectorAll(
      ".terminal-explorer-drop",
    ) || [])
      marked.classList.remove("terminal-explorer-drop");
    explorer.tree?.classList.remove("terminal-explorer-drop");
  };
  // takesDrops lets a row, or the tree's empty part, take a row dragged within the tree:
  // a folder takes it, a file's row gives it to its folder, and a closed folder held over
  // opens after a moment.
  // Files dragged in from outside the page, as from the desktop, rather than a row.
  const carriesFiles = (event) =>
    !explorer.dragging &&
    [...(event.dataTransfer?.types || [])].includes("Files");
  const takesDrops = (element, folder, closedFolder) => {
    // The tree's own folder is its top, which moves with the shell.
    const target = () => (typeof folder === "function" ? folder() : folder);
    const fromTree = (event) =>
      element !== explorer.tree || event.target === explorer.tree;
    element.addEventListener("dragover", (event) => {
      const into = target();
      if (!fromTree(event) || !into) return;
      const files = carriesFiles(event);
      if (!files && !movableInto(explorer.dragging, into)) return;
      event.preventDefault();
      event.stopPropagation();
      if (event.dataTransfer)
        event.dataTransfer.dropEffect = files ? "copy" : "move";
      if (!element.classList.contains("terminal-explorer-drop")) {
        clearDragMarks();
        element.classList.add("terminal-explorer-drop");
      }
      if (closedFolder && explorer.hoverPath !== into) {
        explorer.hoverPath = into;
        explorer.hoverTimer = setTimeout(() => {
          if (explorer.hoverPath === into && !explorer.expanded.has(into))
            toggleFolder(into);
        }, explorerHoverOpen);
      }
    });
    element.addEventListener("dragleave", (event) => {
      const into = target();
      if (event.target === element) {
        element.classList.remove("terminal-explorer-drop");
        if (explorer.hoverPath === into) {
          clearTimeout(explorer.hoverTimer);
          explorer.hoverPath = "";
        }
      }
    });
    element.addEventListener("drop", (event) => {
      if (!fromTree(event)) return;
      const files = carriesFiles(event);
      if (!files && !explorer.dragging) return;
      event.preventDefault();
      event.stopPropagation();
      clearDragMarks();
      if (files) {
        uploadDropped(event.dataTransfer, target());
        return;
      }
      const path = explorer.dragging;
      explorer.dragging = "";
      moveByDrag(path, target());
    });
  };
  // Downloads and uploads. A folder downloads as a zip the server writes as it reads, and
  // a file as it is; the browser shows the progress. Uploads go one file at a time, each
  // refused rather than replacing a file of its name.
  const downloadPath = (path) =>
    filesPath + "/download?path=" + encodeURIComponent(path);
  const downloadExplorerItem = (path) => {
    const link = document.createElement("a");
    link.href = downloadPath(path);
    link.download = "";
    document.body.append(link);
    link.click();
    link.remove();
  };
  // sendUpload posts one file, saying how much has gone as it goes, which fetch cannot.
  // It answers the status and text, or null when the connection failed.
  const sendUpload = (address, file, progress) =>
    new Promise((resolve) => {
      const request = new XMLHttpRequest();
      request.open("POST", address);
      request.setRequestHeader("Content-Type", "application/octet-stream");
      request.upload.onprogress = (event) => {
        if (event.lengthComputable) progress(event.loaded, event.total);
      };
      request.onload = () =>
        resolve({
          ok: request.status >= 200 && request.status < 300,
          status: request.status,
          text: request.responseText,
        });
      request.onerror = request.onabort = () => resolve(null);
      request.send(file);
    });
  // showExplorerBusy says in the footer what waits, with a bar for how far it has come,
  // without drawing the tree again for each step.
  const showExplorerBusy = (text, done) => {
    explorer.busy = text;
    explorer.progress = done;
    if (!explorer.footer) return;
    explorer.footer.replaceChildren(text);
    if (done !== null && done !== undefined) {
      const bar = document.createElement("progress");
      bar.className = "terminal-explorer-progress";
      bar.max = 1;
      bar.value = done;
      explorer.footer.append(bar);
    }
    explorer.footer.title = text;
  };
  const uploadFiles = async (files, into) => {
    if (!files.length || !into) return;
    const failed = [];
    for (const [index, file] of files.entries()) {
      const which =
        "Uploading " +
        file.name +
        (files.length > 1 ? ", " + (index + 1) + " of " + files.length : "");
      showExplorerBusy(which + "...", 0);
      const answer = await sendUpload(
        filesPath +
          "/upload?dir=" +
          encodeURIComponent(into) +
          "&name=" +
          encodeURIComponent(file.name),
        file,
        (sent, total) =>
          showExplorerBusy(
            which +
              ": " +
              Math.floor((sent / total) * 100) +
              "% (" +
              formatBytes(sent) +
              " of " +
              formatBytes(total) +
              ")",
            sent / total,
          ),
      );
      if (!answer?.ok)
        failed.push(
          file.name +
            ": " +
            (answer
              ? (answer.text || "").trim() || "status " + answer.status
              : "the server did not answer"),
        );
    }
    explorer.progress = null;
    explorer.busy = "";
    const uploaded = files.length - failed.length;
    if (failed.length)
      showNote(
        (uploaded
          ? "Uploaded " + uploaded + " of " + files.length + ". "
          : "") +
          "Not uploaded: " +
          failed.join("; "),
        8000,
      );
    else
      showNote(
        "Uploaded " +
          (files.length === 1 ? files[0].name : files.length + " files") +
          " to " +
          into +
          ".",
        2500,
      );
    if (into !== explorer.root) explorer.expanded.add(into);
    await refreshExplorer();
  };
  // A dropped folder is not uploaded: only the files dropped with it are.
  const uploadDropped = (transfer, into) => {
    const folders = new Set(
      [...(transfer.items || [])]
        .map((item) => item.webkitGetAsEntry?.())
        .filter((entry) => entry?.isDirectory)
        .map((entry) => entry.name),
    );
    const files = [...(transfer.files || [])].filter(
      (file) => !folders.has(file.name),
    );
    if (folders.size)
      showNote(
        "Folders are not uploaded: " +
          [...folders].join(", ") +
          (files.length ? ". The files are." : "."),
        6000,
      );
    uploadFiles(files, into);
  };
  const chooseUploads = (into) => {
    const chooser = document.createElement("input");
    chooser.type = "file";
    chooser.multiple = true;
    chooser.hidden = true;
    chooser.onchange = () => {
      const files = [...chooser.files];
      chooser.remove();
      uploadFiles(files, into);
    };
    document.body.append(chooser);
    chooser.click();
  };
  // The menu of a row's changes, after its other choices.
  const explorerChangeChoices = (path, folder) => [
    [
      "New File...",
      () => newExplorerItem("create", folder ? path : parentOf(path)),
      true,
    ],
    [
      "New Folder...",
      () => newExplorerItem("mkdir", folder ? path : parentOf(path)),
      true,
    ],
    ["Cut", () => setExplorerClipboard(path, folder, true), true],
    ["Copy", () => setExplorerClipboard(path, folder, false), !folder],
    [
      "Paste",
      () => pasteExplorerItem(folder ? path : parentOf(path)),
      Boolean(explorer.clipboard),
    ],
    ["Rename...", () => renameExplorerItem(path), true],
    ["Delete...", () => deleteExplorerItem(path, folder), true],
  ];
  // drawEntries puts a directory's rows into the tree, and below each open folder its own.
  const drawEntries = (dir, level, rows) => {
    const listing = explorer.listings.get(dir);
    if (!listing) {
      rows.push(
        explorerNoteRow(level, "Listing...", "terminal-explorer-waiting"),
      );
      return;
    }
    if (listing.error) {
      rows.push(
        explorerNoteRow(level, listing.error, "terminal-explorer-problem"),
      );
      return;
    }
    if (!listing.entries.length)
      rows.push(explorerNoteRow(level, "Empty", "terminal-explorer-waiting"));
    for (const entry of listing.entries) {
      const row = explorerRow(entry, dir, level);
      rows.push(row);
      if (row.getAttribute("aria-expanded") === "true")
        drawEntries(row.explorerPath, level + 1, rows);
    }
    if (listing.total > listing.entries.length) {
      const more = document.createElement("button");
      more.type = "button";
      more.className = "terminal-explorer-more";
      more.textContent = "Open in the Table viewer";
      more.onclick = () => openInViewer(dir);
      rows.push(
        explorerNoteRow(
          level,
          listing.entries.length + " of " + listing.total + " shown.",
          "terminal-explorer-truncated",
          more,
        ),
      );
    }
  };
  const renderExplorer = () => {
    if (!explorer.element) return;
    const hadFocus = explorer.tree.contains(document.activeElement);
    explorer.visible = [];
    const shell = explorerShell();
    const label = shell ? shellName(shell) : "";
    explorer.title.textContent = explorer.root
      ? baseName(explorer.root)
      : "Explorer";
    explorer.where.textContent = explorer.root;
    explorer.where.title = explorer.root;
    const rows = [];
    if (explorer.problem)
      rows.push(
        explorerNoteRow(0, explorer.problem, "terminal-explorer-problem"),
      );
    else if (explorer.root) drawEntries(explorer.root, 0, rows);
    explorer.tree.replaceChildren(...rows);
    if (!explorer.visible.some((row) => row.path === explorer.selected))
      explorer.selected = "";
    const first = explorer.tree.querySelector('[role="treeitem"]');
    if (!explorer.selected && first) first.tabIndex = 0;
    if (hadFocus) (rowOf(explorer.selected) || first)?.focus();
    explorer.footer.replaceChildren();
    if (explorer.busy) showExplorerBusy(explorer.busy, explorer.progress);
    else if (!shell) explorer.footer.append("This pane shows no shell.");
    else if (explorer.follow) explorer.footer.append("Following " + label);
    else {
      explorer.footer.append(
        "Not following" +
          (explorer.shellDir
            ? " · " + label + " is in " + explorer.shellDir
            : ""),
      );
      const back = document.createElement("button");
      back.type = "button";
      back.className = "terminal-explorer-follow-link";
      back.textContent = "Follow";
      back.onclick = () => setFollow(true);
      explorer.footer.append(back);
    }
    explorer.footer.title = explorer.footer.textContent;
  };
  const explorerKeys = (event) => {
    const index = explorer.visible.findIndex(
      (row) => row.path === explorer.selected,
    );
    const current = explorer.visible[index];
    const go = (row) => {
      if (row) selectRow(row.path);
      event.preventDefault();
    };
    switch (event.key) {
      case "ArrowDown":
        return go(
          explorer.visible[index + 1] ||
            (index < 0 ? explorer.visible[0] : null),
        );
      case "ArrowUp":
        return go(explorer.visible[index - 1]);
      case "Home":
        return go(explorer.visible[0]);
      case "End":
        return go(explorer.visible[explorer.visible.length - 1]);
      case "ArrowRight":
        if (!current?.folder || !current.named) return;
        event.preventDefault();
        if (!explorer.expanded.has(current.path)) toggleFolder(current.path);
        else
          go(
            explorer.visible[index + 1]?.parent === current.path
              ? explorer.visible[index + 1]
              : null,
          );
        return;
      case "ArrowLeft":
        if (!current) return;
        event.preventDefault();
        if (current.folder && explorer.expanded.has(current.path))
          toggleFolder(current.path);
        else go(explorer.visible.find((row) => row.path === current.parent));
        return;
      case "Enter":
        if (!current) return;
        event.preventDefault();
        if (!current.named) showNote(unnamedNote, 6000);
        else if (current.folder) toggleFolder(current.path);
        else openInViewer(current.path);
        return;
      case "F2":
        if (!current?.named) return;
        event.preventDefault();
        renameExplorerItem(current.path);
        return;
      case "Delete":
        if (!current?.named) return;
        event.preventDefault();
        deleteExplorerItem(current.path, current.folder);
        return;
    }
    if (!(event.ctrlKey || event.metaKey) || event.altKey) return;
    const key = event.key.toLowerCase();
    if (key === "v") {
      event.preventDefault();
      pasteExplorerItem();
    } else if ((key === "c" || key === "x") && current?.named) {
      event.preventDefault();
      if (key === "c" && current.folder)
        showNote("Folders are not copied here; use cp -r in the shell.", 6000);
      else setExplorerClipboard(current.path, current.folder, key === "x");
    }
  };
  const savedExplorerWidth = () => {
    try {
      return (
        Number(localStorage.getItem(explorerWidthKey)) || explorerDefaultWidth
      );
    } catch {
      return explorerDefaultWidth;
    }
  };
  const applyExplorerWidth = (width) => {
    const room = explorer.body?.getBoundingClientRect().width || 0;
    const most = room
      ? Math.max(explorerMinWidth, room - explorerPanesRoom)
      : width;
    const clamped = Math.round(
      Math.max(explorerMinWidth, Math.min(width, most)),
    );
    panel.style.setProperty("--terminal-explorer-width", clamped + "px");
    return clamped;
  };
  // applyExplorer shows the Explorer or hides it, at the width last chosen as far as the
  // panes leave room.
  const applyExplorer = () => {
    if (!explorer.element) return;
    explorer.element.hidden = !explorer.open;
    explorer.toggle.setAttribute("aria-pressed", String(explorer.open));
    applyExplorerWidth(savedExplorerWidth());
  };
  const setExplorerOpen = (open) => {
    explorer.open = open;
    try {
      localStorage.setItem(explorerOpenKey, open ? "1" : "0");
    } catch {}
    applyExplorer();
    fitPanes();
    if (open) {
      refreshExplorer();
      (
        rowOf(explorer.selected) ||
        explorer.tree.querySelector('[role="treeitem"]') ||
        explorer.tree
      ).focus();
    }
  };
  const buildExplorerResizer = () => {
    const handle = document.createElement("div");
    handle.className = "terminal-explorer-resizer";
    handle.setAttribute("role", "separator");
    handle.setAttribute("aria-orientation", "vertical");
    handle.setAttribute("aria-label", "Resize the Explorer");
    handle.tabIndex = 0;
    const resizeTo = (width) => {
      const applied = applyExplorerWidth(width);
      try {
        localStorage.setItem(explorerWidthKey, String(applied));
      } catch {}
      fitPanes();
    };
    handle.onpointerdown = (event) => {
      event.preventDefault();
      handle.setPointerCapture?.(event.pointerId);
      holdResizes();
      const left = explorer.body.getBoundingClientRect().left;
      handle.onpointermove = (move) => resizeTo(move.clientX - left);
      handle.onpointerup = handle.onpointercancel = () => {
        handle.onpointermove =
          handle.onpointerup =
          handle.onpointercancel =
            null;
        releaseResizes();
      };
    };
    handle.onkeydown = (event) => {
      const step =
        event.key === "ArrowLeft" ? -16 : event.key === "ArrowRight" ? 16 : 0;
      if (!step) return;
      event.preventDefault();
      resizeTo(explorer.element.getBoundingClientRect().width + step);
    };
    return handle;
  };
  // buildExplorer makes the Explorer and the body it shares with the panes.
  const buildExplorer = () => {
    const aside = document.createElement("aside");
    aside.className = "terminal-explorer";
    aside.setAttribute("aria-label", "Explorer");
    const head = document.createElement("div");
    head.className = "terminal-explorer-head";
    explorer.title = document.createElement("span");
    explorer.title.className = "terminal-explorer-title";
    explorer.followButton = explorerIconButton(
      "Follow the shell",
      explorerIcons.follow,
      () => setFollow(!explorer.follow),
    );
    explorer.followButton.title =
      "Follow the shell: the tree moves when this pane's shell changes directory";
    explorer.followButton.setAttribute("aria-pressed", String(explorer.follow));
    head.append(
      explorer.title,
      explorerIconButton("Up one directory", explorerIcons.up, explorerUp),
      explorer.followButton,
      explorerIconButton("New File", explorerIcons.newFile, () =>
        newExplorerItem("create"),
      ),
      explorerIconButton("New Folder", explorerIcons.newFolder, () =>
        newExplorerItem("mkdir"),
      ),
      explorerIconButton("Refresh", explorerIcons.refresh, () =>
        refreshExplorer(),
      ),
      explorerIconButton("Collapse all", explorerIcons.collapse, () => {
        explorer.expanded.clear();
        renderExplorer();
      }),
    );
    explorer.where = document.createElement("div");
    explorer.where.className = "terminal-explorer-where";
    explorer.tree = document.createElement("div");
    explorer.tree.className = "terminal-explorer-tree";
    explorer.tree.setAttribute("role", "tree");
    explorer.tree.setAttribute("aria-label", "Files");
    explorer.tree.tabIndex = -1;
    explorer.tree.onkeydown = explorerKeys;
    // The tree's empty part stands for the top of the tree.
    takesDrops(explorer.tree, () => explorer.root, false);
    explorer.tree.oncontextmenu = (event) => {
      if (event.target !== explorer.tree || !explorer.root) return;
      openPanelMenu(event, [
        ["New File...", () => newExplorerItem("create", explorer.root), true],
        ["New Folder...", () => newExplorerItem("mkdir", explorer.root), true],
        [
          "Paste",
          () => pasteExplorerItem(explorer.root),
          Boolean(explorer.clipboard),
        ],
        ["Upload...", () => chooseUploads(explorer.root), true],
      ]);
    };
    explorer.footer = document.createElement("div");
    explorer.footer.className = "terminal-explorer-footer";
    aside.append(
      head,
      explorer.where,
      explorer.tree,
      explorer.footer,
      buildExplorerResizer(),
    );
    explorer.element = aside;
    explorer.body = document.createElement("div");
    explorer.body.className = "terminal-body";
    explorer.body.append(aside, panesBox);
    // A file dropped beside the tree would make the browser open it in place of the page.
    explorer.body.addEventListener("dragover", (event) => {
      if (carriesFiles(event)) event.preventDefault();
    });
    explorer.body.addEventListener("drop", (event) => {
      if (!carriesFiles(event)) return;
      event.preventDefault();
      showNote("Drop files on a folder in the Explorer to upload them.", 4000);
    });
    new ResizeObserver(() => applyExplorer()).observe(explorer.body);
    return explorer.body;
  };
  // A path dragged from the Explorer onto a pane goes into that pane's shell.
  const takeExplorerDrops = (pane) => {
    const carries = (event) =>
      [...(event.dataTransfer?.types || [])].includes(explorerPathType);
    pane.element.addEventListener("dragover", (event) => {
      if (!carries(event)) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = "copy";
      pane.element.classList.add("terminal-pane-drop");
    });
    pane.element.addEventListener("dragleave", () =>
      pane.element.classList.remove("terminal-pane-drop"),
    );
    pane.element.addEventListener("drop", (event) => {
      pane.element.classList.remove("terminal-pane-drop");
      if (!carries(event)) return;
      event.preventDefault();
      const path = event.dataTransfer.getData(explorerPathType);
      if (focused !== pane) focusPane(pane);
      sendExplorerPath(path);
    });
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
    takeExplorerDrops(pane);
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
      if (inWindow)
        tellOpener({
          type: "rotari-terminal-go",
          path: groups.value.slice(windowPath.length),
        });
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
    clear.title =
      "Clear this shell's screen and scrollback, here and on the server";
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
    splitRightButton = iconButton("Split right", [paneFrame, "M8 2.5v11"], () =>
      splitPane(true),
    );
    splitDownButton = iconButton("Split down", [paneFrame, "M1.5 8h13"], () =>
      splitPane(false),
    );
    placeButton = iconButton(
      "Panel position",
      [paneFrame, placeLines.bottom],
      openPlaceMenu,
    );
    placeButton.disabled = false;
    launchButton = iconButton(
      "Launch here",
      icons.launch,
      () => {
        const shell = shells.find((item) => item.id === focused.shown);
        if (shell) openLaunchDialog(shell);
      },
      "Launch a registered command, such as TensorBoard, in the directory this pane's shell is in now",
    );
    overviewButton = iconButton(
      "All shells",
      icons.overview,
      toggleOverview,
      "All shells, of every project, to choose one",
    );
    overviewButton.disabled = false;
    overviewButton.setAttribute("aria-pressed", "false");
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
    explorer.toggle = document.createElement("button");
    explorer.toggle.type = "button";
    explorer.toggle.className = "terminal-explorer-toggle";
    drawIcon(explorer.toggle, explorerIcons.explorer);
    explorer.toggle.setAttribute("aria-label", "Explorer");
    explorer.toggle.title = "Explorer: the files where this pane's shell is";
    explorer.toggle.onclick = () => setExplorerOpen(!explorer.open);
    bar.append(
      explorer.toggle,
      title,
      groups,
      tabs,
      add,
      overviewButton,
      launchButton,
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
    const body = buildExplorer();
    if (inWindow) {
      for (const button of [maximize, hide]) button.remove();
      panel.append(bar, body);
    } else {
      panel.append(buildResizer(), bar, body);
    }
    document.body.append(panel);
    applyPlace();
    applyExplorer();
    new ResizeObserver(() => {
      if (isStacked() !== laidStacked) applyLayout();
    }).observe(panesBox);
  };
  // openTerminal gives a pane its screen, inside its viewport.
  const openTerminal = (pane) => {
    const { viewport } = pane;
    // xterm.js keeps 1000 lines by default, fewer than the 256 KiB replayed after a
    // link usually holds, so what the server kept would be cut short here.
    const term = new window.Terminal({
      scrollback: terminalScrollback,
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
      new window.WebLinksAddon.WebLinksAddon((_, uri) =>
        window.open(uri, "_blank", "noopener"),
      ),
    );
    term.open(viewport);
    fit.fit();
    term.onData((data) => {
      send(pane, new TextEncoder().encode(data));
      // Enter may have changed the shell's directory, which the Explorer follows.
      if (pane === focused && data.includes("\r")) explorerSoon();
    });
    term.onBinary((data) =>
      send(
        pane,
        Uint8Array.from(data, (character) => character.charCodeAt(0)),
      ),
    );
    term.onResize(({ rows, cols }) => {
      if (dragging) pane.heldResize = { rows, cols };
      else send(pane, JSON.stringify({ type: "resize", rows, cols }));
    });
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
    matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.(
      "change",
      applyTheme,
    );
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
    if (!open) closeOverview();
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
      if (window.opener && !window.opener.closed)
        window.opener.postMessage(message, location.origin);
    } catch {}
  };
  // The page's path as far as whose shells it shows, which a window opens on: a project's
  // page, which a run's page gives, a basedir's All projects, which its other pages give,
  // or Activity and History search, of no project.
  const projectPath = (path = location.pathname) => {
    const page = pageOfPath(path);
    if (page.global) return "/" + page.global;
    const prefix = page.mounted ? "/_basedir/" + page.mounted : "";
    return prefix + (page.project ? "/project/" + page.project : "/");
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
      settleSend(
        pane,
        "The terminal moved to its own window, so nothing was sent.",
      );
    }
    closeOverview();
    root.classList.remove("terminal-open");
    document
      .getElementById("terminal-button")
      ?.setAttribute("aria-pressed", "false");
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
  // followPage has the window show this page's project or basedir, when it is not the one
  // the window last followed. A page of no project, such as Activity, leaves it.
  const followPage = () => {
    const path = projectPath();
    if (pageOfPath(path).global || path === followed()) return;
    rememberFollowed(path);
    terminalWindow.postMessage(
      { type: "rotari-terminal-follow", path },
      location.origin,
    );
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
    const address =
      windowPath +
      projectPath() +
      "#" +
      encodeURIComponent(JSON.stringify(handover));
    const opened = window.open(address, "", "popup,width=900,height=600");
    if (!opened) {
      showBriefly(
        focused,
        "The browser blocked the window. Allow pop-ups for this site to use it.",
      );
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
      showBriefly(
        focused,
        "The page that opened this window is gone, so the terminal stays here.",
      );
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
    terminalWindow.postMessage(
      { type: "rotari-terminal-send", id, text, path: projectPath() },
      location.origin,
    );
    terminalWindow.focus();
    return answer;
  };
  // In the window, only the page that opened it sends, from this server.
  addEventListener("message", async (event) => {
    if (
      !inWindow ||
      event.origin !== location.origin ||
      !window.opener ||
      event.source !== window.opener
    )
      return;
    const { type, id, text, path, place } = event.data || {};
    if (type === "rotari-terminal-send" && typeof text === "string") {
      const problem = await sendToPane(focused, text);
      const elsewhere =
        !problem &&
        typeof path === "string" &&
        projectPath(path) !==
          projectPath(location.pathname.slice(windowPath.length));
      tellOpener({
        type: "rotari-terminal-sent",
        id,
        problem,
        project: elsewhere ? projectName(here) : "",
      });
    } else if (type === "rotari-terminal-follow" && typeof path === "string") {
      if (!path.startsWith("/") || path.startsWith("//")) return;
      if (
        projectPath(path) !==
        projectPath(location.pathname.slice(windowPath.length))
      ) {
        location.assign(windowPath + projectPath(path));
      }
    } else if (
      type === "rotari-terminal-here" &&
      typeof path === "string" &&
      typeof place === "string"
    ) {
      // Only a path on this server: one starting // would be another.
      if (!path.startsWith("/") || path.startsWith("//")) return;
      window.focus();
      if (
        projectPath(path) ===
        projectPath(location.pathname.slice(windowPath.length))
      ) {
        placeOverride = place;
        newAsked = true;
        begin();
      } else {
        location.assign(
          windowPath +
            projectPath(path) +
            (place ? "?start=" + encodeURIComponent(place) : ""),
        );
      }
    }
  });
  // Only this server's pages, and only the window this page opened, have a say. A page
  // loaded while the panel was in a window takes the first that says it is there.
  addEventListener("message", (event) => {
    if (
      inWindow ||
      event.origin !== location.origin ||
      typeof event.data?.type !== "string"
    )
      return;
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
      if (
        typeof path === "string" &&
        path.startsWith("/") &&
        !path.startsWith("//")
      ) {
        rememberFollowed(projectPath(path));
        location.assign(path);
      }
    } else if (
      type === "rotari-terminal-sent" &&
      sendsWaiting.has(event.data.id)
    ) {
      const { problem, project } = event.data;
      if (!problem && typeof project === "string") sentToProject = project;
      sendsWaiting.get(event.data.id)(
        typeof problem === "string" ? problem : null,
      );
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
      terminalWindow.postMessage(
        {
          type: "rotari-terminal-here",
          path: projectPath(),
          place: pagePlace(),
        },
        location.origin,
      );
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
      if (text.length > pasteLimit)
        return "This is too long to send, so nothing was sent.";
    } else if (/[\u0000-\u001f\u007f-\u009f]/.test(text)) {
      // A line break would run what came before it, and other control characters, from a
      // value in the state, could do anything a keyboard can.
      return "This command has control characters, so it was not sent.";
    }
    if (!text) return "There is nothing to send.";
    if (terminalWindow) return sendToWindow(text);
    if (awaitingWindow)
      return "The terminal's window has not answered yet, so nothing was sent.";
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
    const showing =
      pane.term && inGroup.some((shell) => shell.id === pane.shown);
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
  window.sendToTerminalAndSay = async (
    text,
    { word = false, pasted = false } = {},
  ) => {
    sentToProject = "";
    const problem = await window.sendToTerminal(word ? shellWord(text) : text, {
      pasted,
    });
    const sent = sentToProject
      ? "Sent to the terminal's window, in " +
        sentToProject +
        ". Nothing runs until you press Enter there."
      : "Sent. Nothing runs until you press Enter in the terminal.";
    showNote(problem || sent, problem ? 6000 : sentToProject ? 4000 : 2500);
  };
  window.sendButtonPressed = (button) => {
    const { sendText, sendValue } = button.dataset;
    return window.sendToTerminalAndSay(sendText ?? (sendValue || ""), {
      word: sendText === undefined,
    });
  };
  // The Recent jobs page is drawn on the server, which leaves its copy buttons plain.
  const addJobsSendButtons = () => {
    for (const copy of document.querySelectorAll(".jobs-copy")) {
      const send = copy.cloneNode(true);
      const label = (copy.getAttribute("aria-label") || "").replace(
        /^Copy /,
        "",
      );
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
        const path = document.createElementNS(
          "http://www.w3.org/2000/svg",
          "path",
        );
        path.setAttribute("d", shape);
        icon.append(path);
      }
      copy.after(send);
    }
  };
  // Signing out ends every shell, so a page asks first when anything is running in one: a
  // command in its foreground, or processes it left in the background. It names each such
  // shell and what runs there. A shell it cannot look into counts too, saying so, and a page
  // that cannot list the shells asks anyway, rather than ending them unasked.
  let signingOut = false;
  const signOutLine = async (shell) => {
    let preview;
    try {
      const response = await serverFetch(
        shellsPath + "/" + encodeURIComponent(shell.id) + "/preview",
      );
      if (response.ok) preview = await response.json();
      else
        return (
          "  " +
          shellName(shell) +
          ": what runs is not known (" +
          ((await response.text()).trim() || "status " + response.status) +
          ")"
        );
    } catch {
      return (
        "  " +
        shellName(shell) +
        ": what runs is not known (the server did not answer)"
      );
    }
    const below = Array.isArray(preview.processes) ? preview.processes : [];
    const command = preview.command || (shell.running ? "a command" : "");
    if (!command && !below.length) {
      if (preview.processes_error)
        return (
          "  " +
          shellName(shell) +
          ": its processes are not known (" +
          preview.processes_error +
          ")"
        );
      return null;
    }
    const noun = below.length === 1 ? " process" : " processes";
    if (command)
      return (
        "  " +
        shellName(shell) +
        ": " +
        command +
        (below.length ? " (" + below.length + noun + ")" : "")
      );
    const named = below
      .slice(0, 3)
      .map((process) => process.command)
      .join(", ");
    return (
      "  " +
      shellName(shell) +
      ": " +
      below.length +
      " background" +
      noun +
      " (" +
      named +
      (below.length > 3 ? ", ..." : "") +
      ")"
    );
  };
  // confirmEndingShells asks before something that ends every shell, naming what runs in
  // them, or that this could not be checked; ending is "Signing out" or "Stopping the
  // server", and question what to ask. With nothing running, or on a page without the
  // terminal, it asks quietQuestion, or nothing when that is empty.
  const confirmEndingShells = async (ending, question, offered, quietQuestion) => {
    const quiet = () => !quietQuestion || confirm(quietQuestion);
    if (!offered) return quiet();
    let problem = "";
    let listed = [];
    try {
      const response = await serverFetch(shellsPath);
      if (response.ok) listed = (await response.json()).shells || [];
      else
        problem = (await response.text()).trim() || "status " + response.status;
    } catch {
      problem = "the server did not answer";
    }
    if (problem)
      return confirm(
        "Whether anything is running in a shell could not be checked: " +
          problem +
          ". " +
          ending +
          " ends every shell. " +
          question,
      );
    const lines = (await Promise.all(listed.map(signOutLine))).filter(Boolean);
    if (!lines.length) return quiet();
    const shown = lines.slice(0, 5);
    if (lines.length > 5) shown.push("  and " + (lines.length - 5) + " more");
    const head =
      lines.length === 1
        ? "Something is running in a shell"
        : "Something is running in " + lines.length + " shells";
    return confirm(
      head +
        ":\n" +
        shown.join("\n") +
        "\n" +
        ending +
        " ends every shell, and what runs in it. " +
        question,
    );
  };
  const confirmSignOut = async (form) => {
    if (!(await confirmEndingShells("Signing out", "Sign out anyway?", true, "")))
      return;
    signingOut = true;
    form.submit();
  };
  // Stop server asks, even with nothing running, since the page loses its server, then
  // sends the request and says what happened beside the button.
  const stopServer = async (button, offered) => {
    const status = document.getElementById("stop-server-status");
    const say = (text) => {
      if (status) status.textContent = text;
    };
    const asked = await confirmEndingShells(
      "Stopping the server",
      "Stop the rotari web server anyway?",
      offered,
      "Stop the rotari web server? This page will lose its connection.",
    );
    if (!asked) return;
    button.disabled = true;
    say("Stopping...");
    try {
      const response = await serverFetch("/_server/shutdown", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      if (response.status !== 202) {
        button.disabled = false;
        say(
          "Not stopped: " +
            ((await response.text()).trim() || "status " + response.status),
        );
        return;
      }
    } catch {
      button.disabled = false;
      say("Not stopped: the server did not answer.");
      return;
    }
    say("The server stopped. Start rotari web again to use this page.");
    const banner = document.getElementById("disconnect-banner");
    if (banner)
      banner.textContent =
        "rotari web was stopped from this page. Start it again and reload to continue.";
  };
  const guardSignOut = () =>
    document.addEventListener(
      "submit",
      (event) => {
        const form = event.target;
        if (
          signingOut ||
          !(form instanceof HTMLFormElement) ||
          form.getAttribute("action") !== "/_auth/logout"
        )
          return;
        event.preventDefault();
        confirmSignOut(form);
      },
      true,
    );
  const sayAlive = async () => {
    try {
      await serverFetch("/_terminal/alive", { method: "POST" });
    } catch {}
  };
  document.addEventListener("DOMContentLoaded", () => {
    const offered = inWindow || document.getElementById("terminal-button");
    const stopButton = document.getElementById("stop-server-button");
    if (stopButton)
      stopButton.addEventListener("click", () =>
        stopServer(stopButton, Boolean(offered)),
      );
    if (offered) {
      if (!inWindow) {
        addJobsSendButtons();
        guardSignOut();
      }
      // The sidebar is drawn again with every refresh, without the marks.
      const sidebarList = document.getElementById("sidebar-basedirs");
      if (sidebarList)
        new MutationObserver(markSidebarProjects).observe(sidebarList, {
          childList: true,
          subtree: true,
        });
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
        if (!document.hidden && !panes.some((pane) => pane.whenVisible))
          refreshShells();
      }, shellsRefresh);
    }
    // Opening a window asks for a shell, as opening the panel does.
    if (inWindow) {
      if (placeOverride) newAsked = true;
      else startAsked = true;
      setOpen(true);
      // The window goes with the browser tab that opened it. Following a link or reloading
      // keeps that tab, so only closing it closes the window; the shells keep running, as
      // when the panel is hidden. A window opened by its address has no tab to go with.
      const hadOpener = Boolean(window.opener);
      const openerGone = () => {
        try {
          return !window.opener || window.opener.closed;
        } catch {
          return false;
        }
      };
      const beat = () => {
        if (hadOpener && openerGone()) {
          window.close();
          return;
        }
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
