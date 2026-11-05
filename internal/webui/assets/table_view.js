// The Table viewer: delimited or JSON Lines files as paged tables, with
// document previews. The server parses, filters, and pages the table
// (internal/tableview); this script keeps the view's settings in the URL
// fragment, so a reload or a shared link shows the same view, and lays out
// what the server returns.
"use strict";

// The page's query names the source; API requests pass it through and add
// the view's own parameters. Moving to another directory changes it.
let sourceQuery = new URLSearchParams(location.search);

const separatorChoices = [
  { label: "Comma ,", value: "," },
  { label: "Tab", value: "\t" },
  { label: "Semicolon ;", value: ";" },
  { label: "Pipe |", value: "|" },
  { label: "Space", value: " " },
];
const pageSizes = [50, 100, 200, 500];
// The filter operators, as the server names them (internal/tableview).
const filterOperators = [
  { op: "=", label: "=" },
  { op: "!=", label: "≠" },
  { op: "<", label: "<" },
  { op: "<=", label: "≤" },
  { op: ">", label: ">" },
  { op: ">=", label: "≥" },
  { op: "contains", label: "contains" },
  { op: "not_contains", label: "does not contain" },
  { op: "regex", label: "matches regex" },
  { op: "not_regex", label: "does not match regex" },
  { op: "empty", label: "is empty", noValue: true },
  { op: "not_empty", label: "is not empty", noValue: true },
];
// The statistics Describe shows, one row each.
const describeRows = [
  ["histogram", (stats, column) => histogramCell(stats.histogram, column)],
  ["count", (stats) => formatCount(stats.count)],
  [
    "unique",
    (stats) => formatCount(stats.unique) + (stats.unique_over ? "+" : ""),
  ],
  ["top", (stats, column) => topValuesCell(stats, column)],
  ["mean", (stats, column) => formatValue(stats.mean, column)],
  ["std", (stats, column) => formatValue(stats.std, column)],
  ["min", (stats, column) => formatValue(stats.min, column)],
  ["25%", (stats, column) => formatValue(stats.q25, column)],
  ["50%", (stats, column) => formatValue(stats.q50, column)],
  ["75%", (stats, column) => formatValue(stats.q75, column)],
  ["max", (stats, column) => formatValue(stats.max, column)],
  ["sum", (stats, column) => formatValue(stats.sum, column)],
];
// How a directory is split into a stems view's groups.
const groupChoices = [
  { value: "", label: "automatic" },
  { value: "top", label: "first-level directory" },
  { value: "parent", label: "parent directory" },
  { value: "extension", label: "file extension" },
  { value: "suffix", label: "suffix after a separator" },
];

let info = null;
let rows = null;
let description = null;
// view holds the settings the fragment keeps. A filter or sort names a
// column by its position, counted from 0.
const view = {
  base: "",
  sep: "",
  // A directory's listing.
  listing: "files",
  pattern: "**/*",
  expanded: [],
  group: "",
  suffixSep: "_",
  incomplete: false,
  header: true,
  page: 1,
  limit: 100,
  filters: [],
  sort: null,
  describe: false,
  // row is the current row's number, or null.
  row: null,
};
// defaultView is the view a source starts with when the fragment sets
// nothing.
const defaultView = JSON.stringify(view);
// draft is the condition being written in the filter bar.
const draft = { column: 0, op: "contains", value: "" };

// el builds an element: attributes set as properties (or with setAttribute
// for names with a dash), children appended as nodes or text.
function el(tag, attributes, ...children) {
  const node = document.createElement(tag);
  Object.entries(attributes || {}).forEach(([name, value]) => {
    if (value === undefined || value === null || value === false) return;
    if (name.includes("-")) node.setAttribute(name, String(value));
    else node[name] = value;
  });
  setChildren(node, ...children);
  return node;
}

// setChildren replaces a node's children as el adds them: arrays are
// flattened and null, undefined, and false are left out.
function setChildren(node, ...children) {
  node.replaceChildren(
    ...children
      .flat(Infinity)
      .filter(
        (child) => child !== null && child !== undefined && child !== false,
      )
      .map((child) => (child instanceof Node ? child : String(child))),
  );
}

function formatCount(value) {
  return Number(value).toLocaleString("en-US");
}

function formatNumber(value) {
  if (value === undefined || value === null) return "–";
  return Number(value).toLocaleString("en-US", { maximumFractionDigits: 4 });
}

// formatValue writes a number in its column's units.
function formatValue(value, column) {
  if (value === undefined || value === null) return "–";
  if (column && column.type === "duration") return formatDuration(value);
  if (column && column.type === "bytes") return formatBytes(Math.round(value));
  return formatNumber(value);
}

// formatCell writes a cell of a typed column for reading: durations,
// sizes, and times; other cells are shown as they are.
function formatCell(text, column) {
  if (text === "" || !column || !column.type) return text;
  if (column.type === "time") return text.replace("T", " ").replace("Z", "");
  const value = Number(text);
  if (!Number.isFinite(value)) return text;
  if (column.type === "duration") return formatDuration(value);
  if (column.type === "bytes") return formatBytes(value);
  if (column.type === "number") return formatCount(value);
  return text;
}

// formatDuration writes seconds as m:ss.cc, or h:mm:ss past an hour.
function formatDuration(seconds) {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  if (hours) {
    return (
      hours +
      ":" +
      String(minutes).padStart(2, "0") +
      ":" +
      String(Math.floor(rest)).padStart(2, "0")
    );
  }
  return minutes + ":" + rest.toFixed(2).padStart(5, "0");
}

function formatRate(rate) {
  return rate % 1000 ? (rate / 1000).toFixed(1) + " kHz" : rate / 1000 + " kHz";
}

// formatProperties is one line for a media file, such as
// "WAV PCM 16-bit · 0:03.12 · 16 kHz · 1 ch · 256 kbps · 97.5 KiB".
function formatProperties(properties) {
  const parts = [
    properties.format +
      (properties.bit_depth ? " " + properties.bit_depth + "-bit" : ""),
  ];
  if (properties.duration) parts.push(formatDuration(properties.duration));
  if (properties.width && properties.height) {
    parts.push(properties.width + "×" + properties.height);
  }
  if (properties.sample_rate) parts.push(formatRate(properties.sample_rate));
  if (properties.channels) parts.push(properties.channels + " ch");
  if (properties.bitrate) {
    parts.push(Math.round(properties.bitrate / 1000) + " kbps");
  }
  parts.push(formatBytes(properties.size));
  return parts.join(" · ");
}

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

