async function loadLogChunk(queue, run, job, attemptID, stream, before) {
  const response = await fetch(
    "/api/log?project_name=" +
      encodeURIComponent(queue) +
      "&run_id=" +
      encodeURIComponent(run) +
      "&job_id=" +
      encodeURIComponent(job) +
      "&stream=" +
      encodeURIComponent(stream) +
      (attemptID ? "&attempt_id=" + encodeURIComponent(attemptID) : "") +
      "&tail=200&before=" +
      before,
  );
  const text = await response.text();
  if (!response.ok) throw new Error(text.trim() || "HTTP " + response.status);
  return text;
}
// Paths in a job's log that name a file open in the editor, at the line printed
// with them. The lines in view are asked about, and the server says which
// paths exist; the log stays text otherwise.
const logLinkAskLimit = 64;
// A longer line is not searched.
const logLinkLineLimit = 4096;
// A path that is no file is asked about again, since a running job makes files.
const logLinkMissingLife = 10000;
let logLinkObserver = null;
// showLogOutput shows the text of the open log, and nothing else's.
function showLogOutput(output, text) {
  const log = selectedLog;
  // Drawing the same text again would lose a selection in it.
  if (log && log.drawn === text && output.textContent === text) return;
  logLinkObserver?.disconnect();
  logLinkObserver = null;
  if (log) log.drawn = text;
  if (
    !log ||
    log.error ||
    !window.editorOffered() ||
    typeof IntersectionObserver === "undefined"
  ) {
    output.textContent = text;
    return;
  }
  log.links ||= new Map();
  const content = document.createDocumentFragment();
  const searched = [];
  let plain = "";
  text.split("\n").forEach((line, index) => {
    if (index) plain += "\n";
    const candidates =
      line.length > logLinkLineLimit ? [] : window.outputPathCandidates(line);
    if (!candidates.length) {
      plain += line;
      return;
    }
    content.append(plain);
    plain = "";
    const element = document.createElement("span");
    element.textContent = line;
    searched.push({ element, line, candidates });
    content.append(element);
  });
  content.append(plain);
  output.replaceChildren(content);
  const bySpan = new Map(searched.map((item) => [item.element, item]));
  logLinkObserver = new IntersectionObserver(
    (entries, observer) => {
      const inView = entries.filter((entry) => entry.isIntersecting);
      for (const entry of inView) observer.unobserve(entry.target);
      if (inView.length)
        linkLogLines(
          log,
          inView.map((entry) => bySpan.get(entry.target)),
        );
    },
    { root: output },
  );
  for (const { element } of searched) logLinkObserver.observe(element);
}
// linkLogLines asks about lines' candidates and makes links of the answers.
async function linkLogLines(log, lines) {
  const now = Date.now();
  const unknown = [
    ...new Set(
      lines.flatMap((line) => line.candidates.map((item) => item.text)),
    ),
  ].filter((text) => !(log.links.get(text)?.until > now));
  for (let start = 0; start < unknown.length; start += logLinkAskLimit) {
    const candidates = unknown.slice(start, start + logLinkAskLimit);
    let answer;
    try {
      const response = await fetch("/api/log-links", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_name: log.queue,
          run_id: log.run,
          job_id: log.job,
          attempt_id: log.attemptID || "",
          candidates,
        }),
      });
      if (!response.ok)
        throw new Error(
          (await response.text()).trim() || "HTTP " + response.status,
        );
      answer = await response.json();
    } catch (error) {
      if (!log.linksFailed && log === selectedLog)
        window.showEditorNote(
          "Paths in this log are not links: " + error.message,
        );
      log.linksFailed = true;
      return;
    }
    const until = Date.now() + logLinkMissingLife;
    for (const text of candidates) log.links.set(text, { link: null, until });
    for (const link of answer.links || [])
      log.links.set(link.text, { link, until: Infinity });
  }
  if (log !== selectedLog) return;
  for (const line of lines)
    if (line.element.isConnected) linkLogLine(log, line);
}
// linkLogLine makes links of a line's paths, a longer one holding a shorter.
function linkLogLine(log, { element, line, candidates }) {
  const linked = candidates
    .map((item) => ({ ...item, link: log.links.get(item.text)?.link }))
    .filter((item) => item.link)
    .sort((a, b) => a.start - b.start || b.end - a.end);
  const content = document.createDocumentFragment();
  let at = 0;
  for (const { start, end, link } of linked) {
    if (start < at) continue;
    const path = document.createElement("span");
    path.className = "log-path";
    path.textContent = line.slice(start, end);
    window.labelEditorAction(path, {
      title: "Open " + link.path,
      after: link.line ? " at line " + link.line : "",
    });
    path.tabIndex = 0;
    path.setAttribute("role", "link");
    const open = () =>
      window.openInEditor({
        path: link.path,
        folder: false,
        line: link.line,
        column: link.column,
      });
    path.onclick = () => {
      // Selecting text ends in a click too.
      if (getSelection()?.isCollapsed ?? true) open();
    };
    path.onkeydown = (event) => {
      if (event.key === "Enter") open();
    };
    content.append(line.slice(at, start), path);
    at = end;
  }
  if (!at) return;
  content.append(line.slice(at));
  element.replaceChildren(content);
}
function attachLogLoader(output) {
  output.onscroll = async () => {
    if (
      output.scrollTop > 20 ||
      !selectedLog ||
      selectedLog.loading ||
      selectedLog.done ||
      selectedLog.error
    )
      return;
    if (followTimer) clearInterval(followTimer);
    followTimer = null;
    selectedLog.loading = true;
    const previousHeight = output.scrollHeight;
    let chunk = "";
    try {
      chunk = await loadLogChunk(
        selectedLog.queue,
        selectedLog.run,
        selectedLog.job,
        selectedLog.attemptID,
        selectedLog.stream,
        selectedLog.before + 200,
      );
    } catch (error) {
      chunk = "";
    }
    if (!chunk) {
      selectedLog.done = true;
    } else {
      selectedLog.before += 200;
      selectedOutput = chunk + selectedOutput;
      showLogOutput(output, selectedOutput);
      openOutputModal(isCompactOutput(selectedOutput));
      output.scrollTop = output.scrollHeight - previousHeight;
    }
    selectedLog.loading = false;
  };
}
async function showOriginalOutput(run, job, trigger) {
  const parts = pageParts();
  await showLog(decodeURIComponent(parts[1]), run, job, "");
}
async function showLog(queue, run, job, attemptID, stream, logMode) {
  const modal = document.getElementById("output-modal");
  modal.dataset.view = "log";
  logMode = logMode === "separate" ? "separate" : "merge";
  stream = stream || "stdout";
  modal.querySelector("strong").textContent =
    logMode === "separate" ? "Job log — " + stream : "Job log — merged";
  const streamSelect = document.getElementById("log-stream");
  streamSelect.value = stream;
  streamSelect.hidden = logMode !== "separate";
  if (followTimer) clearInterval(followTimer);
  selectedLog = {
    queue: queue,
    run: run,
    job: job,
    attemptID: attemptID,
    stream: stream,
    logMode: logMode,
    before: 0,
    loading: false,
    done: false,
    error: "",
  };
  try {
    selectedOutput = await loadLogChunk(queue, run, job, attemptID, stream, 0);
  } catch (error) {
    selectedLog.error = error.message;
    selectedOutput = "Failed to load log: " + error.message;
  }
  const output = ensureModalOutput();
  showLogOutput(output, selectedOutput);
  openOutputModal(isCompactOutput(selectedOutput));
  output.scrollTop = output.scrollHeight;
  attachLogLoader(output);
  followTimer = setInterval(followOutput, 2000);
}
// showNotes shows a job's notes, as the server formatted them, in the output
// modal, as showDiagnosis shows its analysis.
function showNotes(trigger) {
  if (followTimer) clearInterval(followTimer);
  followTimer = null;
  selectedLog = null;
  selectedOutput = JSON.parse(trigger.dataset.notes || "[]").join("\n\n");
  const output = ensureModalOutput();
  output.textContent = selectedOutput;
  const modal = document.getElementById("output-modal");
  modal.dataset.view = "notes";
  modal.querySelector("strong").textContent = "Notes";
  openOutputModal(true);
}

