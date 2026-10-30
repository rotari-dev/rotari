// Shared language selection and rendering for script panes and Markdown
// fences. highlight.min.js is the locally bundled Highlight.js runtime.
"use strict";

const highlightPreviewByteLimit = 2 * 1024 * 1024;

const syntaxLanguageAliases = new Map([
  ["py", "python"],
  ["python", "python"],
  ["sh", "bash"],
  ["bash", "bash"],
  ["zsh", "bash"],
  ["shell", "bash"],
  ["js", "javascript"],
  ["javascript", "javascript"],
  ["mjs", "javascript"],
  ["cjs", "javascript"],
  ["ts", "typescript"],
  ["typescript", "typescript"],
  ["tsx", "typescript"],
  ["go", "go"],
  ["golang", "go"],
  ["rs", "rust"],
  ["rust", "rust"],
  ["java", "java"],
  ["c", "c"],
  ["h", "c"],
  ["cc", "cpp"],
  ["cpp", "cpp"],
  ["c++", "cpp"],
  ["cxx", "cpp"],
  ["hpp", "cpp"],
  ["cs", "csharp"],
  ["csharp", "csharp"],
  ["c#", "csharp"],
  ["rb", "ruby"],
  ["ruby", "ruby"],
  ["php", "php"],
  ["swift", "swift"],
  ["kt", "kotlin"],
  ["kts", "kotlin"],
  ["kotlin", "kotlin"],
  ["lua", "lua"],
  ["pl", "perl"],
  ["perl", "perl"],
  ["r", "r"],
  ["sql", "sql"],
  ["json", "json"],
  ["jsonl", "json"],
  ["yaml", "yaml"],
  ["yml", "yaml"],
  ["xml", "xml"],
  ["html", "xml"],
  ["htm", "xml"],
  ["css", "css"],
  ["ini", "ini"],
  ["cfg", "ini"],
  ["conf", "ini"],
  ["toml", "toml"],
  ["dockerfile", "dockerfile"],
  ["makefile", "makefile"],
  ["diff", "diff"],
  ["patch", "diff"],
]);

function highlightEscape(text) {
  return String(text).replace(
    /[&<>"']/g,
    (character) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        character
      ],
  );
}

function scriptLanguage(value) {
  let label = String(value || "")
    .trim()
    .toLowerCase();
  if (!/[./\\]/.test(label)) label = label.split(/\s+/, 1)[0];
  const name = label
    .slice(label.lastIndexOf("/") + 1)
    .slice(label.lastIndexOf("\\") + 1);
  const dot = name.lastIndexOf(".");
  const extension = dot >= 0 ? name.slice(dot + 1) : name;
  const language = syntaxLanguageAliases.get(extension) || extension;
  return window.hljs && window.hljs.getLanguage(language) ? language : "";
}

function highlightScriptCode(source, language) {
  source = String(source);
  if (
    source.length > highlightPreviewByteLimit ||
    new TextEncoder().encode(source).length > highlightPreviewByteLimit
  ) {
    return highlightEscape(source);
  }
  const kind = scriptLanguage(language);
  if (!kind || !window.hljs) return highlightEscape(source);
  return window.hljs.highlight(source, {
    language: kind,
    ignoreIllegals: true,
  }).value;
}

// Paged text is highlighted page by page; the endpoint splits pages on line
// boundaries, so a token cannot be cut in the middle of a line.
function highlightScriptChunk(source, language) {
  return highlightScriptCode(source, language);
}