function readView() {
  Object.assign(view, JSON.parse(defaultView));
  const fragment = new URLSearchParams(location.hash.slice(1));
  if (fragment.has("sep")) view.sep = fragment.get("sep");
  if (fragment.get("listing") === "stems") view.listing = "stems";
  if (fragment.has("pattern")) view.pattern = fragment.get("pattern");
  view.expanded = fragment.getAll("expanded");
  if (fragment.has("group")) view.group = fragment.get("group");
  if (fragment.has("suffix_sep")) view.suffixSep = fragment.get("suffix_sep");
  view.incomplete = fragment.get("incomplete") === "1";
  if (fragment.has("base")) view.base = fragment.get("base");
  if (fragment.has("header")) view.header = fragment.get("header") !== "0";
  const pageNumber = Number(fragment.get("page"));
  if (Number.isInteger(pageNumber) && pageNumber > 0) view.page = pageNumber;
  const limit = Number(fragment.get("limit"));
  if (pageSizes.includes(limit)) view.limit = limit;
  try {
    const filters = JSON.parse(fragment.get("filters") || "[]");
    if (Array.isArray(filters)) view.filters = filters;
  } catch {
    view.filters = [];
  }
  const sort = Number(fragment.get("sort"));
  if (fragment.has("sort") && Number.isInteger(sort) && sort >= 0) {
    view.sort = { column: sort, desc: fragment.get("desc") === "1" };
  }
  view.describe = fragment.get("describe") === "1";
  const row = Number(fragment.get("row"));
  if (Number.isInteger(row) && row > 0) view.row = row;
}

function writeView() {
  const fragment = new URLSearchParams();
  if (info.kind === "directory") {
    fragment.set("listing", view.listing);
    fragment.set("pattern", view.pattern);
    view.expanded.forEach((directory) =>
      fragment.append("expanded", directory),
    );
    if (view.listing === "stems") {
      if (view.group) fragment.set("group", view.group);
      if (view.group === "suffix") fragment.set("suffix_sep", view.suffixSep);
      if (view.incomplete) fragment.set("incomplete", "1");
    }
  } else if (info.format === "jsonl") {
    if (view.base) fragment.set("base", view.base);
  } else {
    fragment.set("sep", view.sep);
    if (view.base) fragment.set("base", view.base);
    fragment.set("header", view.header ? "1" : "0");
  }
  fragment.set("page", String(view.page));
  fragment.set("limit", String(view.limit));
  if (view.filters.length)
    fragment.set("filters", JSON.stringify(view.filters));
  if (view.sort) {
    fragment.set("sort", String(view.sort.column));
    if (view.sort.desc) fragment.set("desc", "1");
  }
  if (view.describe) fragment.set("describe", "1");
  if (view.row) fragment.set("row", String(view.row));
  history.replaceState(null, "", "#" + fragment);
}

// apiURL is an API route relative to the page, with the source's query, or
// the given one, and the given parameters.
function apiURL(route, extra, source) {
  const query = new URLSearchParams(source ?? sourceQuery);
  Object.entries(extra || {}).forEach(([name, value]) => {
    if (Array.isArray(value)) {
      query.delete(name);
      value.forEach((item) => query.append(name, String(item)));
    } else if (value !== undefined && value !== null) {
      query.set(name, String(value));
    }
  });
  return route + "?" + query;
}

async function fetchJSON(url) {
  const response = await fetch(url);
  const body = await response.text();
  if (!response.ok) throw new Error(body.trim() || "HTTP " + response.status);
  return JSON.parse(body);
}

function showError(message) {
  const error = document.getElementById("tv-error");
  error.hidden = !message;
  error.textContent = message || "";
}

// currentChild is the path inside the source's directory this page shows,
// or "" for the source itself.
function currentChild() {
  return sourceQuery.get("child") || "";
}

// childOf is a path a cell names inside the directory this page shows, as
// a path inside the source's directory.
function childOf(ref) {
  const parent = currentChild();
  return parent ? parent + "/" + ref : ref;
}

// childURL is the viewer's page for a path inside the source's directory,
// "" naming the source itself.
function childURL(child) {
  const query = new URLSearchParams(sourceQuery);
  if (child) query.set("child", child);
  else query.delete("child");
  return "?" + query;
}

// plainClick tells a plain click on a link, which the page handles itself,
// from one with a modifier, which the browser handles, such as opening a
// new tab.
function plainClick(event) {
  return !(event.ctrlKey || event.metaKey || event.shiftKey || event.button);
}

// moveLink is a link to a path inside the source's directory that a plain
// click shows in this window.
function moveLink(child, attributes, ...children) {
  return el(
    "a",
    Object.assign({}, attributes, {
      href: childURL(child),
      onclick: (event) => {
        if (!plainClick(event)) return;
        event.preventDefault();
        moveTo(child);
      },
    }),
    ...children,
  );
}

// rootName is the source's own name: the server names a child as the
// source's name, a slash, and the child.
function rootName() {
  const child = currentChild();
  return child && info.name.endsWith("/" + child)
    ? info.name.slice(0, -child.length - 1)
    : info.name;
}

// renderName shows where this page is as a path whose directories link to
// their own pages, after a link up to the parent directory.
function renderName() {
  const child = currentChild();
  const parts = child ? child.split("/") : [];
  const crumbs = [rootName()].concat(parts);
  const name = [];
  crumbs.forEach((crumb, index) => {
    if (index) name.push(el("span", { className: "tv-crumb-sep" }, "/"));
    const target = parts.slice(0, index).join("/");
    name.push(
      index === crumbs.length - 1
        ? el("span", { "aria-current": "page" }, crumb)
        : moveLink(target, { className: "tv-crumb" }, crumb),
    );
  });
  setChildren(document.getElementById("tv-name"), name);
  const up = document.getElementById("tv-up");
  up.hidden = !child && info.kind !== "directory";
  setChildren(
    up,
    child
      ? moveLink(
          parts.slice(0, -1).join("/"),
          {
            className: "tv-up",
            title: "Up to the parent directory (Alt+↑)",
            "aria-label": "Up to the parent directory",
          },
          "↑",
        )
      : el(
          "span",
          {
            className: "tv-up",
            "aria-disabled": "true",
            title: "This is the top directory the viewer can show",
          },
          "↑",
        ),
  );
}

function renderHeader() {
  document.title = info.name + " · Table viewer · rotari";
  document.getElementById("tv-trail").textContent = (info.trail || [])
    .filter(Boolean)
    .concat("Table viewer")
    .join(" › ");
  renderName();
  const kind = document.getElementById("tv-kind");
  kind.hidden = info.kind !== "directory";
  kind.textContent = info.kind;
  const download = document.getElementById("tv-download");
  download.hidden = !info.download;
  if (info.download) download.href = info.download;
}

