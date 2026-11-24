// Open in VS Code hands a file or directory of the server to the VS Code installed on the
// browser's machine, with a vscode:// URL. The server runs nothing. A live page has an
// element that says the server's host name and what to suggest; a static export has none
// and offers nothing.
//
// A URL for Remote SSH needs the name the user's own ssh is given for the server, which the
// server cannot know, so the browser asks once and keeps the answer. One address may be
// forwarded to different servers, so it is kept per server host name.
(() => {
  const keyPrefix = "rotari-vscode-";
  const connection = () => document.getElementById("vscode-connection");
  window.vscodeOffered = () => Boolean(connection());

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
  // targetProblem says why a target has no URL. VS Code reads a number after a colon as a
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
      return "VS Code would read the number after the colon in its path as a line";
    if (line === undefined && column === undefined) return "";
    if (folder) return "a directory has no lines";
    if (!positive(line) || (column !== undefined && !positive(column)))
      return "its line or column is not a positive number";
    return "";
  };
  // vscodeURL is the URL that opens a target under a choice.
  const vscodeURL = (target, choice) => {
    const problem =
      targetProblem(target) || (choice.remote ? nameProblem(choice.name) : "");
    if (problem) throw new Error(problem);
    const path = target.path.split("/").map(encodeURIComponent).join("/");
    // A remote file without a line at its end does not open as a file of the window it
    // belongs to, so one without a line is opened at its first.
    const line = target.line ?? (choice.remote && !target.folder ? 1 : 0);
    const position = line
      ? ":" + line + (target.column ? ":" + target.column : "")
      : "";
    return (
      (choice.remote
        ? "vscode://vscode-remote/ssh-remote+" + choice.name
        : "vscode://file") +
      path +
      position +
      // A directory must not take the place of what a window has open.
      (target.folder ? "?windowId=_blank" : "")
    );
  };

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
      note = element("div", "terminal-send-note vscode-note");
      note.setAttribute("role", "status");
      document.body.append(note);
    }
    note.replaceChildren(element("span", "", message));
    if (changeable)
      note.append(
        button("Change", () => {
          note.hidden = true;
          window.changeVSCodeConnection();
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
      url = vscodeURL(target, choice);
    } catch (error) {
      showNote(
        "Cannot open " + target.path + " in VS Code: " + error.message + ".",
        false,
      );
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
        " in VS Code " +
        (choice.remote
          ? "through SSH host " + choice.name
          : "on this machine") +
        "." +
        (unkept ? " The choice was not kept: " + unkept + "." : ""),
      true,
    );
  };

  let dialog = null;
  // showDialog asks where VS Code runs, and then opens the target, if there is one.
  const showDialog = (target, problem) => {
    dialog?.remove();
    const before = document.activeElement;
    const suggested = connection().dataset;
    const { choice: kept } = storedChoice();
    const remoteFirst = kept ? kept.remote : suggested.remote === "true";
    dialog = element("div", "output-modal terminal-launch-modal vscode-modal");
    dialog.setAttribute("role", "dialog");
    dialog.setAttribute("aria-modal", "true");
    dialog.setAttribute("aria-labelledby", "vscode-dialog-title");
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
      target ? "Open in VS Code" : "VS Code connection",
    );
    title.id = "vscode-dialog-title";
    head.append(title);
    const form = element("form", "vscode-form");
    const radio = (label, checked) => {
      const row = element("label");
      const input = element("input");
      input.type = "radio";
      input.name = "vscode-where";
      input.checked = checked;
      row.append(input, " " + label);
      form.append(row);
      return input;
    };
    const local = radio(
      "VS Code runs on the same machine as rotari web",
      !remoteFirst,
    );
    const remote = radio("Connect with Remote SSH", remoteFirst);
    const hostRow = element("label", "vscode-host", "SSH host ");
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
        ".",
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
      const unkept = keepChoice(choice);
      done();
      if (target) open(target, choice, unkept);
      else
        showNote(
          unkept
            ? "The VS Code connection was not kept: " + unkept + "."
            : "Open in VS Code now uses " +
                (choice.remote
                  ? "SSH host " + choice.name
                  : "VS Code on this machine") +
                ".",
          false,
        );
    };
    box.append(head, form);
    dialog.append(box);
    document.body.append(dialog);
    (remote.checked ? name : accept).focus();
  };
  window.changeVSCodeConnection = () =>
    showDialog(null, storedChoice().problem);
  // openInVSCode opens an absolute path of the server: {path, folder, line, column}.
  window.openInVSCode = (target) => {
    const problem = targetProblem(target);
    if (problem) {
      showNote(
        "Cannot open " + target.path + " in VS Code: " + problem + ".",
        false,
      );
      return;
    }
    const stored = storedChoice();
    if (stored.choice) open(target, stored.choice, "");
    else showDialog(target, stored.problem);
  };
  window.showVSCodeNote = (message) => showNote(message, false);
  document.addEventListener("DOMContentLoaded", () => {
    const settings = document.getElementById("vscode-connection-button");
    if (settings)
      settings.addEventListener("click", () => {
        window.closeSettingsMenu?.();
        window.changeVSCodeConnection();
      });
  });
})();
