# Plan: A Richer Table Viewer

Created: 2026-10-30
Status: Implementation is in the working copy; review remains. Tests and build
validation have not been completed.

## Purpose

Make the Table viewer useful for inspecting common analysis outputs and
source-adjacent artifacts, while keeping its existing table workflow,
offline operation, and artifact-root security rules.

## Scope

1. **JSON Lines tables.** Read one JSON value per nonblank line. Build sorted
   columns from the union of object keys; put arrays and top-level scalar
   values in a `value` column. Render strings and numbers as text, nested
   values as compact JSON, and null or missing values as blank. Support the
   existing paging, filtering, sorting, and Describe behavior, plus bounded
   static snapshots.
2. **Markdown previews.** Reuse the Web UI's safe Markdown renderer, provide
   a source toggle, and limit rendered content to 2 MiB. Keep links restricted
   to HTTP and HTTPS and escape source text before rendering.
3. **Syntax highlighting.** Use Highlight.js for common source and config
   languages in text panes and recognized Markdown fences. Keep language
   selection explicit by filename extension or fence label; do not rely on
   automatic detection. Bundle the browser build locally so the Web UI,
   Tableviewer, and static exports work offline.
4. **PDF previews.** Add PDF as a supported media type and use the browser's
   inline viewer, with a link to open the file in a new tab.
5. **Shared Web previews.** Make the Web artifact preview and both Table
   viewer entry points use the same Markdown renderer, syntax highlighter,
   and PDF preview behavior. Avoid a split where a feature works only after
   opening the Table viewer.
6. **Interface integration.** Offer these file types from Web artifact
   launches and `rotari tableview`; document the behavior and update the Web
   contracts. Allow JSON Lines tables and PDF cell media in static table
   snapshots. Direct Markdown, script, and PDF document snapshots remain out
   of scope.

## Non-goals

- Adding spreadsheet, Parquet, database, or other structured formats in this
  change.
- Implementing a general-purpose code editor or a complete CommonMark parser.
- Bundling a PDF engine or adding a remote script dependency.
- Rendering Markdown or script documents as standalone static table
  snapshots.

## Design and constraints

- Keep JSON Lines as a `tableview` source implementing the existing table
  source interface. Cache its index by path, size, and modification time, and
  keep periodic offsets so page reads do not restart at byte zero.
- Bound each JSON Lines record using the table viewer's record limit. Report
  malformed records and unsupported query parameters as errors.
- Keep file classification for table links separate from document preview
  classification. Preserve the shared `Target` path and root checks for all
  file reads, including PDF media and text pages.
- Use the existing Markdown renderer and the pinned local Highlight.js bundle.
  Add no remote script or stylesheet dependency; highlighted markup must be
  generated from source text by the library.
- Load the same local highlighter in the main Web UI and the Table viewer.
  The Web artifact preview must use the shared Markdown, highlighting, and
  PDF helpers, including in static exports where copied artifact contents
  are available.
- Keep the Highlight.js grammars in one pinned local browser bundle. Keep a
  small alias map for filename extensions such as `.py` and `.sh`; fence labels
  can use any language registered by the bundle.
- Treat native PDF rendering as browser-provided. A blank preview must still
  have an open-in-new-tab path.
- Keep static exports bounded by the existing row and media limits, and keep
  their limitations explicit in the UI and CLI help.

## Implementation phases

### Phase 1: Extend table sources and file classification

- Add JSON Lines indexing and row conversion in `internal/tableview`.
- Add JSON Lines to table-file patterns and classify Markdown, scripts, and
  PDFs for direct Table viewer launches.
- Add PDF to the shared media type map and preserve `Target` root checks.

### Phase 2: Add live previews

- Extend Table viewer info and handlers to distinguish tables, text
  documents, and PDFs; reject attempts to request table rows for documents.
- Add Markdown source/render mode, bounded syntax highlighting, and inline PDF
  panes. Replace the local scanner with Highlight.js while preserving the
  shared call path across previews. Reuse the existing
  paged text and media routes. Wire the same renderer, highlighter, and PDF
  preview into the Web artifact modal.
- Keep JSON Lines controls specific to that format and preserve view state.

### Phase 3: Integrate static snapshots and public interfaces

- Support JSON Lines in static table snapshots and copy referenced PDFs under
  the existing limits. Continue to refuse direct document snapshots.
- Extend CLI metadata and Web launcher patterns.
- Update the user guides, architecture notes, security notes, and WEB-19,
  WEB-20, and WEB-22 contract descriptions and status rows.

### Phase 4: Add coverage and validate

- Add table source tests for JSON Lines record parsing, column union,
  conversions, checkpoints, paging, filtering, sorting, and Describe.
- Add Web/API and browser coverage for direct previews, artifact-root
  refusals, safe Markdown, escaped highlighted text, PDFs, launcher offers,
  and static snapshots.
- Run the relevant Go formatting, package, browser/conformance, CLI
  generation, and repository validation commands. Mark contracts `partial`
  until conformance coverage checks the new behavior.

## Acceptance

- JSON Lines tables behave like existing tables for paging, filters, sorting,
  and Describe, and malformed or oversized records return explicit errors.
- Markdown, common Highlight.js source/config languages, and PDF files open from both artifact paths and
  `rotari tableview`; all reads stay under the source's allowed roots.
- Opening a supported file in the Web artifact preview or in Table viewer
  uses the same Markdown rendering, syntax highlighting, and PDF behavior.
- Markdown rendering and syntax highlighting cannot interpret file contents
  as executable HTML.
- Language selection maps filename extensions and Markdown fences to the
  installed Highlight.js grammars; no language is selected by code-content
  auto-detection.
- Static JSON Lines snapshots work within current limits and include copied
  PDF cell media; direct document snapshots are refused clearly.
- CLI help, Web launch options, user documentation, and contract notes agree
  with the implemented behavior.

## Current status

The initial implementation for Phases 1–3 is present in the working copy.
Tests and build validation have not been completed. The current environment
does not provide Go or `gofmt`; contract rows remain `partial` pending
conformance coverage. Use this plan to guide review and any follow-up changes.