function renderSummary() {
  const parts = [];
  if (info.kind === "file") parts.push(formatBytes(info.size));
  if (rows) {
    const noun =
      info.kind === "file"
        ? " rows"
        : view.listing === "stems"
          ? " stems"
          : " entries";
    parts.push(formatCount(rows.total) + noun);
    parts.push(formatCount(rows.columns.length) + " columns");
  }
  document.getElementById("tv-summary").textContent = parts.join(" · ");
}

// showDescribe brings Describe's rows into view: open panes can push the table
// down the page, and the table's own scrolling can have left its head above.
function showDescribe() {
  const page = document.querySelector(".tv-main");
  const area = document.querySelector(".tv-table-area");
  page.scrollTop +=
    area.getBoundingClientRect().top - page.getBoundingClientRect().top;
  document.querySelector(".tv-scroll").scrollTop = 0;
}

// describeToggle is the button that shows or hides Describe.
function describeToggle() {
  return el(
    "button",
    {
      type: "button",
      className: "tv-describe-toggle",
      "aria-pressed": String(view.describe),
      title: "Summarize each column over the matching rows",
      onclick: async () => {
        view.describe = !view.describe;
        renderToolbar();
        const error = await loadRows();
        if (view.describe && error === "") showDescribe();
      },
    },
    "Describe",
  );
}

// changeListing applies a change to how a directory is listed, which makes
// other columns: filters and sort name them by position.
function changeListing(change) {
  Object.assign(view, change);
  view.page = 1;
  view.filters = [];
  view.sort = null;
  renderToolbar();
  return loadRows();
}

// renderDirectoryToolbar lays out a directory's choices: files or stems,
// the pattern, and how stems are grouped.
function renderDirectoryToolbar(toolbar) {
  const listingButton = (value, label) =>
    el(
      "button",
      {
        type: "button",
        "aria-pressed": String(view.listing === value),
        onclick: () => changeListing({ listing: value }),
      },
      label,
    );
  const pattern = el("input", {
    type: "text",
    className: "tv-pattern",
    "aria-label": "Pattern",
    value: view.pattern,
    title: "* matches within a name, ** any directories, {a,b} either",
    onchange: (event) =>
      changeListing({ pattern: event.target.value || "**/*" }),
  });
  const stems = view.listing === "stems";
  const group = el(
    "select",
    {
      "aria-label": "Groups from",
      onchange: (event) => changeListing({ group: event.target.value }),
    },
    groupChoices.map((choice) =>
      el(
        "option",
        { value: choice.value, selected: choice.value === view.group },
        choice.label,
      ),
    ),
  );
  const suffix = el("input", {
    type: "text",
    className: "tv-separator-other",
    "aria-label": "Suffix separator",
    value: view.suffixSep,
    onchange: (event) =>
      changeListing({ suffixSep: event.target.value || "_" }),
  });
  const incomplete = el("input", {
    type: "checkbox",
    checked: view.incomplete,
    onchange: (event) => changeListing({ incomplete: event.target.checked }),
  });
  setChildren(
    toolbar,
    el(
      "span",
      { className: "tv-segmented", role: "group", "aria-label": "View" },
      listingButton("files", "Files"),
      listingButton("stems", "Group by stem"),
    ),
    el("label", {}, el("span", {}, "Pattern"), pattern),
    stems
      ? el(
          "label",
          {},
          el("span", {}, "Groups from"),
          group,
          view.group === "suffix" ? suffix : null,
        )
      : null,
    stems
      ? el(
          "span",
          { className: "tv-muted" },
          "Expand directories in Files to include their files.",
        )
      : null,
    stems
      ? el(
          "label",
          {},
          incomplete,
          el("span", {}, "Only stems missing in some group"),
        )
      : null,
    el("span", { className: "tv-spacer" }),
    describeToggle(),
  );
}

function renderToolbar() {
  const toolbar = document.getElementById("tv-toolbar");
  if (info.preview) {
    setChildren(toolbar);
    return;
  }
  if (info.kind === "directory") {
    renderDirectoryToolbar(toolbar);
    return;
  }
  if (info.format === "jsonl") {
    const bases = info.bases || [];
    const base = el(
      "select",
      {
        "aria-label": "Resolve media paths from",
        disabled: !!info.static,
        onchange: (event) => {
          view.base = event.target.value;
          loadRows();
        },
      },
      bases.map((item, index) =>
        el(
          "option",
          {
            value: item.id,
            selected: view.base ? item.id === view.base : index === 0,
          },
          item.label,
        ),
      ),
    );
    setChildren(
      toolbar,
      el("span", { className: "tv-format-label" }, "JSON Lines"),
      bases.length > 1
        ? el("label", {}, el("span", {}, "Resolve media paths from"), base)
        : null,
      el("span", { className: "tv-spacer" }),
      describeToggle(),
    );
    return;
  }
  const known = separatorChoices.some((choice) => choice.value === view.sep);
  const select = el(
    "select",
    {
      "aria-label": "Separator",
      disabled: !!info.static,
      onchange: changeSeparatorChoice,
    },
    separatorChoices.map((choice) =>
      el(
        "option",
        { value: choice.value, selected: choice.value === view.sep },
        choice.label,
      ),
    ),
    el("option", { value: "other", selected: !known }, "Other…"),
  );
  const other = el("input", {
    type: "text",
    className: "tv-separator-other",
    "aria-label": "Separator character",
    value: known ? "" : view.sep,
    hidden: known,
    disabled: !!info.static,
    maxLength: 2,
    onchange: (event) => changeSeparator(event.target.value),
  });
  const header = el("input", {
    type: "checkbox",
    checked: view.header,
    disabled: !!info.static,
    onchange: (event) => {
      view.header = event.target.checked;
      view.page = 1;
      loadRows();
    },
  });
  const bases = info.bases || [];
  const base = el(
    "select",
    {
      "aria-label": "Resolve media paths from",
      disabled: !!info.static,
      onchange: (event) => {
        view.base = event.target.value;
        loadRows();
      },
    },
    bases.map((base, index) =>
      el(
        "option",
        {
          value: base.id,
          selected: view.base ? base.id === view.base : index === 0,
        },
        base.label,
      ),
    ),
  );
  setChildren(
    toolbar,
    el("label", {}, el("span", {}, "Separator"), select, other),
    el("label", {}, header, el("span", {}, "First row is a header")),
    bases.length > 1
      ? el("label", {}, el("span", {}, "Resolve media paths from"), base)
      : null,
    el("span", { className: "tv-spacer" }),
    describeToggle(),
  );
}

function changeSeparatorChoice(event) {
  if (event.target.value === "other") {
    const other = document.querySelector(".tv-separator-other");
    other.hidden = false;
    other.focus();
    return;
  }
  changeSeparator(event.target.value);
}

