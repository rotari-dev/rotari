// The transcript page (WEB-60): one terminal pane's commands, one card each, taken from
// the tab that opened this page by postMessage, so that their output never goes through
// the server. The cards shown and checked are saved as Markdown, as one HTML file, or as a
// shell script of the commands alone; every file is made here and downloaded.
(() => {
  "use strict";
  // The theme chosen in the Web UI, before anything is drawn.
  try {
    const theme = localStorage.getItem("rotari-theme");
    if (theme === "light" || theme === "dark")
      document.documentElement.dataset.theme = theme;
  } catch {}

  // An output longer than this opens folded, with Show all.
  const foldedLines = 30;
  // The terminal's 16 colours, as the theme gives them.
  const paletteNames = [
    "black",
    "red",
    "green",
    "yellow",
    "blue",
    "magenta",
    "cyan",
    "white",
    "bright-black",
    "bright-red",
    "bright-green",
    "bright-yellow",
    "bright-blue",
    "bright-magenta",
    "bright-cyan",
    "bright-white",
  ];
  // The same 16 for a saved HTML file, which has no theme: the dark one's.
  const savedPalette = [
    "#1e2731",
    "#ff7b72",
    "#7ee787",
    "#e3b341",
    "#79c0ff",
    "#d2a8ff",
    "#76e3ea",
    "#c9d1d9",
    "#6e7681",
    "#ffa198",
    "#a5f0b0",
    "#f2cc60",
    "#a5d6ff",
    "#e2c5ff",
    "#b3f0ff",
    "#f0f6fc",
  ];
  const cubeLevels = [0, 95, 135, 175, 215, 255];
  // colourOf is a run's colour: one of the 16 (from palette), of the 256-colour cube or
  // grey ramp, or a true colour.
  const colourOf = (value, palette) => {
    if (typeof value === "string") return value;
    if (value < 16) return palette[value];
    if (value < 232) {
      const n = value - 16;
      const hex = (level) => cubeLevels[level].toString(16).padStart(2, "0");
      return (
        "#" + hex(Math.floor(n / 36)) + hex(Math.floor(n / 6) % 6) + hex(n % 6)
      );
    }
    const grey = (8 + (value - 232) * 10).toString(16).padStart(2, "0");
    return "#" + grey + grey + grey;
  };
  const themePalette = paletteNames.map(
    (name) => "var(--terminal-" + name + ")",
  );
  // runStyle is a run's CSS, as property and value pairs.
  const runStyle = (s, palette, foreground, background) => {
    const css = [];
    let fg = s.fg === undefined ? null : colourOf(s.fg, palette);
    let bg = s.bg === undefined ? null : colourOf(s.bg, palette);
    if (s.v) [fg, bg] = [bg || background, fg || foreground];
    if (fg) css.push(["color", fg]);
    if (bg) css.push(["background-color", bg]);
    if (s.b) css.push(["font-weight", "bold"]);
    if (s.d) css.push(["opacity", "0.65"]);
    if (s.i) css.push(["font-style", "italic"]);
    if (s.u) css.push(["text-decoration", "underline"]);
    return css;
  };
  const runsText = (runs) => runs.map((run) => run.t).join("");
  const lineCount = (runs) =>
    runs.length ? runs.filter((run) => run.t === "\n").length + 1 : 0;
  const stateWords = (card) =>
    card.state === "ok"
      ? "exit 0"
      : card.state === "failed"
        ? "failed, exit " + card.exit
        : card.state === "running"
          ? "still running"
          : card.state === "unknown"
            ? "no exit code"
            : "before the first prompt";
  const duration = (card) => {
    if (typeof card.startedAt !== "number" || typeof card.endedAt !== "number")
      return "";
    const ms = card.endedAt - card.startedAt;
    if (ms < 10000) return (ms / 1000).toFixed(1) + " s";
    const seconds = Math.round(ms / 1000);
    if (seconds < 3600)
      return Math.floor(seconds / 60) + " min " + (seconds % 60) + " s";
    return (
      Math.floor(seconds / 3600) +
      " h " +
      Math.floor((seconds % 3600) / 60) +
      " min"
    );
  };
  const commandTitle = (card) =>
    card.command === null
      ? "Output before the first prompt"
      : card.command || "(no command line)";
  const element = (tag, className, text) => {
    const made = document.createElement(tag);
    if (className) made.className = className;
    if (text !== undefined) made.textContent = text;
    return made;
  };

  // The page's frame.
  const head = element("header", "transcript-head");
  const title = element("h1", "", "Transcript");
  const about = element("p", "transcript-about");
  const privacy = element(
    "p",
    "transcript-note",
    "The commands and output come from the terminal tab that opened this page; nothing of them goes to the server.",
  );
  const tools = element("div", "transcript-tools");
  const filter = element("input");
  filter.type = "search";
  filter.placeholder = "Filter by command";
  filter.setAttribute("aria-label", "Filter by command");
  const failedLabel = element("label", "transcript-failed");
  const failedOnly = element("input");
  failedOnly.type = "checkbox";
  failedLabel.append(failedOnly, " Failed only");
  const button = (text, title, run) => {
    const made = element("button", "", text);
    made.type = "button";
    made.title = title;
    made.onclick = run;
    return made;
  };
  const checkAll = button("Check all", "Check every card shown", () =>
    checkShown(true),
  );
  const checkNone = button("Check none", "Uncheck every card shown", () =>
    checkShown(false),
  );
  const saveMarkdown = button(
    "Save Markdown",
    "Save the cards shown and checked as a Markdown file",
    () => save("md"),
  );
  const saveHTML = button(
    "Save HTML",
    "Save the cards shown and checked as one HTML file, colours kept",
    () => save("html"),
  );
  const saveScript = button(
    "Save shell script",
    "Save the commands of the cards shown and checked as a bash script, without their output",
    () => save("sh"),
  );
  const refresh = button(
    "Refresh",
    "Take the transcript again from the terminal",
    () => ask(),
  );
  tools.append(
    filter,
    failedLabel,
    checkAll,
    checkNone,
    saveMarkdown,
    saveHTML,
    saveScript,
    refresh,
  );
  const status = element("p", "transcript-status");
  status.setAttribute("role", "status");
  const list = element("main", "transcript-cards");
  head.append(title, about, privacy, tools);
  document.body.append(head, status, list);
  const savers = [checkAll, checkNone, saveMarkdown, saveHTML, saveScript];
  const say = (text) => {
    status.textContent = text;
    status.hidden = !text;
  };

  let transcript = null;
  // Each card's element and whether it is checked, in the transcript's order.
  let shown = [];
  const visible = (item) =>
    (!filter.value ||
      (item.card.command || "")
        .toLowerCase()
        .includes(filter.value.toLowerCase())) &&
    (!failedOnly.checked || item.card.state === "failed");
  const applyFilter = () => {
    for (const item of shown) item.element.hidden = !visible(item);
    const count = shown.filter(visible).length;
    if (transcript && !count && shown.length)
      say("No card matches the filter.");
    else if (transcript) say("");
  };
  filter.oninput = applyFilter;
  failedOnly.onchange = applyFilter;
  const checkShown = (on) => {
    for (const item of shown)
      if (visible(item)) {
        item.checked = on;
        item.box.checked = on;
      }
  };
  const chosen = () =>
    shown
      .filter((item) => item.checked && visible(item))
      .map((item) => item.card);

  // fillOutput puts a card's output in its box, styled, as text.
  const fillOutput = (box, runs) => {
    box.replaceChildren();
    for (const run of runs) {
      if (run.t === "\n" || !Object.keys(run.s).length) {
        box.append(run.t);
        continue;
      }
      const span = element("span", "", run.t);
      for (const [property, value] of runStyle(
        run.s,
        themePalette,
        "var(--terminal-foreground)",
        "var(--terminal-background)",
      ))
        span.style.setProperty(property, value);
      box.append(span);
    }
  };
  // Outputs are drawn as their cards come into view, so a long transcript stays usable.
  const observer =
    typeof IntersectionObserver === "function"
      ? new IntersectionObserver(
          (entries) => {
            for (const entry of entries) {
              if (!entry.isIntersecting) continue;
              observer.unobserve(entry.target);
              entry.target.fill?.();
            }
          },
          { rootMargin: "400px" },
        )
      : null;
  const copy = async (text, what) => {
    try {
      await navigator.clipboard.writeText(text);
      say("Copied " + what + ".");
    } catch (error) {
      say("Copy failed: " + (error?.message || error));
    }
  };
  const cardElement = (card, index) => {
    const item = { card, checked: true };
    const article = element("article", "transcript-card");
    article.dataset.state = card.state;
    const top = element("header", "transcript-card-head");
    const box = element("input");
    box.type = "checkbox";
    box.checked = true;
    box.setAttribute("aria-label", "Include " + commandTitle(card));
    box.onchange = () => (item.checked = box.checked);
    const dot = element("span", "transcript-dot");
    dot.dataset.state = card.state;
    const command = element(
      card.command && card.command.includes("\n") ? "pre" : "code",
      "transcript-command",
      commandTitle(card),
    );
    if (card.command === null) command.classList.add("transcript-command-none");
    const actions = element("span", "transcript-actions");
    if (card.command)
      actions.append(
        button("Copy command", "Copy this command's line", () =>
          copy(card.command, "the command"),
        ),
      );
    actions.append(
      button("Copy output", "Copy this output as text", () =>
        copy(runsText(card.runs), "the output"),
      ),
    );
    top.append(box, dot, command, actions);
    const meta = element(
      "p",
      "transcript-meta",
      [stateWords(card), card.dir, duration(card)].filter(Boolean).join(" · "),
    );
    article.append(top, meta);
    const lines = lineCount(card.runs);
    if (!lines) {
      article.append(element("p", "transcript-empty", "No output."));
    } else {
      const output = element("pre", "transcript-output");
      output.textContent = "…";
      article.fill = () => fillOutput(output, card.runs);
      article.append(output);
      if (lines > foldedLines) {
        output.classList.add("transcript-output-folded");
        const more = button(
          "Show all " + lines.toLocaleString("en-US") + " lines",
          "Show the whole output",
          () => {
            output.classList.remove("transcript-output-folded");
            more.remove();
          },
        );
        more.classList.add("transcript-more");
        article.append(more);
      }
      if (observer) observer.observe(article);
      else article.fill();
    }
    article.id = "card-" + (index + 1);
    Object.assign(item, { element: article, box });
    return item;
  };
  const show = (taken) => {
    transcript = taken;
    const { shell } = taken;
    document.title = "Transcript of " + shell.name + " - rotari";
    title.textContent = "Transcript of " + shell.name;
    about.textContent =
      [shell.project, "taken " + new Date(taken.takenAt).toLocaleString()]
        .filter(Boolean)
        .join(" · ") +
      (taken.partial
        ? " · older output is no longer kept by the terminal, so it is not here"
        : "");
    shown = taken.cards.map(cardElement);
    list.replaceChildren(...shown.map((item) => item.element));
    for (const saver of savers) saver.disabled = !shown.length;
    if (!shown.length)
      say("The terminal has no commands or output to show yet.");
    else applyFilter();
  };

  // Files. fence is a Markdown fence longer than any run of backquotes in text.
  const fence = (text) => {
    const longest = Math.max(
      0,
      ...(text.match(/`+/g) || []).map((run) => run.length),
    );
    return "`".repeat(Math.max(3, longest + 1));
  };
  const codeSpan = (text) => {
    const longest = Math.max(
      0,
      ...(text.match(/`+/g) || []).map((run) => run.length),
    );
    const ticks = "`".repeat(longest + 1);
    const pad = /^[` ]|[` ]$/.test(text) ? " " : "";
    return ticks + pad + text + pad + ticks;
  };
  const markdown = (cards) => {
    const { shell } = transcript;
    const out = [
      "# Transcript of " + shell.name,
      "",
      [
        shell.project,
        "taken " + transcript.takenAt,
        "from rotari's Web terminal",
      ]
        .filter(Boolean)
        .join(", ") + ".",
    ];
    for (const card of cards) {
      out.push("");
      if (card.command === null) out.push("## Output before the first prompt");
      else {
        const lines = card.command.split("\n");
        out.push("## " + codeSpan(lines[0] + (lines.length > 1 ? " …" : "")));
        if (lines.length > 1) {
          const marks = fence(card.command);
          out.push("", marks + "sh", card.command, marks);
        }
      }
      const meta = [
        card.dir ? codeSpan(card.dir) : "",
        card.command === null ? "" : stateWords(card),
        duration(card),
      ].filter(Boolean);
      if (meta.length) out.push("", meta.join(" · "));
      const text = runsText(card.runs);
      out.push("");
      if (text) {
        const marks = fence(text);
        out.push(marks + "text", text, marks);
      } else out.push("_No output._");
    }
    return out.join("\n") + "\n";
  };
  const escapeHTML = (text) =>
    text.replace(
      /[&<>"']/g,
      (c) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[c],
    );
  const savedCSS = [
    "body{margin:24px;background:#0d151e;color:#e7edf4;font:14px/1.5 system-ui,sans-serif}",
    "h1{font-size:20px;margin:0 0 4px}",
    ".about{color:#8b98a6;margin:0 0 20px}",
    "article{border:1px solid #2a3846;border-radius:6px;margin:0 0 14px;background:#111a24}",
    "article h2{font-size:14px;margin:0;padding:8px 12px;border-bottom:1px solid #2a3846}",
    "article h2 code,article h2 pre{font:13px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;margin:0;white-space:pre-wrap}",
    ".meta{margin:0;padding:6px 12px;color:#8b98a6;font-size:12px}",
    ".failed h2{box-shadow:inset 3px 0 0 #ff7b72}",
    ".ok h2{box-shadow:inset 3px 0 0 #7ee787}",
    "pre.output{margin:0;padding:10px 12px;background:#0b1118;color:#c9d1d9;font:12px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;white-space:pre-wrap;overflow-wrap:anywhere;border-radius:0 0 6px 6px}",
    ".empty{margin:0;padding:6px 12px 10px;color:#8b98a6;font-style:italic}",
  ].join("\n");
  const html = (cards) => {
    const { shell } = transcript;
    const parts = [
      "<!doctype html>",
      '<html lang="en"><head><meta charset="utf-8">',
      '<meta name="viewport" content="width=device-width,initial-scale=1">',
      "<title>" + escapeHTML("Transcript of " + shell.name) + "</title>",
      "<style>\n" + savedCSS + "\n</style></head><body>",
      "<h1>" + escapeHTML("Transcript of " + shell.name) + "</h1>",
      '<p class="about">' +
        escapeHTML(
          [
            shell.project,
            "taken " + transcript.takenAt,
            "from rotari's Web terminal",
          ]
            .filter(Boolean)
            .join(", ") + ".",
        ) +
        "</p>",
    ];
    for (const card of cards) {
      const multi = card.command && card.command.includes("\n");
      parts.push(
        '<article class="' +
          escapeHTML(card.state) +
          '"><h2>' +
          (multi ? "<pre>" : "<code>") +
          escapeHTML(commandTitle(card)) +
          (multi ? "</pre>" : "</code>") +
          "</h2>",
      );
      const meta = [stateWords(card), card.dir, duration(card)].filter(Boolean);
      parts.push('<p class="meta">' + escapeHTML(meta.join(" · ")) + "</p>");
      if (!card.runs.length) {
        parts.push('<p class="empty">No output.</p></article>');
        continue;
      }
      let output = "";
      for (const run of card.runs) {
        const css = runStyle(run.s, savedPalette, "#c9d1d9", "#0b1118");
        output += css.length
          ? '<span style="' +
            escapeHTML(css.map(([p, v]) => p + ":" + v).join(";")) +
            '">' +
            escapeHTML(run.t) +
            "</span>"
          : escapeHTML(run.t);
      }
      parts.push('<pre class="output">' + output + "</pre></article>");
    }
    parts.push("</body></html>");
    return parts.join("\n") + "\n";
  };
  const quote = (text) => "'" + text.replace(/'/g, "'\\''") + "'";
  const script = (cards) => {
    const { shell } = transcript;
    const out = [
      "#!/usr/bin/env bash",
      "# Commands of " +
        shell.name +
        (shell.project ? " (" + shell.project + ")" : "") +
        " from rotari's Web terminal, taken " +
        transcript.takenAt +
        ".",
      "# Their output is left out.",
    ];
    let dir = null;
    for (const card of cards) {
      if (!card.command) continue;
      out.push("");
      if (card.dir && card.dir !== dir) {
        out.push("cd " + quote(card.dir));
        dir = card.dir;
      }
      if (card.state === "failed")
        out.push("# Failed with exit " + card.exit + ":");
      else if (card.state === "running")
        out.push("# Still running when the transcript was taken:");
      out.push(card.command);
    }
    return out.join("\n") + "\n";
  };
  const fileStamp = (iso) =>
    iso.replace(/[-:]/g, "").replace(/\..*$/, "").replace("T", "-");
  const save = (kind) => {
    const cards = chosen();
    if (!transcript || !cards.length) {
      say("Check at least one card shown to save.");
      return;
    }
    const [content, type] =
      kind === "md"
        ? [markdown(cards), "text/markdown;charset=utf-8"]
        : kind === "html"
          ? [html(cards), "text/html;charset=utf-8"]
          : [script(cards), "text/x-shellscript;charset=utf-8"];
    const name =
      transcript.shell.name
        .replace(/[^A-Za-z0-9._-]+/g, "-")
        .replace(/^-+|-+$/g, "") || "shell";
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([content], { type }));
    link.download = name + "-" + fileStamp(transcript.takenAt) + "." + kind;
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(link.href), 60000);
    say(
      "Saved " +
        cards.length +
        (cards.length === 1 ? " card as " : " cards as ") +
        link.download +
        ".",
    );
  };

  // Asking the tab that opened this page.
  let waiting = null;
  const ask = () => {
    clearTimeout(waiting);
    if (!window.opener || window.opener.closed) {
      say(
        "This page has no terminal to ask. Open it from a terminal pane's menu, with Open transcript.",
      );
      for (const saver of savers) saver.disabled = !shown.length;
      refresh.disabled = true;
      return;
    }
    say("Asking the terminal…");
    window.opener.postMessage(
      { type: "rotari-transcript-ask" },
      location.origin,
    );
    waiting = setTimeout(
      () =>
        say(
          "The terminal that opened this page did not answer; it may have been closed or reloaded. Open the transcript again from its menu.",
        ),
      4000,
    );
  };
  window.addEventListener("message", (event) => {
    if (
      event.origin !== location.origin ||
      event.source !== window.opener ||
      event.data?.type !== "rotari-transcript"
    )
      return;
    clearTimeout(waiting);
    if (!event.data.transcript) {
      say(event.data.reason || "The terminal sent no transcript.");
      return;
    }
    show(event.data.transcript);
  });
  for (const saver of savers) saver.disabled = true;
  ask();
})();
