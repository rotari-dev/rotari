// Shared document-preview markup used by artifact and Table viewer panes.
"use strict";

function pdfPreviewMarkup(url, title, frameClass) {
  const safeURL = highlightEscape(url);
  const safeTitle = highlightEscape(title);
  const classes = "artifact-pdf-frame" + (frameClass ? " " + frameClass : "");
  return (
    '<iframe class="' +
    classes +
    '" src="' +
    safeURL +
    '" title="PDF preview: ' +
    safeTitle +
    '" loading="lazy"></iframe>' +
    '<p class="meta"><a href="' +
    safeURL +
    '" target="_blank" rel="noopener">Open PDF in a new tab</a></p>'
  );
}