function changeSeparator(value) {
  if ([...value].length !== 1) {
    showError("The separator must be one character.");
    return;
  }
  view.sep = value;
  view.page = 1;
  // Another separator makes other columns: filters and sort name them by
  // position.
  view.filters = [];
  view.sort = null;
  renderToolbar();
  loadRows();
}

function pageCount() {
  return Math.max(1, Math.ceil(rows.matched / view.limit));
}

function sortParams() {
  return view.sort
    ? { sort: view.sort.column, desc: view.sort.desc ? "1" : undefined }
    : {};
}

// tableParams are the parameters that choose the rows, for both the rows
// and their description.
function tableParams() {
  const filters = view.filters.length
    ? JSON.stringify(view.filters)
    : undefined;
  if (info.kind === "directory") {
    const stems = view.listing === "stems";
    return {
      view: view.listing,
      pattern: view.pattern,
      expanded: view.expanded,
      group: stems && view.group ? view.group : undefined,
      suffix_sep: stems && view.group === "suffix" ? view.suffixSep : undefined,
      incomplete: stems && view.incomplete ? "1" : undefined,
      filters,
    };
  }
  if (info.format === "jsonl") {
    return {
      base: view.base || undefined,
      filters,
    };
  }
  return {
    base: view.base || undefined,
    sep: view.sep,
    header: view.header ? "1" : "0",
    filters,
  };
}

// loadSequence numbers loads, so a slow answer to an earlier one never
// replaces a later one.
let loadSequence = 0;

async function loadRows() {
  const sequence = ++loadSequence;
  noteRowTable();
  writeView();
  const params = tableParams();
  const sort = sortParams();
  const [rowsResult, describeResult] = await Promise.allSettled([
    fetchJSON(
      apiURL(
        "api/rows",
        Object.assign({}, params, sort, {
          offset: (view.page - 1) * view.limit,
          limit: view.limit,
        }),
      ),
    ),
    view.describe ? fetchJSON(apiURL("api/describe", params)) : null,
  ]);
  if (sequence !== loadSequence) return;
  rows = rowsResult.status === "fulfilled" ? rowsResult.value : null;
  description =
    describeResult.status === "fulfilled" ? describeResult.value : null;
  const failure = [rowsResult, describeResult].find(
    (result) => result.status === "rejected",
  );
  showError(failure ? failure.reason.message : "");
  if (rows && view.page > pageCount()) {
    view.page = pageCount();
    return loadRows();
  }
  renderSummary();
  renderFilterBar();
  renderTable();
  renderPager();
  return failure ? failure.reason.message : "";
}

function changeFilters(filters) {
  view.filters = filters;
  view.page = 1;
  return loadRows();
}

function operatorOf(op) {
  return filterOperators.find((operator) => operator.op === op);
}

function renderFilterBar() {
  const bar = document.getElementById("tv-filterbar");
  // A refused filter from a URL leaves no rows; its chip is the way out of it.
  if (!rows && !view.filters.length) {
    bar.replaceChildren();
    return;
  }
  const columnName = (index) =>
    rows && rows.columns[index]
      ? rows.columns[index].name
      : "column " + (index + 1);
  const chips = view.filters.map((filter, index) =>
    el(
      "span",
      { className: "tv-chip" },
      el("strong", {}, columnName(filter.column)),
      " " + (operatorOf(filter.op) || { label: filter.op }).label,
      filter.value !== undefined && filter.value !== ""
        ? el("code", {}, filter.value)
        : null,
      el(
        "button",
        {
          type: "button",
          "aria-label": "Remove condition",
          onclick: () =>
            changeFilters(view.filters.filter((_, other) => other !== index)),
        },
        "×",
      ),
    ),
  );
  if (!rows) {
    setChildren(
      bar,
      el("span", { className: "tv-filter-label" }, "Filter"),
      chips,
    );
    return;
  }
  if (info.static) {
    // A static snapshot (table_view_static.js) cannot filter.
    setChildren(
      bar,
      el("span", { className: "tv-filter-label" }, "Filter"),
      el("span", { className: "tv-muted tv-static-note" }, info.static),
    );
    return;
  }
  draft.column = Math.min(draft.column, rows.columns.length - 1);
  const value = el("input", {
    type: "text",
    className: "tv-filter-value",
    "aria-label": "Value",
    value: draft.value,
    hidden: operatorOf(draft.op).noValue,
    oninput: (event) => (draft.value = event.target.value),
    onkeydown: (event) => {
      if (event.key === "Enter") addFilter();
    },
  });
  const column = el(
    "select",
    {
      "aria-label": "Column",
      onchange: (event) => (draft.column = Number(event.target.value)),
    },
    rows.columns.map((column, index) =>
      el(
        "option",
        { value: index, selected: index === draft.column },
        column.name,
      ),
    ),
  );
  const operator = el(
    "select",
    {
      "aria-label": "Operator",
      onchange: (event) => {
        draft.op = event.target.value;
        value.hidden = !!operatorOf(draft.op).noValue;
      },
    },
    filterOperators.map((operator) =>
      el(
        "option",
        { value: operator.op, selected: operator.op === draft.op },
        operator.label,
      ),
    ),
  );
  setChildren(
    bar,
    el("span", { className: "tv-filter-label" }, "Filter"),
    chips,
    view.filters.length > 1
      ? el("span", { className: "tv-muted" }, "all must match")
      : null,
    el(
      "span",
      { className: "tv-filter-form" },
      column,
      operator,
      value,
      el("button", { type: "button", onclick: addFilter }, "+ Add"),
    ),
    view.filters.length
      ? el(
          "button",
          {
            type: "button",
            className: "tv-link-button",
            onclick: () => changeFilters([]),
          },
          "Clear all",
        )
      : null,
  );
}

// addFilter adds the condition being written. A refused one comes off again,
// back in the form to be corrected, with the rows as they were.
async function addFilter() {
  const previous = view.filters;
  const typed = draft.value;
  const filter = { column: draft.column, op: draft.op };
  if (!operatorOf(draft.op).noValue) filter.value = draft.value;
  draft.value = "";
  const error = await changeFilters(view.filters.concat(filter));
  if (error) {
    view.filters = previous;
    draft.value = typed;
    await loadRows();
    showError(error);
  }
}

// toggleSort orders by a column, ascending, then descending, then not.
function toggleSort(column) {
  if (!view.sort || view.sort.column !== column) {
    view.sort = { column, desc: false };
  } else if (!view.sort.desc) {
    view.sort = { column, desc: true };
  } else {
    view.sort = null;
  }
  view.page = 1;
  loadRows();
}

