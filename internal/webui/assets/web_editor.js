// Open in EDITOR hands a file or directory of the server to an editor installed on the
// browser's machine, with a URL of the editor's scheme. The server runs nothing. A live page
// has an element that says the server's host name and what to suggest; a static export has
// none and offers nothing.
//
// The editor is the browser's, VS Code until another is chosen. A URL that goes over SSH
// needs the name the user's own ssh is given for the server, which the server cannot know,
// so the browser asks once and keeps the answer. One address may be forwarded to different
// servers, so it is kept per server host name.
(() => {
  // The editors, the first unless another is chosen. VS Code and those made from it take
  // one form of URL under schemes of their own. There is no URL of one's own to give: the
  // storage it would be kept in can be written by a launched service, on the same origin.
  const editors = [
    { id: "vscode", name: "VS Code" },
    { id: "vscodium", name: "VSCodium" },
    { id: "cursor", name: "Cursor" },
    { id: "zed", name: "Zed", zed: true },
  ];
  const editorKey = "rotari-editor";
  const keyPrefix = "rotari-editor-host-";
  const connection = () => document.getElementById("editor-connection");
  // A live page can offer it, and does unless this browser was told, in Settings, not to.
  // What is drawn once has the class editor-action, which the page's style then hides.
  const hiddenKey = "rotari-editor-hidden";
  const root = document.documentElement;
  const applyHidden = (hidden) => {
    root.toggleAttribute("data-editor-hidden", hidden);
    const shown = document.getElementById("editor-show");
    if (shown) shown.checked = !hidden;
    for (const id of ["editor-choice", "editor-connection-button"]) {
      const control = document.getElementById(id);
      if (control) control.disabled = hidden;
    }
  };
  const storedHidden = () => {
    try {
      return localStorage.getItem(hiddenKey) === "1";
    } catch {
      return false;
    }
  };
  applyHidden(storedHidden());
  // Another tab, or the terminal's window, follows.
  window.addEventListener("storage", (event) => {
    if (event.key === hiddenKey || event.key === null)
      applyHidden(storedHidden());
    if (event.key === editorKey || event.key === null) {
      readEditor();
      labelActions();
    }
  });
  window.editorAvailable = () => Boolean(connection());
  window.editorOffered = () =>
    window.editorAvailable() && !root.hasAttribute("data-editor-hidden");

  // editor is the editor in use, and editorProblem why it is not what was kept.
  let editor = editors[0];
  let editorProblem = "";
  const readEditor = () => {
    let kept = null;
    try {
      kept = localStorage.getItem(editorKey);
    } catch {}
    const found = editors.find((each) => each.id === kept);
    editor = found || editors[0];
    editorProblem =
      kept === null || found ? "" : "The kept editor is not one known here.";
  };
  // keepEditor uses an editor from now on, and returns why it could not be kept, or "".
  const keepEditor = (chosen) => {
    editor = chosen;
    editorProblem = "";
    labelActions();
    try {
      localStorage.setItem(editorKey, chosen.id);
      return "";
    } catch (error) {
      return error.message;
    }
  };
  // What is drawn once names the editor, and is named again when the editor changes. Its
  // title, label, and text are what data-editor-title, -label, and -text hold, then "in" the
  // editor, then, in a title, what data-editor-after holds.
  const labelAction = (element) => {
    const { editorTitle, editorLabel, editorText, editorAfter } =
      element.dataset;
    const named = (text) => text + " in " + editor.name;
    if (editorTitle !== undefined)
      element.title = named(editorTitle) + (editorAfter || "");
    if (editorLabel !== undefined)
      element.setAttribute("aria-label", named(editorLabel));
    if (editorText !== undefined) element.textContent = named(editorText);
  };
  const labelActions = () => {
    for (const element of document.querySelectorAll(
      "[data-editor-title], [data-editor-label], [data-editor-text]",
    ))
      labelAction(element);
    const chosen = document.getElementById("editor-choice");
    if (chosen) chosen.value = editor.id;
  };
  readEditor();
  window.editorName = () => editor.name;
  // labelEditorAction names an element: {title, label, text, after}, as labelAction.
  window.labelEditorAction = (element, { title, label, text, after }) => {
    if (title !== undefined) element.dataset.editorTitle = title;
    if (label !== undefined) element.dataset.editorLabel = label;
    if (text !== undefined) element.dataset.editorText = text;
    if (after) element.dataset.editorAfter = after;
    labelAction(element);
  };

  // nameProblem says why ssh should not be given a name: one starting with "-" would be an
  // option. It is checked at each use, since a launched service, on the same origin, can
  // write the storage.
  const nameProblem = (name) => {
    if (typeof name !== "string" || !name) return "Give the SSH host.";
    if (
      !/^([A-Za-z0-9._][A-Za-z0-9._-]*@)?[A-Za-z0-9._][A-Za-z0-9._-]*$/.test(
        name,
      )
    )
      return "An SSH host is letters, digits, dots, hyphens, and underscores, with USER@ before it if you need one, and does not start with a hyphen.";
    return "";
  };
  // storedChoice is the choice this browser kept for the server, or a problem to show in
  // the dialog, or neither when it has not been asked.
  const storedChoice = () => {
    const host = connection().dataset.host;
    if (!host)
      return {
        problem:
          "The server's host name is not known, so the choice is not kept.",
      };
    let kept;
    try {
      kept = localStorage.getItem(keyPrefix + host);
    } catch (error) {
      return { problem: "This browser keeps nothing here: " + error.message };
    }
    if (kept === null) return {};
    let choice;
    try {
      choice = JSON.parse(kept);
    } catch {}
    if (choice?.remote === false) return { choice: { remote: false } };
    if (choice?.remote !== true)
      return { problem: "The kept choice could not be read." };
    const problem = nameProblem(choice.name);
    return problem
      ? { problem: "The kept SSH host cannot be used. " + problem }
      : { choice: { remote: true, name: choice.name } };
  };
  // keepChoice returns why the choice could not be kept, or "".
  const keepChoice = (choice) => {
    const host = connection().dataset.host;
    if (!host) return "the server's host name is not known";
    try {
      localStorage.setItem(keyPrefix + host, JSON.stringify(choice));
      return "";
    } catch (error) {
      return error.message;
    }
  };

  const positive = (value) => Number.isSafeInteger(value) && value > 0;
  // targetProblem says why a target has no URL. Editors read a number after a colon as a
  // line, wherever it is in the path, so a path that has one cannot be named.
  const targetProblem = ({ path, folder, line, column }) => {
    if (typeof path !== "string" || !path.startsWith("/"))
      return "its path is not absolute";
    if (path.includes("\0")) return "its path holds a NUL character";
    if (
      path
        .split(":")
        .slice(1)
        .some((part) => part.trim() === "" || Number.isFinite(Number(part)))
    )
      return (
        editor.name +
        " would read the number after the colon in its path as a line"
      );
    if (line === undefined && column === undefined) return "";
    if (folder) return "a directory has no lines";
    if (!positive(line) || (column !== undefined && !positive(column)))
      return "its line or column is not a positive number";
    return "";
  };
  // editorURL is the URL that opens a target in the editor under a choice.
  const editorURL = (target, choice) => {
    const problem =
      targetProblem(target) || (choice.remote ? nameProblem(choice.name) : "");
    if (problem) throw new Error(problem);
    const path = target.path.split("/").map(encodeURIComponent).join("/");
    const at = (line) =>
      line ? ":" + line + (target.column ? ":" + target.column : "") : "";
    if (editor.zed)
      return (
        (choice.remote ? "zed://ssh/" + choice.name : "zed://file") +
        path +
        at(target.line)
      );
    // A remote path without a line at its end is taken for a directory, so a file without
    // a line is opened at its first.
    return (
      editor.id +
      (choice.remote
        ? "://vscode-remote/ssh-remote+" + choice.name
        : "://file") +
      path +
      at(target.line ?? (choice.remote && !target.folder ? 1 : 0)) +
      // A directory must not take the place of what a window has open.
      (target.folder ? "?windowId=_blank" : "")
    );
  };
  // cannotOpen says why a target was not opened.
  const cannotOpen = (target, reason) =>
    showNote(
      "Cannot open " + target.path + " in " + editor.name + ": " + reason + ".",
      false,
    );

  const element = (tag, className, text) => {
    const made = document.createElement(tag);
    if (className) made.className = className;
    if (text !== undefined) made.textContent = text;
    return made;
  };
  const button = (text, action) => {
    const made = element("button", "", text);
    made.type = "button";
    made.onclick = action;
    return made;
  };

  let note, noteTimer;
  const showNote = (message, changeable) => {
    if (!note) {
      note = element("div", "terminal-send-note editor-note");
      note.setAttribute("role", "status");
      document.body.append(note);
    }
    note.replaceChildren(element("span", "", message));
    if (changeable)
      note.append(
        button("Change", () => {
          note.hidden = true;
          window.changeEditorConnection();
        }),
      );
    note.hidden = false;
    clearTimeout(noteTimer);
    noteTimer = setTimeout(() => (note.hidden = true), 8000);
  };
  // open follows the URL. A page is not told whether anything opened, so the note says only
  // what was asked.
  const open = (target, choice, unkept) => {
    let url;
    try {
      url = editorURL(target, choice);
    } catch (error) {
      cannotOpen(target, error.message);
      return;
    }
    const link = element("a");
    link.href = url;
    link.hidden = true;
    document.body.append(link);
    link.click();
    link.remove();
    showNote(
      "Opening " +
        target.path +
        " in " +
        editor.name +
        " " +
        (choice.remote
          ? "through SSH host " + choice.name
          : "on this machine") +
        "." +
        (unkept ? " The choice was not kept: " + unkept + "." : ""),
      true,
    );
  };

  let dialog = null;
  // fillEditorSelect lists the editors, at the one in use.
  const fillEditorSelect = (select) => {
    for (const each of editors) {
      const option = element("option", "", each.name);
      option.value = each.id;
      select.append(option);
    }
    select.value = editor.id;
  };
  const selectedEditor = (select) =>
    editors.find((each) => each.id === select.value) || editors[0];
  // showDialog asks for the editor and where it runs, and then opens the target, if there is one.
  const showDialog = (target, problem) => {
    dialog?.remove();
    const before = document.activeElement;
    const suggested = connection().dataset;
    const { choice: kept } = storedChoice();
    const remoteFirst = kept ? kept.remote : suggested.remote === "true";
    dialog = element("div", "output-modal terminal-launch-modal editor-modal");
    dialog.setAttribute("role", "dialog");
    dialog.setAttribute("aria-modal", "true");
    dialog.setAttribute("aria-labelledby", "editor-dialog-title");
    const done = () => {
      dialog?.remove();
      dialog = null;
      if (before instanceof HTMLElement && before.isConnected) before.focus();
    };
    dialog.onclick = (event) => {
      if (event.target === dialog) done();
    };
    dialog.onkeydown = (event) => {
      event.stopPropagation();
      if (event.key !== "Escape") return;
      event.preventDefault();
      done();
    };
    const box = element("div", "output-panel terminal-launch");
    const head = element("header");
    const title = element(
      "strong",
      "",
      target ? "Open in editor" : "Editor connection",
    );
    title.id = "editor-dialog-title";
    head.append(title);
    const form = element("form", "editor-form");
    const editorRow = element("label", "editor-which", "Editor ");
    const which = element("select");
    fillEditorSelect(which);
    editorRow.append(which);
    form.append(editorRow);
    const radio = (label, checked) => {
      const row = element("label");
      const input = element("input");
      input.type = "radio";
      input.name = "editor-where";
      input.checked = checked;
      row.append(input, " " + label);
      form.append(row);
      return input;
    };
    const local = radio(
      "It runs on the same machine as rotari web",
      !remoteFirst,
    );
    const remote = radio("Connect over SSH", remoteFirst);
    const hostRow = element("label", "editor-host", "SSH host ");
    const name = element("input");
    name.type = "text";
    name.autocomplete = "off";
    name.spellcheck = false;
    name.value = kept?.name || suggested.name || "";
    hostRow.append(name);
    const hint = element(
      "p",
      "terminal-launch-hint",
      "The name you use with ssh on the machine this browser runs on, as in ssh " +
        (suggested.name || "HOST") +
        ", or USER@HOST. The editor asks for a password if ssh needs one; do not put it here.",
    );
    const says = element("p", "terminal-launch-problem", problem || "");
    says.setAttribute("role", "alert");
    const update = () => (name.disabled = !remote.checked);
    local.onchange = update;
    remote.onchange = update;
    update();
    const choices = element("div", "terminal-launch-choices");
    const accept = element("button", "", target ? "Open" : "Save");
    accept.type = "submit";
    choices.append(button("Cancel", done), accept);
    form.append(hostRow, hint, says, choices);
    form.onsubmit = (event) => {
      event.preventDefault();
      const choice = remote.checked
        ? { remote: true, name: name.value.trim() }
        : { remote: false };
      const refused = choice.remote ? nameProblem(choice.name) : "";
      if (refused) {
        says.textContent = refused;
        name.focus();
        return;
      }
      const unkept = keepEditor(selectedEditor(which)) || keepChoice(choice);
      done();
      if (target) open(target, choice, unkept);
      else
        showNote(
          unkept
            ? "The choice was not kept: " + unkept + "."
            : "Files open in " +
                editor.name +
                " " +
                (choice.remote
                  ? "through SSH host " + choice.name
                  : "on this machine") +
                ".",
          false,
        );
    };
    box.append(head, form);
    dialog.append(box);
    document.body.append(dialog);
    (remote.checked ? name : accept).focus();
  };
  window.changeEditorConnection = () =>
    showDialog(null, editorProblem || storedChoice().problem);
  // openInEditor opens an absolute path of the server: {path, folder, line, column}.
  window.openInEditor = (target) => {
    // A path linked in a log before this was hidden is text again.
    if (!window.editorOffered()) return;
    const problem = targetProblem(target);
    if (problem) {
      cannotOpen(target, problem);
      return;
    }
    const stored = storedChoice();
    if (stored.choice && !editorProblem) open(target, stored.choice, "");
    else showDialog(target, editorProblem || stored.problem);
  };
  window.showEditorNote = (message) => showNote(message, false);
  document.addEventListener("DOMContentLoaded", () => {
    applyHidden(root.hasAttribute("data-editor-hidden"));
    const shown = document.getElementById("editor-show");
    if (shown)
      shown.addEventListener("change", () => {
        const hidden = !shown.checked;
        applyHidden(hidden);
        try {
          if (hidden) localStorage.setItem(hiddenKey, "1");
          else localStorage.removeItem(hiddenKey);
        } catch (error) {
          showNote(
            "Open in editor is " +
              (hidden ? "hidden" : "shown") +
              " on this page only: " +
              error.message,
            false,
          );
        }
      });
    const chosen = document.getElementById("editor-choice");
    if (chosen) {
      fillEditorSelect(chosen);
      chosen.addEventListener("change", () => {
        const unkept = keepEditor(selectedEditor(chosen));
        if (unkept)
          showNote(
            "Files open in " +
              editor.name +
              " on this page only: " +
              unkept +
              ".",
            false,
          );
      });
    }
    labelActions();
    const settings = document.getElementById("editor-connection-button");
    if (settings)
      settings.addEventListener("click", () => {
        window.closeSettingsMenu?.();
        window.changeEditorConnection();
      });
  });
})();
