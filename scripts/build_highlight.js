"use strict";

const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const packageDir = path.join(root, "node_modules", "@highlightjs", "cdn-assets");
const packageJSON = JSON.parse(
  fs.readFileSync(path.join(packageDir, "package.json"), "utf8"),
);
const source = fs.readFileSync(path.join(packageDir, "es", "highlight.min.js"), "utf8");
const exportPattern = /export\{([A-Za-z_$][\w$]*) as default\};\s*$/;
const match = source.match(exportPattern);
if (!match || !source.startsWith(`/*!\n  Highlight.js v${packageJSON.version} `)) {
  throw new Error(
    `Unexpected Highlight.js bundle format or version in ${packageDir}`,
  );
}

const outputDir = path.join(root, "internal", "webui", "assets", "vendor");
fs.mkdirSync(outputDir, { recursive: true });
const bundle = source.replace(exportPattern, `window.hljs = ${match[1]};`);
fs.writeFileSync(
  path.join(outputDir, "highlight.min.js"),
  `(function () {\n${bundle}\n})();\n`,
);
fs.copyFileSync(
  path.join(packageDir, "LICENSE"),
  path.join(outputDir, "highlightjs-license.txt"),
);