function renderTable() {
  const table = document.getElementById("tv-table");
  if (!rows) {
    table.replaceChildren();
    return;
  }
  // A stems view's rows open all their files side by side.
  const rowActions = info.kind === "directory" && view.listing === "stems";
  const head = el(
    "thead",
    {},
    el(
      "tr",
      {},
      el("th", { className: "tv-number", scope: "col" }, "#"),
      rowActions ? el("th", { scope: "col" }, "") : null,
      rows.columns.map((column, index) => {
        const sorted = view.sort && view.sort.column === index;
        const order = sorted
          ? view.sort.desc
            ? "descending"
            : "ascending"
          : "none";
        return el(
          "th",
          { scope: "col", "aria-sort": order },
          el(
            "button",
            {
              type: "button",
              className: "tv-sort",
              title: "Sort by " + column.name,
              onclick: () => toggleSort(index),
            },
            column.name,
            sorted ? (view.sort.desc ? " ▼" : " ▲") : "",
          ),
        );
      }),
    ),
    describeHead(rowActions),
  );
  const body = el(
    "tbody",
    {},
    rows.rows.map((row) =>
      el(
        "tr",
        {
          "data-number": row.number,
          onclick: (event) => {
            // A click on a control in the row, or one ending a text
            // selection, is not a move.
            if (event.target.closest("a, button, input, select")) return;
            if (String(window.getSelection())) return;
            const cell = event.target.closest("td[data-column]");
            if (cell) currentColumn = Number(cell.dataset.column);
            stepToRow(row);
            markCurrentRow();
          },
        },
        el("td", { className: "tv-number" }, formatCount(row.number)),
        rowActions ? el("td", {}, openRowButton(row)) : null,
        rows.columns.map((_, index) =>
          el("td", { "data-column": index }, tableCell(row, index)),
        ),
      ),
    ),
  );
  table.replaceChildren(head, body);
  markCurrentRow();
  if (!rows.rows.length) {
    body.append(
      el(
        "tr",
        {},
        el(
          "td",
          {
            className: "tv-empty",
            colSpan: rows.columns.length + (rowActions ? 2 : 1),
          },
          "No rows.",
        ),
      ),
    );
  }
}

async function toggleDirectory(directory) {
  const previous = view.expanded;
  const expanded = view.expanded.includes(directory)
    ? view.expanded.filter(
        (path) => path !== directory && !path.startsWith(directory + "/"),
      )
    : view.expanded.concat(directory);
  view.expanded = expanded;
  const error = await loadRows();
  if (error && view.expanded === expanded) {
    view.expanded = previous;
    await loadRows();
    showError(error);
  }
}

// directoryCell is a Files listing's directory: its name expands it in
// place, and the arrow shows it in this window, or with a modifier in a new
// tab.
function directoryCell(row, text) {
  const expanded = view.expanded.includes(row.directory);
  return el(
    "span",
    { className: "tv-directory-cell" },
    el(
      "button",
      {
        type: "button",
        className: "tv-directory tv-link-button",
        "aria-expanded": String(expanded),
        "aria-label": (expanded ? "Collapse " : "Expand ") + row.directory,
        onclick: () => toggleDirectory(row.directory),
      },
      (expanded ? "▾ " : "▸ ") + text,
    ),
    moveLink(
      childOf(row.directory),
      {
        className: "tv-enter",
        title: "Open " + row.directory + " here (Ctrl+click: new tab)",
        "aria-label": "Open " + row.directory,
      },
      "→",
    ),
  );
}

function tableCell(row, index) {
  const text = row.cells[index] ?? "";
  const content =
    row.directory && index === 0
      ? directoryCell(row, text)
      : row.media && row.media[index]
        ? mediaCell(row, index)
        : info.kind === "directory" &&
            rows.columns[index].type === "path" &&
            text
          ? fileCell(row, index)
          : cellContent(text, rows.columns[index]);
  if (info.kind !== "directory" || view.listing !== "files" || index !== 0)
    return content;
  const depth = (row.directory || text).split("/").length - 1;
  return el(
    "span",
    { className: "tv-tree-path", style: "padding-left: " + depth + "rem" },
    content,
  );
}

// The current row. Keys and clicks move it through the rows the page shows, in
// their displayed order and across pages, and the active pane's current tab
// follows: it shows its column of the new row.

// How long moves must pause before the tab follows. Held keys must not start a
// decode per row, since a decode cannot be cancelled.
const followDelayMs = 150;
// rowTable is the tableIdentity view.row counts rows in.
let rowTable = null;
// currentColumn is the framed cell's column in the current row. It stays as the
// row moves, and is null until a row is current.
let currentColumn = null;
// pendingFollow is the move the tab has not followed yet.
let pendingFollow = null;
// rowPageLoading holds further moves while one loads the next page.
let rowPageLoading = false;

// tableIdentity names what a row number means. Filters, the sort, and paging
// keep each row's number, so they are not part of it; expanding a directory
// renumbers its rows, and a new separator or header row changes them.
function tableIdentity() {
  return JSON.stringify([
    sourceQuery.toString(),
    view.sep,
    view.header,
    view.base,
    view.listing,
    view.pattern,
    view.expanded,
    view.group,
    view.suffixSep,
    view.incomplete,
  ]);
}

// noteRowTable forgets the current row when the table it counted in changes.
function noteRowTable() {
  const table = tableIdentity();
  if (rowTable !== null && rowTable !== table) {
    view.row = null;
    currentColumn = null;
    if (pendingFollow) clearTimeout(pendingFollow.timer);
    pendingFollow = null;
  }
  rowTable = table;
}

// currentRow is the current row, if it is on this page.
function currentRow() {
  return rows ? rows.rows.find((row) => row.number === view.row) : undefined;
}

// firstFileColumn is where the frame starts in a row: its first file, so that
// Enter and Space have something to act on, or its first column.
function firstFileColumn(row) {
  const index = rows.columns.findIndex(
    (_, index) => rowFile(row, index).kind !== "notice",
  );
  return Math.max(index, 0);
}

function markCurrentRow() {
  const row = currentRow();
  if (row && currentColumn === null) currentColumn = firstFileColumn(row);
  document
    .querySelectorAll("#tv-table tbody tr[data-number]")
    .forEach((element) => {
      const current = Number(element.dataset.number) === view.row;
      element.classList.toggle("tv-row-current", current);
      if (current) element.setAttribute("aria-current", "true");
      else element.removeAttribute("aria-current");
      element.querySelectorAll("td[data-column]").forEach((cell) => {
        cell.classList.toggle(
          "tv-cell-current",
          current && Number(cell.dataset.column) === currentColumn,
        );
      });
    });
}