function showDiagnosis(trigger) {
  if (followTimer) clearInterval(followTimer);
  followTimer = null;
  selectedLog = null;
  const analysis = JSON.parse(trigger.dataset.diagnoses || "{}");
  const sections = (analysis.diagnoses || []).map(
    (item) =>
      item.name + "\nEvidence: " + item.evidence + "\nNext: " + item.suggestion,
  );
  if (analysis.status === "no_match") {
    sections.push(
      "No known rule matched.\nNext: " + diagnosisGuidance.noMatchNext,
    );
  } else if (analysis.status === "unavailable") {
    sections.push(
      "Unavailable: " +
        analysis.note +
        "\nNext: " +
        diagnosisGuidance.unavailableNext,
    );
  }
  if (analysis.outdated) {
    sections.push("Note: " + diagnosisGuidance.outdatedNote);
  }
  selectedOutput = sections.join("\n\n");
  const output = ensureModalOutput();
  output.textContent = selectedOutput;
  const modal = document.getElementById("output-modal");
  modal.dataset.view = "diagnosis";
  modal.querySelector("strong").textContent = "Diagnosis";
  openOutputModal(true);
}
async function followOutput() {
  if (!selectedLog || selectedLog.before > 0 || selectedLog.loading) return;
  let latest = "";
  try {
    latest = await loadLogChunk(
      selectedLog.queue,
      selectedLog.run,
      selectedLog.job,
      selectedLog.attemptID,
      selectedLog.stream,
      0,
    );
  } catch (error) {
    return;
  }
  if (latest && (selectedLog.error || latest !== selectedOutput)) {
    selectedLog.error = "";
    selectedOutput = latest;
    const output = ensureModalOutput();
    showLogOutput(output, latest);
    output.scrollTop = output.scrollHeight;
    openOutputModal(isCompactOutput(latest));
  }
}
async function log(queue, run, job, attemptID, logMode) {
  await showLog(queue, run, job, attemptID, "stdout", logMode);
}
async function changeLogStream(stream) {
  if (!selectedLog) return;
  await showLog(
    selectedLog.queue,
    selectedLog.run,
    selectedLog.job,
    selectedLog.attemptID,
    stream,
    selectedLog.logMode,
  );
}
function lastLogLines(value, count) {
  const lines = String(value || "").split("\n");
  return lines.slice(Math.max(0, lines.length - count)).join("\n");
}
async function copyText(value) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const area = document.createElement("textarea");
  area.value = value;
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.append(area);
  area.select();
  document.execCommand("copy");
  area.remove();
}
function copied(button) {
  const status = button
    .closest(".output-box")
    ?.querySelector(".modal-copy-status");
  clearTimeout(button.copyResetTimer);
  button.classList.add("copied");
  if (status) status.classList.add("visible");
  button.title = "Copied!";
  button.setAttribute("aria-label", "Copied!");
  button.innerHTML =
    '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"></path></svg>';
  button.copyResetTimer = setTimeout(() => {
    button.classList.remove("copied");
    if (status) status.classList.remove("visible");
    button.title = button.dataset.copyTitle;
    button.setAttribute("aria-label", button.dataset.copyTitle);
    button.innerHTML = button.dataset.copyIcon;
  }, 1200);
}
function copiedTextButton(button) {
  const label = button.textContent;
  button.textContent = "Copied";
  clearTimeout(button.copyResetTimer);
  button.copyResetTimer = setTimeout(() => (button.textContent = label), 1200);
}
async function fetchSelectedLog(tail) {
  if (!selectedLog || selectedLog.error) return selectedOutput;
  const suffix = tail ? "&tail=" + tail : "";
  const response = await fetch(
    "/api/log?project_name=" +
      encodeURIComponent(selectedLog.queue) +
      "&run_id=" +
      encodeURIComponent(selectedLog.run) +
      "&job_id=" +
      encodeURIComponent(selectedLog.job) +
      "&stream=" +
      encodeURIComponent(selectedLog.stream) +
      (selectedLog.attemptID
        ? "&attempt_id=" + encodeURIComponent(selectedLog.attemptID)
        : "") +
      suffix,
  );
  if (!response.ok) throw new Error(await response.text());
  const value = await response.text();
  return tail ? lastLogLines(value, tail) : value;
}
async function copyModalOutput(button) {
  try {
    await copyText(
      document.getElementById("output-modal").dataset.view === "log"
        ? await fetchSelectedLog(0)
        : selectedOutput,
    );
    copied(button);
  } catch (error) {
    alert(error.message);
  }
}
// sendModalOutput sends what the dialog's copy button copies. A path is one word of a
// command, and anything else is pasted text.
async function sendModalOutput() {
  const view = document.getElementById("output-modal").dataset.view;
  try {
    const text = view === "log" ? await fetchSelectedLog(0) : selectedOutput;
    await window.sendToTerminalAndSay(
      text,
      view === "path" ? { word: true } : { pasted: true },
    );
  } catch (error) {
    alert(error.message);
  }
}
async function sendLogTail() {
  try {
    await window.sendToTerminalAndSay(await fetchSelectedLog(100), {
      pasted: true,
    });
  } catch (error) {
    alert(error.message);
  }
}
async function copyLogTail(button) {
  try {
    await copyText(await fetchSelectedLog(100));
    copiedTextButton(button);
  } catch (error) {
    alert(error.message);
  }
}
function updateModalActions() {
  const modal = document.getElementById("output-modal");
  const view = modal.dataset.view;
  document.getElementById("log-stream").hidden =
    view !== "log" || !selectedLog || selectedLog.logMode !== "separate";
  const copyTail = document.getElementById("copy-tail");
  copyTail.hidden = view !== "log";
  copyTail.dataset.copyTitle = "Copy last 100 lines";
  copyTail.dataset.copyIcon ||= copyTail.innerHTML;
  const offersTerminal = Boolean(document.getElementById("terminal-button"));
  document.getElementById("send-tail").hidden =
    view !== "log" || !offersTerminal;
  document.getElementById("report-note").hidden = view !== "ai";
  const redactToggle = document.getElementById("report-redact-toggle");
  redactToggle.hidden = view !== "ai";
  const sourceToggle = document.getElementById("report-source-toggle");
  sourceToggle.hidden = view !== "ai";
  sourceToggle.textContent = reportShowsSource
    ? "Show rendered"
    : "Show Markdown";
  sourceToggle.setAttribute("aria-pressed", String(reportShowsSource));
  const copyButton = document.getElementById("copy-modal");
  copyButton.hidden =
    view === "generate-config" ||
    view === "notification-config" ||
    view === "notification-config-generate" ||
    modal.dataset.editing === "true";
  document.getElementById("send-modal").hidden =
    copyButton.hidden || !offersTerminal;
  document.getElementById("notification-config-save").hidden =
    view !== "notification-config" ||
    document.getElementById("notification-config-editor").dataset.editable !==
      "true";
  document.getElementById("notification-config-reload").hidden =
    view !== "notification-config" ||
    document.getElementById("notification-config-editor").dataset.editable !==
      "true";
  const copyTitle = view === "log" ? "Copy log" : "Copy";
  copyButton.title = copyTitle;
  copyButton.setAttribute("aria-label", copyTitle);
  copyButton.dataset.copyTitle = copyTitle;
  copyButton.dataset.copyIcon ||= copyButton.innerHTML;
}
// reportShowsSource is whether the report modal shows the report's
// Markdown source instead of rendering it.
let reportShowsSource = false;
// showReportOutput puts selectedOutput, the report's Markdown, in the
// modal both as source and rendered; openOutputModal shows one of them.
function showReportOutput() {
  ensureModalOutput().textContent = selectedOutput;
  document.getElementById("modal-markdown").innerHTML =
    renderMarkdown(selectedOutput);
}
function toggleReportSource() {
  reportShowsSource = !reportShowsSource;
  openOutputModal(false);
}
async function showAIReport(project, run, job, jobIDs) {
  const modal = document.getElementById("output-modal");
  modal.dataset.view = "ai";
  modal.querySelector("strong").textContent = job ? "Job report" : "Run report";
  selectedLog = null;
  selectedOutput = "Preparing...";
  reportShowsSource = false;
  showReportOutput();
  openOutputModal(false);
  selectedReportContext = { project, run, job, jobIDs };
  setReportRedactionButton(true);
  await fetchAIReport(true);
}
async function fetchAIReport(redact) {
  const { project, run, job, jobIDs } = selectedReportContext;
  const params = new URLSearchParams({
    project_name: project.project_name,
    run_id: run.run_id,
  });
  if (job) params.set("job_id", job.id);
  if (!job && jobIDs)
    jobIDs.forEach((jobID) => params.append("job_ids", jobID));
  if (!redact) params.set("redact", "false");
  const response = await fetch("/api/report?" + params);
  selectedOutput = await response.text();
  if (!response.ok)
    selectedOutput = "Failed to prepare report: " + selectedOutput;
  showReportOutput();
  openOutputModal(false);
}
function setReportRedactionButton(redact) {
  const button = document.getElementById("report-redact-toggle");
  button.textContent = `Redact: ${redact ? "On" : "Off"}`;
  button.setAttribute("aria-pressed", String(redact));
  button.dataset.redact = String(redact);
}
async function toggleReportRedaction() {
  if (!selectedReportContext) return;
  const redact =
    document.getElementById("report-redact-toggle").dataset.redact !== "true";
  setReportRedactionButton(redact);
  selectedOutput = "Preparing...";
  showReportOutput();
  await fetchAIReport(redact);
}
function ensureOutputBox() {
  let output = document.getElementById("log");
  if (!output) {
    output = document.createElement("pre");
    output.id = "log";
    output.className = "log";
    const main = document.querySelector("main");
    const pageTitle = document.getElementById("page-title");
    const section = pageTitle && pageTitle.parentElement;
    if (main && section) main.insertBefore(output, section);
  }
  return output;
}
function ensureModalOutput() {
  return document.getElementById("modal-log") || ensureOutputBox();
}
function openOutputModal(compact) {
  const modal = document.getElementById("output-modal");
  const view = modal.dataset.view;
  const readOnlyNote = document.getElementById("config-readonly-note");
  if (readOnlyNote) readOnlyNote.hidden = view !== "config-snapshot";
  document.getElementById("config-current-note").hidden =
    (view !== "config" && view !== "notification-config") ||
    pageParts()[2] !== "run";
  const configSource = document.getElementById("config-source-select");
  if (configSource) configSource.hidden = view !== "config";
  const editingConfig = view === "config" && modal.dataset.editing === "true";
  const editingNotificationConfig = view === "notification-config";
  const generatingConfig =
    view === "generate-config" || view === "notification-config-generate";
  const configPaths = document.getElementById("modal-config-paths");
  configPaths.hidden =
    (view !== "config" &&
      view !== "config-snapshot" &&
      view !== "notification-config") ||
    configPaths.childElementCount === 0;
  document
    .querySelector(".output-box")
    .classList.toggle("notification-config-output", editingNotificationConfig);
  const renderedReport = view === "ai" && !reportShowsSource;
  ensureModalOutput().hidden =
    editingConfig ||
    editingNotificationConfig ||
    generatingConfig ||
    view === "artifacts" ||
    view === "launches" ||
    view === "launcher-config" ||
    view === "recover" ||
    renderedReport;
  document.getElementById("modal-markdown").hidden = !renderedReport;
  document.getElementById("artifact-view").hidden = view !== "artifacts";
  document.getElementById("launch-view").hidden =
    view !== "launches" && view !== "launcher-config";
  document.getElementById("recover-view").hidden = view !== "recover";
  ensureModalOutput().classList.toggle(
    "log-error",
    view === "log" && !!selectedLog && !!selectedLog.error,
  );
  document.getElementById("config-editor").hidden = !editingConfig;
  document.getElementById("config-generator").hidden = !generatingConfig;
  document.getElementById("notification-config-editor").hidden =
    !editingNotificationConfig;
  if (!editingConfig) delete modal.dataset.editing;
  updateModalActions();
  modal.style.display = "flex";
  modal.querySelector(".output-panel").classList.toggle("compact", !!compact);
}
function closeOutputModal() {
  document.getElementById("output-modal").style.display = "none";
  delete document.getElementById("output-modal").dataset.view;
  delete document.getElementById("output-modal").dataset.editing;
  if (followTimer) clearInterval(followTimer);
  followTimer = null;
  selectedLog = null;
  selectedOutput = "";
  selectedReportContext = null;
}
function restoreSelectedOutput() {
  if (selectedLog && selectedOutput) {
    const output = ensureModalOutput();
    showLogOutput(output, selectedOutput);
    openOutputModal(isCompactOutput(selectedOutput));
    attachLogLoader(output);
  }
}