// scrollCurrentRow brings the current row and framed cell into view by scrolling
// the table alone. The panes above it stay where they are, so the part of the
// table in view can end above its own bottom edge, where the page cuts it off.
function scrollCurrentRow() {
  const row = document.querySelector("#tv-table tr.tv-row-current");
  if (!row) return;
  const scroller = document.querySelector(".tv-scroll");
  const area = scroller.getBoundingClientRect();
  const page = document.querySelector(".tv-main").getBoundingClientRect();
  const header = document.querySelector("#tv-table thead tr");
  const top = Math.max(area.top + (header ? header.offsetHeight : 0), page.top);
  const bottom = Math.min(area.bottom, page.bottom);
  const box = row.getBoundingClientRect();
  if (box.top < top) scroller.scrollTop -= top - box.top;
  else if (box.bottom > bottom) scroller.scrollTop += box.bottom - bottom;
  const cell = row.querySelector("td.tv-cell-current");
  if (!cell) return;
  const edges = cell.getBoundingClientRect();
  if (edges.left < area.left) scroller.scrollLeft -= area.left - edges.left;
  else if (edges.right > area.right)
    scroller.scrollLeft += edges.right - area.right;
}

function setCurrentRow(row) {
  view.row = row.number;
  markCurrentRow();
  writeView();
  scrollCurrentRow();
}

// releaseTableFocus takes focus from the control just clicked in the table,
// which would otherwise keep Enter and Space and keep its focus ring on the row
// left behind.
function releaseTableFocus() {
  const focused = document.activeElement;
  if (focused && focused.closest("#tv-table")) focused.blur();
}

// stepToRow makes a row current, and shortly after, the tab follows.
function stepToRow(row) {
  if (row.number === view.row) return;
  if (pendingFollow) clearTimeout(pendingFollow.timer);
  setCurrentRow(row);
  pendingFollow = { row, timer: setTimeout(finishFollow, followDelayMs) };
}

function finishFollow() {
  if (!pendingFollow) return;
  const { row, timer } = pendingFollow;
  clearTimeout(timer);
  pendingFollow = null;
  followActiveTab(tableIdentity(), row.number, (column) =>
    rowFile(row, column),
  );
}

// rowFile is what a column of a row opens in a pane: the file its cell names,
// or, so that a pane never goes on showing another row's file, a notice saying
// why there is none.
function rowFile(row, index) {
  const origin = { table: tableIdentity(), row: row.number, column: index };
  const media = row.media && row.media[index];
  if (media && !media.error) return Object.assign({}, media, { origin });
  const column = rows && rows.columns[index];
  const text = row.cells[index];
  if (
    !media &&
    !(row.directory && index === 0) &&
    info.kind === "directory" &&
    column &&
    column.type === "path" &&
    text
  ) {
    return { kind: "text", ref: text, origin };
  }
  const label = "Row " + formatCount(row.number);
  return {
    kind: "notice",
    ref: label,
    origin,
    message: media
      ? label + ": " + media.ref + ": " + media.error
      : label +
        " has no file in " +
        (column ? column.name : "this column") +
        ".",
  };
}

// rowFiles is every file a row names, in column order.
function rowFiles(row) {
  return (rows ? rows.columns : [])
    .map((_, index) => rowFile(row, index))
    .filter((media) => media.kind !== "notice");
}

// openRowFile opens one cell's file in the active pane; the row becomes current.
function openRowFile(row, index) {
  finishFollow();
  currentColumn = index;
  setCurrentRow(row);
  openMedia(rowFile(row, index));
  releaseTableFocus();
}

// openWholeRow opens a row's files side by side.
function openWholeRow(row, media) {
  finishFollow();
  setCurrentRow(row);
  openMediaRow(media);
  releaseTableFocus();
}

// moveRow moves the current row by one in the displayed order. From a row not
// on this page, it starts at the page's first row.
async function moveRow(delta) {
  if (rowPageLoading) return;
  const index = rows.rows.findIndex((row) => row.number === view.row);
  if (index < 0) {
    stepToRow(rows.rows[0]);
    return;
  }
  const target = index + delta;
  if (target >= 0 && target < rows.rows.length) {
    stepToRow(rows.rows[target]);
    return;
  }
  const page = view.page + delta;
  if (page < 1 || page > pageCount()) return;
  rowPageLoading = true;
  view.page = page;
  try {
    const error = await loadRows();
    if (error === "" && view.page === page && rows && rows.rows.length)
      stepToRow(rows.rows[delta > 0 ? 0 : rows.rows.length - 1]);
  } finally {
    rowPageLoading = false;
  }
}

// moveColumn moves the frame along the row, stopping at either end. It opens
// nothing. Without a current row on this page, it starts as moveRow does.
function moveColumn(delta) {
  if (!currentRow()) {
    moveRow(delta);
    return;
  }
  const column = (currentColumn ?? 0) + delta;
  if (column < 0 || column >= rows.columns.length) return;
  currentColumn = column;
  markCurrentRow();
  scrollCurrentRow();
}

// rowKey handles the table's keys. Controls keep the keys they use.
function rowKey(event) {
  if (event.ctrlKey || event.metaKey || event.altKey) return;
  if (event.defaultPrevented) return;
  const openRow = event.key === "Enter" && event.shiftKey;
  if (event.shiftKey && !openRow) return;
  const target = event.target;
  const inside = (selector) => target.closest && target.closest(selector);
  if (inside("input, select, textarea, audio, video, [contenteditable]"))
    return;
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    if (!rows || !rows.rows.length) return;
    event.preventDefault();
    moveRow(event.key === "ArrowDown" ? 1 : -1);
    return;
  }
  if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
    if (!rows || !rows.rows.length) return;
    event.preventDefault();
    moveColumn(event.key === "ArrowRight" ? 1 : -1);
    return;
  }
  if (inside("button, a, summary")) return;
  if (event.key === "Enter") {
    const row = currentRow();
    if (!row) return;
    if (openRow) {
      const files = rowFiles(row);
      if (!files.length) return;
      event.preventDefault();
      if (!event.repeat) openWholeRow(row, files);
      return;
    }
    if (rowFile(row, currentColumn).kind === "notice") return;
    event.preventDefault();
    if (!event.repeat) openRowFile(row, currentColumn);
    return;
  }
  if (event.key !== " ") return;
  // The framed file plays; on a cell without one, Space still pauses what is
  // playing, and with no player at all it keeps scrolling the page.
  const row = currentRow();
  const file = row && rowFile(row, currentColumn);
  const playable = file && (file.kind === "audio" || file.kind === "video");
  if (event.repeat) {
    if (playable || activePlayer()) event.preventDefault();
    return;
  }
  if (playable) {
    event.preventDefault();
    finishFollow();
    togglePlayer(playerOf(file));
  } else if (toggleActivePlayer()) {
    event.preventDefault();
  }
}

// openRowButton opens a row's files in panes side by side, to compare them.
function openRowButton(row) {
  const media = rowFiles(row).filter((media) => media.kind !== "text");
  return media.length
    ? el(
        "button",
        {
          type: "button",
          className: "tv-open-row",
          title: "Open this row's files in panes side by side",
          onclick: () => openWholeRow(row, media),
        },
        "Open row",
      )
    : null;
}

// fileCell is a directory's path to a file that is not media, which opens
// as text in the active pane; a table file also opens in a Table viewer of
// its own in a new tab.
function fileCell(row, index) {
  const ref = row.cells[index];
  const table = row.tables && row.tables.includes(index);
  return el(
    "span",
    { className: "tv-media" },
    el(
      "button",
      {
        type: "button",
        className: "tv-media-path tv-link-button tv-text-file",
        title: "Open in the active pane",
        onclick: () => openRowFile(row, index),
      },
      ref,
    ),
    table
      ? el(
          "a",
          {
            className: "tv-open-table",
            href: childURL(childOf(ref)),
            target: "_blank",
            rel: "noopener",
            title: "Open " + ref + " in a new Table viewer",
          },
          "Open table",
        )
      : null,
  );
}

// cellContent is a cell that names no media: a path of a stems view's
// group that has no file for the stem says so.
function cellContent(text, column) {
  if (column.type === "path") {
    return text
      ? el("span", { className: "tv-media-path" }, text)
      : el("span", { className: "tv-missing" }, "— missing");
  }
  return formatCell(text, column);
}

// placeMedia is a file a cell names as a pane keeps it: with the source
// query its path resolves under, which stays when the page moves to
// another directory, and its path inside the source's directory to show.
function placeMedia(media) {
  if (media.source !== undefined) return media;
  const child = currentChild();
  const inDirectory =
    info && info.kind === "directory" && child && !media.ref.startsWith("/");
  return Object.assign({}, media, {
    source: sourceQuery.toString(),
    path: inDirectory ? child + "/" + media.ref : media.ref,
  });
}

// mediaURL requests the file a cell names, as the server resolved it, or
// its copy in a static snapshot.
function mediaURL(media) {
  if (media.url) return media.url;
  return apiURL(
    "api/media",
    { ref: media.ref, base: media.base || undefined },
    media.source,
  );
}

// downloadURL saves a tab's file, or is "" for a tab that holds no file or a file
// a static snapshot did not copy.
function downloadURL(media) {
  if (media.kind === "notice" || media.kind === "histogram") return "";
  if (media.url) return media.url;
  if (info && info.static) return "";
  return apiURL(
    "api/download",
    { ref: media.ref, base: media.base || undefined },
    media.source,
  );
}

// mediaCell shows a path that names an image, audio, or video file: the
// path, which opens the file, and its properties, or why it is not shown.
function mediaCell(row, index) {
  const media = row.media[index];
  return el(
    "span",
    { className: "tv-media" },
    media.error
      ? el("span", { className: "tv-media-path" }, media.ref)
      : el(
          "a",
          {
            className: "tv-media-path",
            href: mediaURL(media),
            target: "_blank",
            rel: "noopener",
            title: "Open in the active pane",
            // A plain click opens the file in a pane; with a modifier the
            // browser opens the link as usual.
            onclick: (event) => {
              if (!plainClick(event)) return;
              event.preventDefault();
              openRowFile(row, index);
            },
          },
          media.ref,
        ),
    el(
      "span",
      { className: media.error ? "tv-media-error" : "tv-media-properties" },
      media.error || formatProperties(media.properties),
    ),
  );
}

// histogramCell is a button that opens a column's histogram in a pane: bars in
// the table would widen every column.
function histogramCell(histogram, column) {
  if (!histogram) return "–";
  return el(
    "button",
    {
      type: "button",
      className: "tv-link-button tv-open-histogram",
      title: "Open the histogram of " + column.name + " in the active pane",
      onclick: () => {
        // Each selection of rows has its own tab, since it counts different rows.
        const selection = JSON.stringify(view.filters);
        openMedia({
          kind: "histogram",
          ref: column.name,
          path: column.name,
          source: sourceQuery.toString() + "#" + selection,
          histogram,
          column,
          filtered: view.filters.length > 0,
        });
        releaseTableFocus();
      },
    },
    "Histogram",
  );
}

// The bar counts a histogram tab offers: each divides the server's bins exactly,
// so merging them stays exact.
const histogramBinChoices = [10, 20, 50, 100, 200];
const histogramBinDefault = 20;

// mergeBins adds neighbouring bins together, leaving count of them. One bin, for
// equal values, has nothing to merge.
function mergeBins(counts, count) {
  if (counts.length <= count) return counts;
  const size = counts.length / count;
  const merged = new Array(count).fill(0);
  counts.forEach((value, index) => {
    merged[Math.floor(index / size)] += value;
  });
  return merged;
}

// histogramView draws a histogram into a pane's root, each bar with its range and
// count, at the tab's chosen number of bars.
function histogramView(media, root) {
  const { histogram, column } = media;
  const counts = mergeBins(histogram.counts, media.bins || histogramBinDefault);
  const tallest = Math.max(...counts);
  const width = (histogram.max - histogram.min) / counts.length;
  const value = (number) => formatValue(number, column);
  const total = counts.reduce((sum, count) => sum + count, 0);
  const summary =
    formatCount(total) +
    " numbers" +
    (media.filtered ? " in the rows selected when opened" : "") +
    "; tallest bar " +
    formatCount(tallest);
  return [
    el(
      "div",
      { className: "tv-tab-title" },
      el("strong", {}, column.name),
      el("span", { className: "tv-media-properties" }, summary),
      histogram.counts.length > 1
        ? el(
            "label",
            { className: "tv-histogram-bins" },
            "Bars",
            el(
              "select",
              {
                "aria-label": "Number of bars",
                onchange: (event) => {
                  media.bins = Number(event.target.value);
                  setChildren(root, histogramView(media, root));
                },
              },
              histogramBinChoices.map((choice) =>
                el(
                  "option",
                  {
                    value: choice,
                    selected: choice === counts.length,
                  },
                  String(choice),
                ),
              ),
            ),
          )
        : null,
    ),
    el(
      "div",
      {
        className: "tv-histogram",
        role: "img",
        "aria-label":
          formatCount(total) +
          " numbers from " +
          value(histogram.min) +
          " to " +
          value(histogram.max),
      },
      counts.map((count, index) =>
        el(
          "span",
          {
            className: "tv-histogram-bin",
            title:
              value(histogram.min + index * width) +
              " – " +
              value(histogram.min + (index + 1) * width) +
              ": " +
              formatCount(count),
          },
          count
            ? el("span", {
                className: "tv-histogram-bar",
                style: "height: " + (count / tallest) * 100 + "%",
              })
            : null,
        ),
      ),
    ),
    el(
      "div",
      { className: "tv-histogram-axis" },
      el("span", {}, value(histogram.min)),
      el("span", {}, value(histogram.max)),
    ),
  ];
}

// topValuesCell lists a column's most frequent values, as text.
function topValuesCell(stats, column) {
  if (stats.top_over)
    return el(
      "span",
      { className: "tv-muted" },
      "too many distinct values to list",
    );
  if (!stats.top) return "–";
  return el(
    "div",
    { className: "tv-top-values" },
    stats.top.map((top) => {
      const shown = formatCell(top.value, column) + (top.cut ? "…" : "");
      return el(
        "div",
        { className: "tv-top-value" },
        el("span", { className: "tv-top-text", title: shown }, shown),
        el("span", { className: "tv-muted" }, formatCount(top.count)),
      );
    }),
  );
}

// describeHead is the Describe rows, under the column names.
function describeHead(rowActions) {
  if (!view.describe || !description) return null;
  const note = description.distribution_over
    ? el(
        "tr",
        { className: "tv-describe" },
        el(
          "td",
          {
            className: "tv-muted",
            colSpan: rows.columns.length + (rowActions ? 2 : 1),
          },
          "Too many numbers for quartiles and histograms; add a filter to see them.",
        ),
      )
    : null;
  const lines = describeRows.map(([name, show]) =>
    el(
      "tr",
      { className: "tv-describe" },
      el("th", { className: "tv-number", scope: "row" }, name),
      rowActions ? el("td", {}) : null,
      rows.columns.map((_, index) => {
        const stats = description.columns[index];
        return el("td", {}, stats ? show(stats, rows.columns[index]) : "–");
      }),
    ),
  );
  return [note, lines];
}

function goToPage(number) {
  view.page = Math.min(Math.max(1, number), pageCount());
  loadRows();
}

// exportLink downloads every row the current view selects. It appears only after
// those rows have loaded, so the query is known to be good: a browser shows no
// response body for a download that fails. A static snapshot has no server.
function exportLink() {
  if (info.static || !rows.matched) return null;
  const params = Object.assign({}, tableParams(), sortParams());
  delete params.base;
  const scope = info.kind === "directory" ? " listed" : "";
  return el(
    "a",
    {
      className: "tv-export",
      href: apiURL("api/export", params),
      download: "",
      title:
        "Download all " +
        formatCount(rows.matched) +
        scope +
        " selected rows, in this order",
    },
    "Export rows",
  );
}

function renderPager() {
  const pager = document.getElementById("tv-pager");
  if (!rows) {
    pager.replaceChildren();
    return;
  }
  const first = rows.offset + 1;
  const last = rows.offset + rows.rows.length;
  const count = el(
    "span",
    { className: "tv-pager-count" },
    rows.rows.length
      ? "Rows " + formatCount(first) + "–" + formatCount(last) + " of "
      : "No rows of ",
    formatCount(rows.matched),
    rows.matched !== rows.total
      ? el(
          "span",
          { className: "tv-muted" },
          " matching (" + formatCount(rows.total) + " in all)",
        )
      : null,
    rows.cut
      ? el("span", { className: "tv-muted" }, " · long cells are shortened")
      : null,
    (rows.notes || []).map((note) =>
      el("span", { className: "tv-muted" }, " · " + note),
    ),
    rows.rows.length
      ? el(
          "span",
          { className: "tv-muted tv-pager-keys" },
          " · ↑↓←→ move · Enter open · Shift+Enter open row · Space play",
        )
      : null,
  );
  const pages = pageCount();
  const button = (label, target, disabled) =>
    el(
      "button",
      { type: "button", disabled, onclick: () => goToPage(target) },
      label,
    );
  const input = el("input", {
    type: "number",
    className: "tv-page-input",
    min: 1,
    max: pages,
    value: view.page,
    "aria-label": "Page",
    onchange: (event) => goToPage(Number(event.target.value) || 1),
  });
  const buttons = el(
    "div",
    { className: "tv-pager-buttons" },
    button("« First", 1, view.page <= 1),
    button("‹ Previous", view.page - 1, view.page <= 1),
    el("label", {}, "Page", input, "of " + formatCount(pages)),
    button("Next ›", view.page + 1, view.page >= pages),
    button("Last »", pages, view.page >= pages),
  );
  const size = el(
    "label",
    { className: "tv-pager-size" },
    "Rows per page",
    el(
      "select",
      {
        onchange: (event) => {
          view.limit = Number(event.target.value);
          view.page = Math.floor(rows.offset / view.limit) + 1;
          loadRows();
        },
      },
      pageSizes.map((size) =>
        el(
          "option",
          { value: size, selected: size === view.limit },
          String(size),
        ),
      ),
    ),
  );
  setChildren(pager, count, exportLink(), buttons, size);
}

// loadSource shows the source the page's query names: its header, and its
// table or document.
async function loadSource() {
  const sequence = ++loadSequence;
  rows = null;
  description = null;
  let loaded;
  try {
    loaded = await fetchJSON(apiURL("api/info"));
  } catch (error) {
    if (sequence !== loadSequence) return;
    // Nothing of the page left before stays on show under the new query.
    document.querySelector(".tv-table-area").hidden = false;
    ["tv-toolbar", "tv-filterbar", "tv-table", "tv-pager"].forEach((id) =>
      setChildren(document.getElementById(id)),
    );
    document.getElementById("tv-summary").textContent = "";
    if (info) renderName();
    showError(error.message);
    return;
  }
  if (sequence !== loadSequence) return;
  info = loaded;
  showError("");
  if (!view.sep) view.sep = info.separator || ",";
  renderHeader();
  document.querySelector(".tv-table-area").hidden = Boolean(info.preview);
  if (info.preview) {
    renderToolbar();
    openMedia({
      kind: info.preview,
      ref: info.preview_ref,
      base: "file",
      properties: {
        format: info.preview === "pdf" ? "PDF" : "",
        size: info.size,
      },
    });
    return;
  }
  renderToolbar();
  await loadRows();
  scrollCurrentRow();
}

// moveTo shows a directory or file inside the source's directory in this
// window, as a new history entry. Its view starts afresh, keeping only Files
// or Group by stem; open panes stay.
function moveTo(child) {
  const listing = view.listing;
  history.pushState(null, "", childURL(child));
  sourceQuery = new URLSearchParams(location.search);
  readView();
  view.listing = listing;
  return loadSource();
}

async function start() {
  readView();
  startPanesHandle();
  window.addEventListener("popstate", () => {
    sourceQuery = new URLSearchParams(location.search);
    readView();
    loadSource();
  });
  document.addEventListener("keydown", (event) => {
    const child = currentChild();
    if (event.altKey && event.key === "ArrowUp" && child) {
      event.preventDefault();
      moveTo(child.split("/").slice(0, -1).join("/"));
      return;
    }
    rowKey(event);
  });
  await loadSource();
}

start();
