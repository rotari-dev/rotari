# Web UI and static export

This document describes the Web UI, static export, and embedded asset
boundaries. Read it for Web/API/static-site changes in addition to
[00-overview.md](00-overview.md).

Representative implementation and tests:

- [internal/webui/assets.go](../internal/webui/assets.go) for embedded asset
  declarations.
- [internal/webui/webui.go](../internal/webui/webui.go) and
  [internal/webui/webui_test.go](../internal/webui/webui_test.go) for Web
  handlers and static export, and
  [cmd/rotari/web.go](../cmd/rotari/web.go) for the `web` command, which
  supplies the CLI metadata, environment definitions, and config template
  through `webui.Options`.
- [internal/webui/assets/web_app_core.js](../internal/webui/assets/web_app_core.js)
  and [internal/webui/assets/web_template.html](../internal/webui/assets/web_template.html)
  for the browser application and page shell.

## Job timeline

**WEB-1** A run timeline counts carried results at its initial point and
includes timestamped submission and completion events for jobs executed in
the run, even when those jobs have origin metadata. The initial point uses
the actual run start; an initial load sample corrects older summaries that
recorded a late start. The run page draws the timeline as one lane per job
(`lanes`, built by `BuildJobLanes` from the same inputs and finish rule):
an executed job spans its submission to its finish, a carried job and a job
not yet started are noted instead, and the axis starts at the initial point,
labelled with its clock time (and date when the run crosses a day), with the
rest marked as offsets from it. Its height stays within 260px whatever the job
count or parallelism: past 16 lanes' worth of jobs the lanes lose their names
and thin down to 2px, then share pixel rows, drawn failed over running over
success, and hovering a row lists its jobs. The shared
carried-result decision is in
[internal/runlineage](../internal/runlineage/runlineage.go); timeline
projection is in [internal/web](../internal/web/timeline.go) and display is in
[web_app_charts.js](../internal/webui/assets/web_app_charts.js) (`TestBuildJobLanes`,
`TestJobTimelineDrawsOneLanePerJob`, `TestJobTimelineCompressesManyJobs`), covered by
[`TestFilteredRerunCarriesCompletedResults`](../conformance/02-lifecycle/lifecycle_test.go) and
[`TestRunTimelineStartsAtActualRunStart`](../conformance/02-lifecycle/lifecycle_test.go).

## Activity

**WEB-10** `GET /api/activity?from=TIME&to=TIME` (RFC 3339) lists, across every
registered basedir, the runs whose span meets the range: from a run's start to
its finish, or to now while it runs, and only its start when it recorded no
finish (`Run.Span` and `Overlapping` in
[internal/runlist](../internal/runlist/runlist.go)). It lists the same runs,
lifecycles, and recorded times as `rotari runs` (`runlist.Collect`), with each
run's working directory, host, source revisions, and own notes
(`runlist.ReadDetails`). Every registered basedir is listed: one that no
longer exists is marked `missing`, and one whose runs cannot be read carries
its error while the others are still listed. A missing, malformed, reversed,
or longer than 45-day range is rejected with 400 rather than clamped. The
endpoint only reads, and only the runs that may meet the range: it skips,
without opening them, runs whose ID says they started at or after the range
and runs whose directory last changed before it, except a project's running
or interrupted run (`runlist.CollectBetween`). It also gives the latest run
start in any basedir (`latest_started_at`, from run IDs alone), so the page
opens on the latest day with runs in one more request. Implemented by `handleActivity` in
[activity.go](../internal/webui/activity.go); covered by
`TestActivityListsRunsAcrossBasedirs` and `TestActivityRejectsBadRanges` in
[activity_test.go](../internal/webui/activity_test.go), and
`TestActivityListsRunsAcrossRegisteredBasedirs` in
[conformance/05-web/activity_test.go](../conformance/05-web/activity_test.go).

The Activity page (`/activity/`, [web_app_activity.js](../internal/webui/assets/web_app_activity.js))
draws that response as a month calendar and one day's timeline. It sorts runs
into the browser's days by each run's `ends_at` (the server's span end), and
opens on today, or on the latest day with runs when today has none, saying so.
A day's lanes are one per (basedir, project), grouped by basedir and
collapsible, newest first, with lanes past twelve folded behind a "more"
button; bars are coloured by result and link to the run page. The eight
projects with the most running time in the shown month get the categorical
colours `--c1`–`--c8`, always beside their names; the rest share grey. A
missing or unreadable basedir is listed above the calendar. A static export
embeds every run of its basedir for the page and says the page shows that
basedir only. Covered by `TestActivityPage` in
[activity_page_test.go](../internal/webui/activity_page_test.go) and
`TestStaticExportIncludesActivity`.

## History search

**WEB-2** The live Web UI provides a dedicated history search across projects
in user-selected scopes. Each scope selects a registered basedir and may
narrow to one project and one run; multiple scopes are combined as a union.
A scope may instead select all basedirs (`all_basedirs` in the API), which
expands on the server to every registered basedir and cannot be narrowed to a
project or run. An all-basedirs search still returns the results it could read
and names in `unsearched` each basedir that is missing or could not be read; a
scope naming one basedir fails the search on such an error instead. Results of
a search over several basedirs name each result's basedir.
The dropdown choices load hierarchically from the selected basedir and project.
Search scope is independent of basedir checkboxes used to monitor browser
notifications. The result level (project, run, or job) is selected once for
the whole search. Each condition selects an attribute from that level or one
of its ancestors: project results expose project attributes, run results
expose run and project attributes, and job results expose job, run, and
project attributes. Run attributes include the run's host and working
directory; a job with no job-specific working directory uses its run's
working directory for job searches. Ignore-case matching is on by default and
can be disabled. Job status uses the shared persisted execution-status
projection, so interrupted-run attempts can be searched by recorded states
such as `running (recorded)`, `waiting (recorded)`, `pending`,
`suspended (recorded)`, or
`unknown`, without requiring a live supervisor.
Fuzzy matching is an independent, opt-in setting for free-text attributes; it
allows a small edit distance and ignores terms shorter than four characters.
Status, executor, and diagnosis conditions use dropdowns and exact value
matching. Diagnosis choices come from the built-in diagnosis rules plus the
Python exception fallback, not from scanning saved results. Other attributes
including job and run host and working directory accept search text and match
substrings by default. Additional
conditions are joined in displayed order with AND or OR, evaluated left-to-right.
Results always remain at the selected level. Results are
ordered by activity time with a
stable tie-break and paged in batches of 50. Time ranges apply to the selected
result level: run start/finish for runs, job finish/submission (falling back to
the run time) for jobs, and activity in a run or job for projects. The static
export includes the search route but explains that history search requires the
live Web UI. Search and dropdown-option APIs validate basedir IDs against the
server's registered-basedir allowlist and only read persisted state. Selecting
a job result opens its run detail, selects the table page containing that job,
and scrolls the matching row into view.

Search projection and condition evaluation live in
[`internal/web/search.go`](../internal/web/search.go); the API scanner is in
[`internal/webui/history_search.go`](../internal/webui/history_search.go),
and the page is in
[`web_app_search.js`](../internal/webui/assets/web_app_search.js). Package and
API coverage is in
[`internal/web/search_test.go`](../internal/web/search_test.go) and
[`internal/webui/webui_test.go`](../internal/webui/webui_test.go); the
cross-project and cross-basedir API behavior and recorded unfinished job status
are covered by
[`TestHistorySearchAcrossProjects`](../conformance/05-web/history_search_test.go)
[`TestHistorySearchFindsRecordedUnfinishedJobStatus`](../conformance/05-web/history_search_test.go),
and [`TestHistorySearchAllBasedirs`](../conformance/05-web/history_search_test.go).

## Asset layout

Web assets live under `internal/webui/assets/`:

```text
internal/webui/assets/
├── web_template.html
├── web_tokens.css
├── web_shell.js
├── web_styles.css
├── web_shell.css
├── web_app_core.js
├── web_app_actions.js
├── web_app_logs.js
├── vendor/
│   ├── highlight.min.js
│   └── highlightjs-license.txt
├── web_app_highlight.js
├── web_app_previews.js
├── web_app_markdown.js
├── web_app_artifacts.js
├── web_app_tables.js
├── web_app_charts.js
├── web_app_matrix.js
├── web_app_notifications.js
├── web_app_search.js
├── web_app_bootstrap.js
├── web_static_bootstrap.js
├── jobs_template.html
├── web_info_styles.css
├── favicon-dark.svg
├── favicon-light.svg
└── fonts/
```

`assets.go` embeds these files. The JavaScript files are concatenated in this
order and delivered as one script; they intentionally share the global scope:

1. `web_app_core.js`
2. `web_app_actions.js`
3. `web_app_logs.js`
4. `vendor/highlight.min.js`
5. `web_app_highlight.js`
6. `web_app_previews.js`
7. `web_app_markdown.js`
8. `web_app_artifacts.js`
9. `web_app_launches.js`
10. `web_app_tables.js`
11. `web_app_charts.js`
12. `web_app_matrix.js`
13. `web_app_notifications.js`
14. `web_app_search.js`
15. `web_app_activity.js`
16. `web_app_bootstrap.js`

The local Highlight.js browser bundle is generated from the pinned
`@highlightjs/cdn-assets` package (`package.json` and `package-lock.json`) by
[`scripts/build_highlight.js`](../scripts/build_highlight.js). Run
`npm run build:highlight` after changing its version. The bundle is embedded
in the Web UI, served to Tableviewer pages, and copied to Tableviewer static
snapshots; no CDN is required. Its BSD-3-Clause notice is available at
`/highlightjs-license.txt` and is copied into static exports.

This sequence is dependency order, not JavaScript module isolation: the
highlighter and PDF helper precede the Markdown renderer and artifact view.
`web_app_notifications.js` must load before `web_app_bootstrap.js`:
it wraps the global `refresh()` function, and bootstrap both calls `refresh()`
immediately and passes it to `setInterval`, so the wrap must already be in
place by then.

Colours and typefaces come from `web_tokens.css`: light values on `:root`,
dark values when the OS prefers dark and the viewer has not chosen light, or
when the viewer chose dark. Stylesheets and scripts set no colour literal and
name fonts only through `--font-sans` and `--font-mono`
(`TestStylesAndScriptsTakeColoursFromTokens`,
`TestStylesheetsTakeFontsFromTokens`). `web_shell.js` is inlined in `<head>`
of both page templates. It applies the viewer's System / Light / Dark choice,
kept per browser, before the page is drawn (`TestWebThemeChoice`), and runs the
page frame: the Settings menu, the navigation drawer, and the sidebar's width
(`TestWebShellMenuAndDrawer`). The fonts
are IBM Plex Sans and Mono, IBM's Latin-1 WOFF2 files under the SIL OFL
([fonts/README.md](../internal/webui/assets/fonts/README.md));
[fonts.go](../internal/webui/fonts.go) serves them and their licence under
`/fonts/` with `/web_fonts.css` and writes them once at a static export's
root (`TestLiveServerServesEmbeddedFonts`, `TestStaticExportWritesFontsOnce`).

A status is shown as a pill whose tone (ok, bad, run, warn, off) names its
colour and icon. `statusTones` in
[internal/webui/status.go](../internal/webui/status.go) is the only
status-to-tone rule: Go renders the Recent jobs page with it and injects it
into the application script, which normalises labels the same way
(`TestStatusToneMatchesInGoAndJS`).

A table renderer emits its table whole: columns, labels, and an `Actions`
cell built with `actionsCell` in `web_app_core.js`, whose buttons carry their
arguments in `data-` attributes. The cell is last in a row, except on the
project's queue table, where it comes first beside the row's editors. No later render step renames,
moves, or restyles a table's columns, and the run page's sections are drawn
whole by `renderRunGraphics` in `web_app_charts.js`. Of those sections the
failure causes section and the job timeline start open and the others closed;
one the viewer opens or closes stays so in that browser (`runSectionOpen`).
The failure causes section is drawn only for a run with failed or blocked
jobs and shows the run's `lineage_summary.failures` groups (CLI-4) as they
come: it does not regroup jobs. A cause's Select button checks exactly that
cause's jobs, and only the project's latest run, once finished, shows each
cause's retry preview, selected as the CLI's `retry:` hint selects it
(`TestWebRunPageShowsFailureCauses`). Layout lives in the
stylesheets; only data-driven sizes and colours are set inline.

The static export bootstrap is kept separately in `web_static_bootstrap.js`
because it provides the static fetch and routing adapters used only by
generated pages. It is formatted as ordinary JavaScript, then receives the
generated state, logs, and reports during export.

`/jobs/` is server-rendered from the same `joblist.Collect` path
([internal/joblist](../internal/joblist/joblist.go)) as `rotari jobs`:
it includes running jobs and jobs completed within the preceding 24 hours by
default, or up to the latest run when that ended earlier, and the live page
selects other windows and ranges as WEB-23 describes. Its static equivalent is
`jobs/index.html`, which lists that default with no range bar; preserve that page whenever changing the Web export layout.

The All projects and Recent jobs pages load the same page-frame stylesheet,
`web_shell.css` (sidebar, top bar, and status pills); static Web exports
include it beside each generated application page. Keep shared frame changes
in that asset rather than duplicating them in the page-specific stylesheets. `TestWebSidebarStylesAreSharedWithJobsPage`
and `TestGenerateStaticWebWritesProjectPages` in
[internal/webui/webui_test.go](../internal/webui/webui_test.go) cover the shared
asset and static copies.
The shared page frame uses the full viewport width without a maximum-width
cap, including in static exports; content padding and the mobile navigation
drawer remain in place (`TestWebShellUsesFullWidth` in
[internal/webui/shell_test.go](../internal/webui/shell_test.go)).

The live Web UI sidebar is hierarchical: registered basedirs (plus the
startup `--basedir`) contain projects, and projects contain runs. Other than
History search, users choose a project from the selected basedir's tree.
`/api/state` is a lightweight index: it lists project names,
run counts, the latest and immediately preceding run summaries, and running-lock
metadata without loading project queues or completed run details. It also
keeps the preceding summary and full details of currently running runs
across the selected basedir, which browser notifications used to read before
they came from recorded notifications (WEB-21). Opening
a project loads its queue and run summaries through `/api/project`; opening a
completed run loads its jobs, attempts, context, and timeline through `/api/run`.
`/api/active-runs` exposes the same index for other basedirs; no page reads
it now. Completed run details are cached in the browser for that
session and are not polled again. These API routes are bound to the basedir in
the URL mount. The cross-basedir `/api/history-search` route is the exception:
it accepts only basedir IDs from the registered allowlist. The mount identifier resolves
only to the startup basedir or a basedir in the read-only registry list;
unlisted IDs are not accepted. The static export remains a complete
single-basedir snapshot and has no basedir switching. Covered by
[`TestWebStateLoadsProjectAndRunDetailsOnDemand`](../internal/webui/webui_test.go),
[`TestWebSwitchesBetweenRegisteredBasedirs`](../internal/webui/webui_test.go),
and [`TestWebSidebarLazilyListsProjectsInOtherBasedirs`](../internal/webui/webui_test.go).

Both page kinds share one frame. A top bar holds breadcrumbs (the basedir,
then the project and run), the notification toggle, Refresh, and a Settings
menu with the theme choice and, on application pages, the config and
notification config actions, and at its foot which rotari serves the page
(WEB-37); the page title and its facts follow. The sidebar
starts with Views (History search), then the basedir list. Below 760px wide the
sidebar is a drawer opened from the top bar, and no page scrolls sideways.

The basedir list is introduced by a `Registered basedirs` heading so the tree
is self-describing. Each basedir entry is collapsible and only the basedir
currently mounted starts expanded; collapsing or expanding another entry is a
user choice the sidebar keeps. An entry, the breadcrumb naming a page's
basedir, and the history search's basedir choice label the basedir by its parent directory in bold and its own name,
with the full path in its tooltip, because basedirs usually share a name such
as `.rotari-state`; `newWebBaseDir` in [assets.go](../internal/webui/assets.go)
computes the label once for both page kinds
(`TestNewWebBaseDirLabelsByParentAndName`). `All projects` and `Recent jobs` are nested
inside each basedir entry rather than offered once at the top level, since both
pages describe a single basedir. `All projects` has its own disclosure control;
its project list is nested beneath it, and `Recent jobs` follows the project
tree. The sidebar scrolls independently of the page body, preserves its
manually selected position during background refreshes, and remembers the
position per basedir. A separator at the sidebar/content boundary is shown on
hover and can be dragged to resize the sidebar; a label too long for the
width ends with an ellipsis. These rules apply to both the application page
and the server-rendered Recent jobs page.

Each basedir row has a notification monitor checkbox, drawn as a bell. The selected basedir IDs
are stored in browser local storage under a key scoped to the current Web
server session. Restarting `rotari web` therefore discards the extra selected
basedirs and returns monitoring to the current basedir. When more than one
basedir is listed, a bell beside the `Registered basedirs` heading turns every
basedir's checkbox on, or off when all are already on; it shows a mixed state
while only some are on, and the selection it makes is stored the same way
(`TestWebSidebarLazilyListsProjectsInOtherBasedirs`,
`TestWebJobsPageTogglesAllBasedirNotifications`). The inbox reads the
recorded notifications of the selected basedirs (WEB-21); a newly selected
basedir's recorded history enters the inbox without desktop notifications.
Desktop notifications remain controlled by the global notification permission
and on/off toggle.

**WEB-12** `rotari web` reads the registered basedir list at startup and
rereads the basedir registry on every page load and on `GET /api/basedirs`,
which the Refresh button calls. A basedir registered or unregistered after the
server started therefore appears or disappears without a restart. A mount
whose basedir ID is not listed rereads the registry once before answering 404,
so a page opened directly at a newly registered basedir is served. When the
reread fails, `/api/basedirs` answers 500 and the previous list stays. The
ways to update the application page are ordered; each reads everything the
one before it reads:

1. The 2-second poll reads `/api/state`, the selected project when its
   revision changed, and the run details it needs. It does not reread the
   basedir list.
2. Refresh also reloads the selected project whatever its revision, rereads
   the basedir list, and drops the other basedirs' cached project lists, so an
   expanded basedir lists its projects again.
3. Reloading the page also discards every browser-side cache and the
   sidebar's expansion state.
4. Restarting `rotari web` also rereads its options and configuration and
   starts a new notification session.

The Recent jobs page's Refresh reloads the page. Covered by
[`TestWebRereadsRegisteredBasedirsWithoutRestart`](../internal/webui/basedir_reload_test.go),
[`TestWebRefreshButtonRereadsBasedirs`](../internal/webui/basedir_reload_test.go),
and [`TestWebListsBasedirsRegisteredAfterStart`](../conformance/05-web/basedir_reload_test.go).

## Web server and control-plane security

- `web` binds `--host`/`--port`, defaulting to `127.0.0.1:8787`.
- A busy default port scans upward for a free port. An explicit port,
  including `--port 0`, never falls back and fails immediately if unavailable.
- The actually bound address is reported after listener creation. Non-loopback
  hosts produce a warning when no Web UI token is configured.
- `--auth-token` or `ROTARI_WEB_AUTH_TOKEN` protects every live Web route with
  a login form and a revocable 12-hour session cookie; API clients authenticate
  with `Authorization: Bearer TOKEN` or `X-Rotari-Token: TOKEN`. Basic
  authentication is not supported. Browser login and session use require
  HTTPS (TLS, or `X-Forwarded-Proto: https` from a TLS-terminating proxy) or
  a direct loopback connection: a loopback client addressing a loopback
  `Host` (`localhost`, `127.0.0.1`, `[::1]`) with no `Forwarded`,
  `X-Forwarded-*`, or `X-Real-Ip` header, whose plain HTTP never leaves the
  host. The session cookie has the `Secure` attribute, except on that direct loopback
  connection, where Safari would discard it; see
  [why](../docs/SECURITY.md#why-the-session-cookie-has-no-secure-on-direct-localhost). API token authentication
  does not encrypt HTTP traffic and must only be used over a trusted network
  or HTTPS. The notification
  settings form masks a saved webhook URL in its input and never returns that
  URL in the read API response, but masking is only a visual safeguard: a URL
  entered in the form is sent to the Web server over the current HTTP
  connection when saved. Use HTTPS through a trusted reverse proxy for browser
  sessions; do not treat the password-style field or auth token as transport
  encryption.
- A non-loopback listener also exposes registered basedir paths; the warning
  without an auth token includes that path information.
- `loadWebState` ([internal/webui/webui.go](../internal/webui/webui.go)) exposes persisted runtime metadata for each project: `running.lock` fields and,
  in the project's `server`, whether its supervisor's `server.pid` exists and the PID it records. The panel does not query process
  liveness or infer that `state.lock` is held from the file's existence.
- A supervisor has no control surface of its own: only the `run` command that
  started it can send it requests, over inherited pipes, so reaching it does
  not depend on `ROTARI_PRIVATE_STATE` or on socket permissions.
- Web mutating routes are gated by `allowControl`, enabled by default and
  configurable with `--allow-control` or `ROTARI_WEB_ALLOW_CONTROL`.
  `--allow-control=false` rejects them with `403` before reading request bodies.
  Read-only `GET` routes and the read-only `POST /api/history-search` remain
  available.
- Web state exposes only whether an environment variable is set. Raw values
  never cross the HTTP boundary; `Value` is populated only by the local
  `rotari env` CLI command.

**WEB-35** Each live `rotari web` and `rotari tableview` listener appends a
`server_start` record to the private, bounded
`<masterdir>/web-events.jsonl`, naming its command, PID, version, executable
SHA-256 and modification time, bound host and port, authentication source,
and its applicable control, notification, terminal, launch, artifact-root,
and registered-basedir counts or flags. Servers protected by a token also
record allow-listed decisions for sign-in, refused sign-in transport,
missing browser sessions, rejected API credentials, and sign-out, including
only the outcome, transport class, bounded user agent, and whether an accepted
cookie was `Secure`. They never record a credential or cookie value, submitted
user name, URL, path, query, `next`, forwarding header value, client address,
or artifact-root path. Records are single-write JSON Lines, rotate at 1 MiB,
rate-limit each authentication event kind with the suppressed count on its
next record, and never make a request fail or wait for filesystem I/O; the
first write failure is warned once. Static exports record nothing. Implemented
by [`internal/weblog`](../internal/weblog/) and
[`internal/webui/auth.go`](../internal/webui/auth.go); the binary and secret
boundary are covered by `TestBugReportNeverIncludesSecrets` in
[`conformance/03-interfaces/bug_report_test.go`](../conformance/03-interfaces/bug_report_test.go),
with event, limit, concurrency, and failure cases in
[`internal/weblog/log_test.go`](../internal/weblog/log_test.go) and
[`internal/webui/auth_events_test.go`](../internal/webui/auth_events_test.go).

## Config viewing and generation

The Web config view, generation, and save contracts are described with the
rest of config handling in
[01-resolution-and-config.md](01-resolution-and-config.md#configuration-files).

## Desktop notifications

**WEB-21** The live main and Recent jobs pages keep an in-page inbox,
implemented once in [web_shell.js](../internal/webui/assets/web_shell.js), of
the notifications runs recorded under the master directory (NOT-2). Every two
seconds, and at once when a basedir bell changes, the page reads
`GET /api/notifications?basedirs=ID,...`
([notifications.go](../internal/webui/notifications.go)). The main page asks
for its own basedir and the other basedirs whose bell is on; Recent jobs asks
for the basedirs whose bell is on. An unknown basedir ID answers 400. The
server answers the latest 50 entries, newest first, each after its basedir's
cleared cursor, naming its basedir ID and rendered with its project's current
`[browser]` settings by `notificationlog.Entries`, plus the read and cleared
cursors of each requested basedir and a description of each record it could
not read, which the inbox shows. A cursor is a record's creation time and ID;
each basedir has its own pair in `inbox-state.json`, and an entry after its
basedir's read cursor is unread. Opening the inbox moves the read cursor of
each basedir it shows to that basedir's newest shown entry, and **Clear
notifications** moves both cursors there, through
`POST /api/notifications/state` with `{"basedirs": {ID: cursors}}`; other
basedirs' cursors do not move, so a basedir whose bell is turned on later
still shows its unread entries. The server only moves cursors forward, so
pages using the same master directory share them across reloads and
restarts. Without control (`--allow-control=false`) that POST answers 403 and
the page keeps its own cursors until it is reloaded. Entries are announced to
screen readers and shown as desktop notifications only when they first appear
in a later read for an unchanged basedir selection, so history present when
the page loads, or when a bell is turned on, never pops up. The inbox works
without the desktop API, permission, secure context, or desktop on/off
setting. Static exports have no inbox. Covered by
`TestNotificationInboxWithoutDesktopAPI` and
`TestNotificationInboxAnnouncesOnlyNewEntries` in
[run_notifications_test.go](../internal/webui/run_notifications_test.go), and
`TestNotificationsListSelectedBasedirsWithProjectSettings` and
`TestNotificationStateIsPerBasedirSharedAndNeedsControl` in
[notifications_test.go](../internal/webui/notifications_test.go). The built
binary is checked by `TestWebNotificationInboxWithoutDesktopPermission` and
`TestWebInboxShowsNotificationsRecordedWhileClosed` in
[notifications_test.go](../conformance/05-web/notifications_test.go).

`rotari web` and the run processes never talk to each other directly: runs
append records and the server reads them per request. The pages detect no
events themselves; the server itself records only the interrupted runs it
finds (NOT-4). A run reports a job once its retries end, as a failure
when its exit code is not 0 (including blocked jobs and jobs cancelled before
they start), and its own completion as a success only when its exit code is
0; events within one batch window share one entry (NOT-1).

Which events are shown comes from `notifications.toml`, whose `[browser]`
section uses the same keys and field vocabulary as `[webhook]` but holds its
own values. The server reads each record's project settings when it renders
the inbox, so a saved or reloaded file applies to open pages, and to entries
already recorded, on their next read without restarting the process.
`browser.job_failure`, `browser.job_success`, `browser.run_failure`, and
`browser.run_success` select the events, `browser.max_jobs` bounds the failed
and the succeeded jobs listed per entry, and the body is truncated at 1000
characters. `rotari web` still validates its basedir's file at startup and
`/api/notification-settings` serves a project's settings to the editor. The
static export has no server to read, save, or reload those files, so it
offers neither the notification editor nor live notifications.

The permission itself (`Notification.permission`) cannot be revoked from
JavaScript once granted, so the global on/off toggle is a separate
`localStorage` flag (`rotari-notifications-enabled`) checked before showing
each notification; it does not touch the browser's actual permission grant.
`--notifications`/`ROTARI_WEB_NOTIFICATIONS` (`webui.Options.Notifications`,
injected into the app bundle and the Recent jobs page) only seeds the
toggle's starting value for an origin that has never set the `localStorage`
flag; an explicit prior toggle click always wins. The static Recent jobs page
shows this local toggle too, but does not poll for live job events.

When the page cannot use notifications, because it is not a secure context
(plain HTTP from a non-loopback host) or the site's permission is denied, the
toggle on both the application and Recent jobs pages stays visible but
disabled, and its title gives the reason from `notificationUnavailableReason`
in `web_shell.js`. Covered by
`TestNotificationToggleExplainsWhyItIsUnavailable`.

## Recovering interrupted runs

**WEB-25** The live server previews and applies `rotari unlock` (SAFE-4)
for one project of the basedir in its URL mount through `project.Unlock`, as
the MCP preview and apply tools do
([unlock.go](../internal/webui/unlock.go)).
`GET /api/unlock?project_name=NAME[&run_id=ID][&assume_stopped=true]` is a
dry run: it answers the `UnlockResult` (`run_id`, `action`, `coordinator`,
`repair` with each execution's evidence, `allowed`, `refusal_code`,
`revision`, and the rest), plus `refusal` (the CLI's explanation) when
refused and `rerun_command` (`project.RerunCommand`) for a release. A
project with no interrupted run answers `allowed: true` and no `run_id`.
`POST /api/unlock` with `project_name`, optional `run_id`, the preview's
`if_revision` (required), and `assume_stopped` applies it; a refusal or a
changed project answers 409 with the result and `error`, a read-only server
answers 403, and an invalid name (`/` or `\`) or missing project answers
400. Neither route stops jobs.

The run page's toolbar and each interrupted run's row on the project page
have **Recover…**, which opens a dialog with the preview: Finalize or
Release, the coordinator's state and reason, each managed execution's state,
reason, source, and observation time, and the refusal. `coordinator_live`
and `execution_active` keep the apply button disabled. For
`coordinator_unknown` and `execution_unknown` it offers a tick ("I confirmed
that the coordinator and all of the run's jobs have stopped"); ticking it
previews again with `assume_stopped=true`, and only then can the user apply.
The apply sends the preview's revision and the tick; a 409 asks for a new
preview. After a release the dialog names the rerun command and starts
nothing. Covered by `TestWebUnlockPreviewsAndAppliesThroughProjectUnlock`
and `TestRecoverDialogFollowsThePreview` in
[unlock_test.go](../internal/webui/unlock_test.go), and through the binary
by the Web variants of `TestUnlockFinalizationRepair` and by
`TestUnlockRefusesLiveExecutions`.

## Web UI model and reports

The Web UI is a projection of the same persisted model, not a separate
database. `show --report`, the Web UI's `/api/report`, and static Web
generation all use the same Go formatter, `internal/report`, for run and job
reports. A run report is the run's record for people and agents: its sources,
its run notes, a table of its jobs, and each job's notes and evidence. The
report modal renders the Markdown, escaping all text and keeping only http(s)
links, and shows the source on request; Copy gives the Markdown. rotari does
not transmit or submit the report. Covered by
`TestRunReportRecordsSourcesNotesAndJobTable`,
`TestRenderMarkdownRendersReportsAndEscapesText`, and
`TestStaticWebReportRedactionToggle`.

The run page's jobs table (`runJobsHTML` in `web_app_core.js`) shows each
job's command under its name, as the Recent jobs page does. Both show an
attempt ID by its end (`.id-tail` in `web_shell.css`, written by
`idTailHTML` in `web_app_core.js` and `assets.go`), while its text, title,
and copy button keep the whole ID. Hosts, Executor options, and Working
directory have a copy button when not empty. Every column stays in the table; a column whose
values are the same for every job, and Job ID and Finished, start hidden, and the Columns
menu above the table turns any column but Job, Status, and Exit on or off,
remembered per browser. A row ends with all its actions as buttons, without a menu, in lines of
four: Log, Artifacts (on a job that has started, for the attempt Log shows;
disabled before; its rows, the working directory first, have the launch
buttons, so there is no separate Launch), Diagnosis on a failed job,
Notes (disabled when the job has none), Report, and Path. Covered by
`TestWebRunJobsTable` in
[run_jobs_test.go](../internal/webui/run_jobs_test.go) and
`TestWebLaunchWorkingDirectoryWithoutCandidatesInBrowser` in
[launch_browser_test.go](../internal/webui/launch_browser_test.go). The project's queue
table follows the same column rule, implemented once in `columnTable`: Job
name / ID and Command always shown, the source times hidden, and the rest
shown when they differ; a hidden column's editors stay in the row, so Save
still sends their values. Covered by `TestWebQueueTable` in
[queue_table_test.go](../internal/webui/queue_table_test.go).

**WEB-11** Each row of the All projects page ends with Path and Delete.
Delete asks for confirmation and then deletes the whole project through
`POST /api/delete-project` with `{"project_name": NAME}`, which calls the
same `Editor.DeleteProject` as `delete --project` (SAFE-12) and returns its
message and any warnings. The button is disabled for a project the page
shows as running; the server rejects a running or interrupted project and
an invalid name, and a read-only server or static export answers `403`.
`deleteProjectButton` in
[web_app_core.js](../internal/webui/assets/web_app_core.js) and
`deleteProject` in
[web_app_actions.js](../internal/webui/assets/web_app_actions.js); covered by
`TestWebAllProjectsDeleteProject` in
[delete_project_test.go](../internal/webui/delete_project_test.go),
`TestWebDeleteProjectEndpoint` in
[webui_test.go](../internal/webui/webui_test.go), and
`TestDeleteProjectRemovesOnlyThatProject` in
[conformance/04-coordination/private_state_test.go](../conformance/04-coordination/private_state_test.go).

Run job projections include each persisted attempt. The jobs table defaults to
the latest attempt, but stores a browser-local selection per job so rows can
independently display a prior attempt's result, timestamps, and log. The log
endpoint validates that an optional attempt ID belongs to its requested run and
job before reading that attempt directory.

The run page keeps Cancel and Suspend/Resume out of individual job rows.
The selection buttons and a count of checked jobs sit beside the Jobs heading,
and the toolbar under it (`runSelectionHTML` and `runActionsHTML` in
`web_app_core.js`) is rendered with the table.
The toolbar actions after Report and before the run's own actions (a button
per command for the directory holding the basedir where launching is
available, Cancel run while the run is running, and Delete run) operate on
the checked jobs:
Cancel includes selected pending, running, and suspended jobs; Suspend targets
selected running jobs; Resume targets selected suspended jobs. Cancel and
Suspend stay disabled unless at least one checked job is running, while Resume
is enabled when a checked job is suspended. The Web API accepts one `job_id`
for existing callers or a `job_ids` list for these bulk actions. The browser
selection and action state are implemented in
[`web_app_actions.js`](../internal/webui/assets/web_app_actions.js) and
[`web_app_logs.js`](../internal/webui/assets/web_app_logs.js); the API validates
IDs in [`webui.go`](../internal/webui/webui.go). Covered by
[`TestRunBulkControlsOperateOnSelectedJobs`](../internal/webui/webui_test.go)
and [`TestWebJobControlRejectsStaleRunID`](../internal/webui/webui_test.go).

A run page adds one collapsible section per matrix group, collapsed by default,
between the run graphics and the job table controls
([web_app_matrix.js](../internal/webui/assets/web_app_matrix.js)).
`addMatrixPanels` runs at the end of each render, like the other run
sections, and its header shows the group's success and failure counts.
`LoadJobs` attaches each member's group ID, base name, dimensions, and values
from the run's command snapshot; it leaves out the base environment, which may
hold secrets. Rows and columns default to the first two dimensions and can be
switched with the section's selectors (choosing the other axis's dimension
swaps them); remaining dimensions split the group into one grid per
combination. Cells are keyed by values in dimension order, so any axis choice
finds the same jobs. Expansion and axis choices are kept per group while the
page is open. A cell takes the worst state of its jobs (array tasks share a
cell and show `succeeded/total`), classified from the same resolved `result`
as the jobs table. Clicking a cell opens a box, attached to `document.body` so
the periodic re-render does not remove it, that lists each of the cell's jobs
with a job-name copy button and a copy of its table row's action buttons plus
`Show in table`. A copy presses the button at the same position in the current
row, so every action, including ones added by later render steps, behaves
exactly as in the table. `restoreMatrixActions` moves the box to the
re-rendered cell, rebuilds it when the cell's jobs changed, or closes it when
the cell is gone or collapsed. Jobs whose matrix provenance was cleared by a
partial copy or change appear only in the table. Covered by `TestWebRunViewDrawsMatrixGrid` in
[cmd/rotari/web_test.go](../cmd/rotari/web_test.go).

Long values in the Command, Working directory, Dependencies, and Executor
options columns of the job and queue tables start clamped to three lines, and
the command under a run page's job name to two
(`clampLongTableCells` in
[web_app_tables.js](../internal/webui/assets/web_app_tables.js)). Clicking the
text, which highlights on hover, or pressing Enter or Space on it expands or
collapses it; a click that ends a text selection does not, so values can still
be selected. A cell is
considered once its text exceeds 60 characters, and it is left unclamped when
the browser measures that the text already fits. Buttons such as copy icons
stay outside the clamped text, cells with editors are left alone, and expanded
cells stay open across re-renders. Covered by `TestWebRunViewClampsLongCells`.

Report generation redacts known hostnames and paths, then applies heuristic
redaction to common absolute paths and FQDNs in log text. This is best-effort
privacy protection, not complete secret detection; users must review reports
before sharing them externally. Both `rotari web` and the static export expose
a `Redact: On` / `Redact: Off` button in the report modal. Turning it off
requests the unredacted report; the static export includes both report variants
in its generated files, so treat the export as containing sensitive data even
when the UI initially displays redacted reports. Redaction is on by default.

## Runtime and static mode

The ordinary Web UI and GitHub Pages static demo use the same JavaScript.
`pageParts()` abstracts route parsing:

`staticPath()` prefixes links with the repository base path, and
`rewriteStaticLinks()` repairs dynamically created absolute links. Keep links in
the HTML template relative where possible.

Static export injects a bootstrap before the app script. The bootstrap provides
persisted state, logs, reports, configuration files, config-generation targets,
and defines `routeParts()`. For a selected-job report, it returns only the
selected jobs' pre-generated reports, rather than the whole run report. The
static app uses the same UI functions and request flow as the ordinary Web UI
whenever possible, including configuration viewing and editing. Requests that
would mutate state or files return the same read-only `403` message as a server
started with `--allow-control=false`. The bootstrap must run before the app
script: otherwise the first state request can hit GitHub Pages' 404 document
and briefly render that HTML as application text.

Static pages receive `web_fonts.css`, `web_tokens.css`, `web_styles.css`, and
`web_shell.css` beside every generated `index.html`; the fonts
themselves are written once, under `fonts/` at the export root. If a new asset
or static API endpoint is added, update both the normal Web handler and
`webui.GenerateStatic`/its bootstrap.

**WEB-9** A page of the static export shows what the live server's page
shows: the same tables, columns, row actions, and panels, with controls
answered by the read-only `403`. A page opened from disk has a path ending in
`index.html`; `routeParts()` drops that file name, so the page is routed like
its live path. Implemented by `routeParts` in
[web_static_bootstrap.js](../internal/webui/assets/web_static_bootstrap.js)
and the table renderers in
[web_app_core.js](../internal/webui/assets/web_app_core.js); covered for the
projects overview and a project page by
`TestStaticExportPagesShowWhatLivePagesShow` in
[conformance/05-web/static_pages_test.go](../conformance/05-web/static_pages_test.go).

## Web CLI modes

**WEB-3** `rotari web --static-dir DIR` writes a static export and exits; it
does not start the live HTTP server. Options that configure that server
(`--host`, `--port`, `--auth-token`, `--artifact-root`), its control endpoints
(`--allow-control`), launching (`--allow-exec`, `--allowed-host`), or the
terminal (`--allow-terminal` and the `--terminal-*` options) are
rejected when explicitly supplied by flag, environment, or config, rather than
accepted and silently ignored. A value a workspace, basedir, or project config
may not supply (WEB-36) is not supplied: it is ignored with its warning, and
the export goes on. An `auth-token` from the environment or any config is
likewise ignored with a warning naming where it is, never the token: it
protects a live server, and an export has none. `--auth-token` on the
command line is still refused. The shared web command
boundary is in [cmd/rotari/web.go](../cmd/rotari/web.go); static-mode pair and
standalone checks are in
[conformance/03-interfaces/pairweb/web_static_pairs_test.go](../conformance/03-interfaces/pairweb/web_static_pairs_test.go).

**WEB-4** The static export's initial browser notification toggle, on both the
application and Recent jobs pages, follows `--notifications`. The Recent
jobs page does not poll for live events. Static export does not load saved
notification settings or provide live event polling: it has no server. The
markers are rendered by [internal/webui/assets.go](../internal/webui/assets.go)
and checked by `TestCLIFlagPairWebNotificationsEffect` in
[conformance/03-interfaces/pairweb/web_static_pairs_test.go](../conformance/03-interfaces/pairweb/web_static_pairs_test.go).

**WEB-5** A job row's Artifacts action shows the artifact candidates of the
attempt its Log button shows, laid out as `show -j JOB --artifacts`
prints them (CLI-16). `GET /api/artifacts?project_name=P&run_id=R&job_id=J`
(with an optional `attempt_id` of that job) returns the same
`jobstatus.ListArtifacts` JSON that `show -j J --json` carries, and rejects
an unknown job or another job's attempt. The static export embeds the listing
of every job's latest attempt and of each of its attempts, so the button
works there too; what each path holds is observed when the export is made.
The handler is in [internal/webui/artifacts.go](../internal/webui/artifacts.go)
and the view in `showArtifacts` in
[web_app_artifacts.js](../internal/webui/assets/web_app_artifacts.js); covered by
`TestWebShowsArtifactCandidates` in
[conformance/05-web/artifacts_test.go](../conformance/05-web/artifacts_test.go).

**WEB-6** The live server previews and downloads an artifact candidate's
content only when (a) the request names an entry of the attempt's recorded
listing by index, or its recorded absolute working directory as entry `wd`,
or, with a project and an existing run but no job or attempt, the directory
holding the basedir as entry `basedir_parent` (the run page's Files view),
optionally with a clean relative child path inside a listed
directory, never a free path; (b) the path is under an
`rotari web --artifact-root DIR`, or, when no other user could have chosen
the listing's paths, under the job's recorded working directory or the
directory holding its basedir (never `/`); and (c) opening it through
`os.Root` does not leave that root, including through a symlink. No other
user could have chosen the paths when the basedir, its `projects`
directory, the project's directory, `queue.json`, `meta.json`, `runs`, and the
run's directory are each owned by the Web server's user or root and not
writable by group or others (permission bits only; an ACL is not seen):
whoever can write those can make a job record any working directory and
path. Otherwise `/api/artifacts` adds `roots_note`, which says why only
`--artifact-root` is served, and the Web UI shows it.
`/api/artifacts` adds `previewable` per entry (an empty list for a listing
without candidates; left out only by a static export without copied
contents), and `working_directory_previewable` for the working directory,
which the view lists first as `.`, also when there are no candidates (type
from `working_directory_type`, which the CLI's
listing JSON carries too) and which is not a candidate: it is served under
the same roots, so not for an untrusted record without `--artifact-root`;
a refused request is 403, a
missing file 404. Images (png, jpg, jpeg, gif, svg), audio (wav, mp3, flac,
ogg, oga, opus, m4a), video (mp4, webm, mov), and PDF files are served inline, with
range requests so media can seek, and every other file, or any file with
`download=1`, as an attachment, all with `Cache-Control: no-store`,
`X-Content-Type-Options: nosniff`, and a `sandbox` Content-Security-Policy,
so an SVG's scripts never run. `/api/artifact-array` describes a `.npy`
file, or each of up to 200 arrays in a `.npz` file, from their headers:
dtype, shape, order, and the first 50 values in memory order for boolean,
integer, and floating dtypes; object arrays are pickles and their values are
never read. Files are
streamed without a size limit; text is served 64 KiB at a time on line
boundaries, from the start or, for `.log` and `.txt` in the UI, from the end,
and a file with a NUL byte is not text. The live UI offers **Open as text**
for a file of no previewed type, such as one without an extension, reading
it from the start; a static export offers no such button. A directory lists its immediate
children, directories first and then by name, 200 per page, reading at most
100,000 names. `--artifact-root` must name a directory and is rejected with
`--static-dir`; static exports contain listings but no file contents unless
WEB-7 applies. The
handlers are in [internal/webui/artifact_files.go](../internal/webui/artifact_files.go)
and the view in [web_app_artifacts.js](../internal/webui/assets/web_app_artifacts.js).
The artifact modal and Table viewer share the local Markdown renderer,
Highlight.js highlighter, and PDF iframe helper from
[`web_app_markdown.js`](../internal/webui/assets/web_app_markdown.js),
[`web_app_highlight.js`](../internal/webui/assets/web_app_highlight.js), and
[`web_app_previews.js`](../internal/webui/assets/web_app_previews.js). Markdown
rendering and syntax highlighting are each limited to 2 MiB; larger previews
show source. Highlight.js uses an explicit language selected from a filename
extension or fence label; it does not auto-detect. PDFs use the
browser's inline viewer. The same scripts are included in static Web exports
when artifact contents are copied; covered by
`TestWebPreviewsArtifactsUnderAllowedRoots` and
`TestWebServesArtifactsOnlyFromTrustedRecords` in
[conformance/05-web/artifacts_test.go](../conformance/05-web/artifacts_test.go).
The Table viewer (WEB-19) also serves, inline with the same headers, the
image, audio, video, and PDF files a served table's cells name, and any such
file as text pages, only under the same roots; the ownership rule is `artifactRecordsTrust` in
[internal/webui/artifact_trust.go](../internal/webui/artifact_trust.go).
Opening an entry, including a directory child, scrolls to the rendered preview
below the listing; loading additional pages does not scroll it. This browser
behavior is checked by `TestWebArtifactPreviewInBrowser` in
[internal/webui/artifact_preview_test.go](../internal/webui/artifact_preview_test.go).

**WEB-7** `rotari web --static-dir DIR --static-artifact-contents` copies the
contents of the candidates WEB-6 would serve without `--artifact-root`
(which static export rejects), into `DIR/artifact-files/`: each file of at most
10 MiB, up to 100 MiB in all, one copy per file however many attempts list it.
It embeds the first answer of the other previews (a text file of at most
1 MiB as one page, an `.npy`/`.npz` description, a directory's first page),
marks those entries previewable, and prints how many files and bytes it
copied when it copied any. Opening a directory's children or later pages in
the export says they are not included. The flag requires `--static-dir`.
Without it, a static export has no file contents. Implemented in
[internal/webui/static_artifacts.go](../internal/webui/static_artifacts.go)
and `staticArtifactFileURL` in
[web_static_bootstrap.js](../internal/webui/assets/web_static_bootstrap.js);
covered by `TestStaticExportCopiesArtifactContents` in
[conformance/05-web/artifacts_test.go](../conformance/05-web/artifacts_test.go).

**WEB-8** A run report, from `show RUN_ID --report`, `/api/report`, or the
static export, reads each job's log, for its job table's last log line and
its log excerpt, from the attempt that produced the job's result: a job
carried into a retry is read from the run it was carried from, through
`jobstatus.ResultAttemptDir`, the lookup `FilterJob` uses. Implemented by
`readReportLog` in [internal/report/report.go](../internal/report/report.go);
covered by `TestRunReportReadsCarriedJobLogs` in
[conformance/05-web/report_test.go](../conformance/05-web/report_test.go) and
`TestRunReportReadsCarriedJobLogsFromTheirOrigin` in
[internal/report/record_test.go](../internal/report/record_test.go).

## Launching commands on artifacts

`rotari web --allow-exec` runs the commands a user registered in
`launchers.toml` against job artifacts. The user guide is
[Launching commands](../docs/WEB.md#launching-commands),
the file format [Launcher commands](../docs/CONFIGURATION.md#launcher-commands),
and the design [the artifact launcher plan](../development/2026-10-20-artifact-launcher/plan.md).
The manager is [internal/launcher](../internal/launcher/), the Web side
[internal/webui/launches.go](../internal/webui/launches.go),
[launch_guard.go](../internal/webui/launch_guard.go), and
[launch_proxy.go](../internal/webui/launch_proxy.go), and the CLI side
[cmd/rotari/web_launch.go](../cmd/rotari/web_launch.go).

**WEB-13** Launching is off unless `rotari web` gets `--allow-exec` on the
command line; no config file or environment variable enables it. It is
rejected with `--allow-control=false`, `--allowed-host` is rejected without
it or `--allow-terminal`, and both are rejected with `--static-dir` (WEB-3). With it, every route
requires the Web token, on loopback listeners too: without
`--auth-token` or `ROTARI_WEB_AUTH_TOKEN`, the server generates one and
prints it on stderr; it always prints the sign-in user name and never prints a
configured token. Browser users sign in through the login form over HTTPS, or
over plain HTTP on a direct loopback connection (see the authentication
bullet above), and receive a revocable 12-hour session cookie, `Secure` except on direct loopback; other
plain-HTTP login and session use are refused. API callers continue to use bearer or `X-Rotari-Token` credentials.
Session-cookie mutations require a same-origin request, and signing out
revokes the session and expires both Web and launch cookies, with `Secure` set as
it was when they were issued. Without it,
`GET /api/launchers` reports launching unavailable with the server's reason
and enabling hint, and the launch API answers 403 with the same text. Covered
by `TestWebLaunchRequiresOptInAndToken` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go),
and the login-cookie requirements by
`TestWebLoginFormCreatesAndRevokesSession`,
`TestWebSessionCookieIsSecureExceptOnDirectLoopback`, `TestWebLogoutCookiesMatchTheTransport`,
`TestWebLoginOverHTTPOnlyOnDirectLoopback`, and
`TestWebLoopbackSessionRejectedFromRemoteClient` in
[webui_test.go](../internal/webui/webui_test.go), implemented in
[auth.go](../internal/webui/auth.go).
Logout cookie attributes across HTTP, TLS, and TLS proxies are checked by
`TestWebLogoutCookiesMatchTheTransport` in
[launch_auth_test.go](../internal/webui/launch_auth_test.go).

**WEB-14** While launching is enabled, a request whose `Host` is not
`localhost`, `127.0.0.1`, `[::1]` (any port), a specific `--host`, or an
`--allowed-host` name gets 421, and a request other than `GET` or `HEAD`, or a
WebSocket handshake, gets 403 unless its `Origin` equals its own scheme and host, or it has no `Origin`
and authenticates with `X-Rotari-Token` or `Authorization: Bearer`. Basic
authentication alone, which a browser resends by itself, does not exempt a
request without `Origin`. Covered by `TestWebLaunchGuardsRequests` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go).

**WEB-15** Only the built-in commands (TensorBoard, Jupyter Lab, MLflow, MLflow
(SQLite), Aim, Streamlit, Datasette, marimo, Panel, Gradio, and VS Code) and the commands in the global config
directory's `launchers.toml` (`$XDG_CONFIG_HOME/rotari/launchers.toml`) can
run; an entry there with a built-in's name replaces it, and a copy of the file in a basedir, project,
or workspace is never read. `GET /api/launchers` marks a command
`unavailable`, with the reason, when the first program it runs is not on the
Web server's `PATH` (or an absolute path to it is not executable), and such a
command neither previews nor starts. `GET /api/launchers?kind=K&name=N` lists
only the commands that accept a target of kind K named N, including a
command's file name `patterns` and its `file_patterns`, which limit only files,
and a start refuses a target its patterns do not match. Each repeated `target=K:N` (at most 1000) gets, in order, an
entry of `targets` naming the commands and viewers that accept it, the
commands only while launching is available; the Web UI puts a button per
command in each artifact row, in lines of three (two on a narrow screen), and
preview title line, disabled with the
reason when its program is missing, and says once above them why nothing
can be launched when launching is unavailable
(`fillArtifactLaunchButtons` in
[web_app_launches.js](../internal/webui/assets/web_app_launches.js)). `rotari web --allow-exec` refuses to
start when that file, a directory above it, or a symlink on the way is owned
by another user or writable by group or others, or when the file is invalid,
and likewise, with sticky directories like `/tmp` allowed, when `$TMPDIR` or a
directory above it could let another user replace the private runtime
directory, whose generated Jupyter config runs as the launching user;
`rotari config --launchers` writes a template there with mode `0600` and
`config --list` lists it. `POST /api/launch` accepts only a registered
command's name, an artifact named as WEB-6 names one (entry index, `wd`,
or `basedir_parent`, and optional child under an allowed root; the run
page's toolbar has one Files button, which browses `basedir_parent`,
`runFilesButton` in
[web_app_launches.js](../internal/webui/assets/web_app_launches.js)), and
whether to start another
instance; any other field is rejected and nothing starts, as are an unknown
name, a command that does not accept the artifact's kind, and an artifact
that does not exist on the Web host. The command runs with `/bin/sh -c`,
empty stdin, `ROTARI_LAUNCHER_ARTIFACT` and `ROTARI_LAUNCHER_CWD` set, and
without `ROTARI_WEB_AUTH_TOKEN`. Covered by
`TestWebLaunchRunsOnlyRegisteredCommands` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go),
`targets` by `TestLaunchAvailabilityForATarget` and `basedir_parent` by
`TestLaunchOnBasedirParent` in
[launches_test.go](../internal/webui/launches_test.go), the row buttons by
`TestWebLaunchInBrowser` and `TestArtifactRowLaunchButtonsTakeColumns` and the Files view by `TestWebRunFilesInBrowser` in
[launch_browser_test.go](../internal/webui/launch_browser_test.go), and its
toolbar button by `TestWebRunJobsTable` in
[run_jobs_test.go](../internal/webui/run_jobs_test.go).

**WEB-16** A start while the same command runs on the same target and
working directory returns that launch, marked reused, unless another instance
is requested. `GET /api/launches` lists launches with their state and exit
status, `GET /api/launch-log` pages their output, and `POST /api/launch-stop`
ends a launch's whole process group. Stopping or killing `rotari web`, even
with SIGKILL, ends every launch's process group. Covered by
`TestWebLaunchLifecycle` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go).

**WEB-27** On live application pages where launching is available, the top-bar
**Launches** button shows the current number of launches in `starting`,
`running`, or `stopping` state, refreshed every two seconds. Completed launches
are not counted. Covered by the browser check in `TestWebLaunchLifecycle` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go).

**WEB-17** A service launch (`mode = "service"`) is `starting` until the
process listening on `127.0.0.1:$ROTARI_LAUNCHER_PORT` belongs to the launch
and its readiness path under `$ROTARI_LAUNCHER_BASE_PATH` answers 2xx or 3xx;
a process outside the launch on that port fails it. Each service launch gets
its own `$ROTARI_LAUNCHER_TOKEN`, which never appears in an API response,
`$ROTARI_LAUNCHER_TOKEN_FILE`, a file only the server's user can read that holds
the token and is removed when the launch ends, and
`$ROTARI_LAUNCHER_JUPYTER_CONFIG`, a Jupyter config file only the server's user
can read, holding no secret, that sets that token as the login token after
reading the user's own config, so the built-in Jupyter Lab's token is in no
process's arguments and a `token = ''` of the user's cannot turn it off.
Only after readiness does `/_launch/ID/...` forward requests to it, with path
and query unchanged. Open links in both Artifacts and Launches keep this
server-wide path, including after refresh on a `/_basedir/ID/` page.
Requests are forwarded without the browser's `Authorization` and
`X-Rotari-Token` headers or Rotari launch-session cookie, and with the
command's `proxy_headers` (the readiness probe sends them too), in which
`$ROTARI_LAUNCHER_TOKEN` is the
launch's token and `$ROTARI_LAUNCHER_BASIC_AUTH` the Base64 of `rotari:` and
that token, so a service can refuse anyone who reaches its port
without the proxy. A `Cookie` in `proxy_headers` precedes the browser's other
cookies rather than replacing them, so it wins over one of the same name for a
service that reads the first. Before then and
after it ends the route answers 503, and 404 for an unknown ID.
Forwarded requests carry `X-Forwarded-Host` (the request's checked `Host`, never
a browser-sent `X-Forwarded-Host`) and `X-Forwarded-Proto` (`https` over TLS or
from a TLS-terminating proxy, otherwise `http`), so a service that builds
absolute URLs names the address the browser used, not its loopback port.
An authenticated request under `/_launch/` establishes an HttpOnly,
SameSite=Strict cookie scoped to that prefix (Secure over TLS), valid for 12
hours and invalidated by a Web server restart. It authenticates only launch
routes, including requests carrying the application's own Authorization
header; all Host and Origin checks still apply. Other application cookies
are forwarded unchanged. Implemented by
[launch_auth.go](../internal/webui/launch_auth.go) and
[launch_proxy.go](../internal/webui/launch_proxy.go), with session lifetime
and guard checks in
[launch_auth_test.go](../internal/webui/launch_auth_test.go).
Covered by
`TestWebLaunchServesServices` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go).

**WEB-18** The live server shows the global launcher file whether or not
launching is enabled: `GET /api/launcher-config` returns its path, whether it
exists, its text, whether it is editable on this server, why
`rotari web --allow-exec` would refuse it, and the commands a restarted server
would load (built-ins included, each with `builtin` and `unavailable`).
`POST /api/generate-launcher-config` writes the
same template as `rotari config --launchers` there, mode `0600`, answers 409
without changing an existing file. `POST /api/save-launcher-config` validates
and atomically saves edits only to an existing trusted file when its contents
still match the editor's original version; stale edits answer 409, invalid
TOML answers 400, and a read-only server answers 403. The Settings menu's
Launcher config editor and Generate launcher config use these routes
([internal/webui/launcher_config.go](../internal/webui/launcher_config.go)).
Covered by `TestWebShowsAndGeneratesLauncherConfig` in
[conformance/05-web/launch_test.go](../conformance/05-web/launch_test.go).

**WEB-19** The live server's Table viewer reads an artifact named as WEB-6
names one, under the same roots and with the same refusals (403 outside
them or through an escaping symlink, 404 when missing), at
`/tableview/?<the content request's query>`. Each artifact row and preview
title line offers it, as a link that opens in a new tab, for every directory and for files whose names
match `*.csv`, `*.tsv`, `*.tab`, `*.psv`, `*.txt`, `*.scp`, `*.lst`,
`*.list`, `*.dat`, or `*.jsonl` (with the corresponding uppercase variants),
and Markdown, PDF, and source/config files for Python, shell, JavaScript/TypeScript,
Go, Rust, Java, C/C++, C#, Ruby, PHP, Swift, Kotlin, Lua, Perl, R, SQL,
JSON/YAML/XML/HTML/CSS/INI/TOML, diff, Dockerfile, and Makefile. Their exact
case-sensitive file patterns are `documentPatterns` in
[internal/tableview/delimited.go](../internal/tableview/delimited.go). The viewer
runs no command, so it needs no
`--allow-exec` and is listed in `/api/launchers` as `viewers` whether or not
launching is available. `/tableview/api/rows` splits the file into records
on line breaks and cells on `sep`, which must be exactly one character other
than a double quote or a line break, honoring double quotes as CSV does and
skipping blank lines; `header=0` keeps the first record as a row (default
`1`, its column names). It returns `limit` rows (default 100, at most 1000)
from `offset`, numbered from 1, with the total, cells cut to 4 KiB, and an
error for a record over 16 MiB; the file is indexed once per version and
separator, so a page does not reread it from the start. `filters`, a JSON
list of `{column, op, value}` with columns counted from 0, keeps the rows
every condition matches: `=` and `!=` compare numbers when the cell and the
value both read as finite numbers and text otherwise, `<`, `<=`, `>`, `>=`
need a number and match only cells that read as one, `contains` and
`not_contains` compare text case-sensitively, `regex` and `not_regex` search a
cell's stored text (not its formatted display) with Go's `regexp` syntax,
case-sensitively unless the pattern says `(?i)`, and `empty` and `not_empty`
test for a blank cell (a short row's missing cells are blank); a column out
of range, an unknown operator, an ordering against a non-number, or an invalid
pattern is refused. In the page, a refused new condition comes off again with the
rows as they were, the error shown, and the condition back in the form; a refused
condition from the URL stays as a chip that can be removed. `sort` (a column) and `desc=1` order the selected rows, numbers by
value before other cells by text, keeping the file's order among equal
cells, for up to 1,000,000 rows. Rows keep their numbers in the file.
`/tableview/api/describe` summarizes each column over the rows the filters
select, as pandas' `describe` does: non-blank cells, distinct values (up to
100,000, then `unique_over`), and over the cells that read as numbers their
count, mean, sample standard deviation, minimum, maximum, and sum. It also gives the
quartiles of those numbers by linear interpolation (`q25`, `q50`, `q75`) and a
`histogram` of 200 equal-width bins from minimum to maximum (one bin when all are
equal, none when the range overflows), unless the selected rows hold more than
10,000,000 numbers, which sets `distribution_over` and leaves them out of every
column. `top` lists up to 5 most frequent non-blank values with their counts, the
most frequent and then first seen first, each cut to 256 bytes (`cut`); it is absent
when every value appears once, and past 10,000 distinct values `top_over` says so
instead. The figures are exact or absent, from one read of the file. The page's
Describe rows show a **Histogram** button for a column that has one, which opens the
bars in a tab of the active pane, one tab per column and selection of rows, with 10,
20, 50, 100, or 200 bars merged from those bins; a row move does not replace it.
A `.jsonl` file instead has one JSON value per nonblank line. Object keys
across all records become columns in sorted name order; arrays and scalar
values use a `value` column. Strings and numbers become text, nested arrays
and objects become compact JSON, and null or missing values are blank.
Malformed records and records over 16 MiB are errors. It uses the same
filtering, sorting, paging, and Describe behavior as delimited tables.
A cell whose trimmed text ends in an image, audio, video, or PDF extension (as
WEB-6 lists them) names a file: absolute as it is, or relative to the base
`base` names, `basedir` (the default), `wd` (the job's recorded working
directory), or `file` (the table's own directory). Each such cell of a page
comes with the file's properties read from its headers without decoding it
(format; duration, sample rate, channels, and bit depth for WAV, FLAC,
MP3, Ogg Vorbis and Opus, M4A, MP4, MOV, and WebM; width and height for
images and video; size; and a bitrate, the format's own for PCM WAV and
constant-bitrate MP3 and otherwise size over duration), or with why it is
not shown: outside the roots, through an escaping symlink, or missing.
`/tableview/api/media?ref=&base=` serves that file under the same roots and
refusals, and nothing that is not an image, audio, video, or PDF file.
A directory is read as a table instead, through the same roots without
following directory symlinks and reading at most 100,000 entries. Initially
only its immediate children are read. Repeated `expanded` parameters name
relative slash-separated directory paths whose immediate children are also
read; ancestors must also be expanded. Absolute paths, `.` and `..`
components, backslashes, empty paths, non-directories, and unavailable
directories are refused. Collapsed directories are not read, even when
`pattern` contains `**`. A directory read error is surfaced, not hidden.
The files `pattern` matches by their slash-separated path inside it (`*` within
a name, `**` any number of directories, `{a,b}` alternatives; default
`**/*`) are included, and directory rows remain available regardless of the
pattern. Listings are keyed by the expanded scope and listed again after
10 seconds. `view=files`
(the default) has one row per file or directory, in depth-first sibling
name order unless explicitly sorted, with its path, format, duration,
sample rate, channels, bitrate, width, height, size, and modification time
in typed columns: a filter on a duration takes seconds or `m:ss`, on a size
a binary suffix such as `10M`, and on a time compares RFC 3339 text.
Directory rows carry `directory` (the full relative path), a path cell ending
in `/`, and modification time, but no media properties or file size.
`/tableview/api/text?ref=&base=` reads any file a path names, resolved and
refused as for media but of any type, as text pages the way
`/api/artifact-text` does (`from`, `offset`, 64 KiB on line boundaries, a
NUL byte refusing it); a directory's page opens a path cell that names no
media or directory that way, in a pane.
`/tableview/api/download?ref=&base=` saves any regular file a path names, resolved
and refused the same way, as an attachment under its own name. Each pane offers it
for the file showing, or the copy in a static snapshot.
In either view a row's `tables` lists its path cells naming a delimited or
JSON Lines table file (by the table-file base-name patterns), which the
page links to a viewer of its own in a new tab: the same query with
`child` naming the file inside the directory, as for any artifact.
Filters, sorting, and Describe cover only this opened scope.
`view=stems` uses only files in this same scope, with one row per stem and one column per group, `group` naming
how a path splits: `top` (the first directory; the stem is the rest
without its extension), `parent` (the file's directory and its name without
the extension), `extension`, or `suffix` (the name's end after the last
`suffix_sep`); without `group`, `extension` when no matched file is in a
subdirectory and `top` otherwise. A group without the stem's file has a
blank cell; `incomplete=1` keeps the stems with one; only the first file of
a stem in a group is listed, and the page's `notes` say how many were left
out, or that the listing was cut short. A directory's media paths are
relative to it. A file's parameters are refused for a directory and a
directory's for a file, rather than ignored.
`/tableview/api/info` names the file or directory, a file's size, download,
and the separator its extension suggests, and the bases; a JSON Lines file
names its format, and text documents name their preview. The page renders
Markdown with a source toggle and safely highlights supported source/config
languages from the local Highlight.js bundle in text panes; rendering and
highlighting are limited to the first 2 MiB, after which
the source is shown. PDFs use the browser's inline PDF viewer and also link to a new
tab. Opening a Markdown, script, or PDF file directly shows its content, and
table row APIs refuse to treat it as a delimited table. The
page opens a cell's media in tabs of the active pane of panes
above the table, which keeps the height of the page below its header, so
open panes scroll the page rather than shrink the table; panes
split right or down, one pane always active; tabs move between panes by
drag and drop, a drop near an edge splitting that pane; audio shows a
waveform and a spectrogram the browser computes from the decoded samples, each
with a grid at round times that the time axis labels, and a vertical scale of
amplitude or frequency; a bar between them sets the waveform's height for the pane,
and the waveform alone fills it;
of the channels' mix or, for a file of more than one channel, any one
channel chosen per file, which the player then plays alone from every
speaker (Mix plays the file as it is); a plot whose size changes is drawn
stretched from its last drawing until the size holds, then computed again,
and linked panes share one time range and playback time. The page keeps the separator, header, media
base, directory listing and expanded paths, filters, sort, Describe, page, and page size in its
URL fragment, and drops
the filters and sort when the separator or the listing changes. Clicking
a directory in Files expands it, loading only its immediate children;
clicking again collapses it and removes its descendants from the expanded
scope. Expansion preserves the filters, sort, and current page (clamping
the page if collapse reduces the page count); a failed expansion is shown
as an error and the previous scope is restored. Group by stem shares this scope and never expands directories
implicitly. A directory row's arrow shows that directory in the same window
as the source's `child`, a new history entry, so Back returns; the page's
name is the path from the source, each directory above linking to its own
page, after an up link (also Alt+↑) to the parent, which is disabled at the
source itself: the viewer never moves above the directory it was opened on.
These links open in a new tab with a modifier, as links do. Moving keeps
Files or Group by stem and starts the rest of the view afresh; open panes
stay, each file still read from where it was opened and titled by its path
from the source. A directory that cannot be shown leaves its error and no
rows of the page before. Implemented by
[internal/tableview](../internal/tableview/), including
[jsonl.go](../internal/tableview/jsonl.go), and
[internal/webui/table_view.go](../internal/webui/table_view.go), with the page in
[table_view.js](../internal/webui/assets/table_view.js) and
[table_view_panes.js](../internal/webui/assets/table_view_panes.js),
[web_app_highlight.js](../internal/webui/assets/web_app_highlight.js),
[web_app_previews.js](../internal/webui/assets/web_app_previews.js), and
[web_app_markdown.js](../internal/webui/assets/web_app_markdown.js); covered by
`TestWebTableViewerReadsArtifacts` in
[conformance/05-web/table_view_test.go](../conformance/05-web/table_view_test.go), with
the rules in [internal/tableview/filter_test.go](../internal/tableview/filter_test.go),
[internal/tableview/directory_test.go](../internal/tableview/directory_test.go),
and [internal/tableview/media_test.go](../internal/tableview/media_test.go) (whose
sample files ffmpeg made, with ffprobe's values),
the page checked by `TestTableViewPagesInBrowser` and
`TestTableViewFiltersInBrowser`, `TestTableViewShowsMediaInBrowser`,
`TestTableViewExpandsDirectoriesInBrowser`, `TestTableViewGroupsStemsInBrowser`, `TestTableViewOpensFilesInPanesInBrowser`,
`TestTableViewMovesTabsAndLinksPanesInBrowser`,
`TestTableViewOpensTableFilesInBrowser`, `TestTableViewMovesBetweenDirectoriesInBrowser`,
`TestTableViewShowsTextFilesInBrowser`,
`TestTableViewChoosesAChannelInBrowser`, `TestChannelSamplesMixesChannels`,
`TestTableViewPlotsComputeOnceInBrowser`, and `TestSpectrogramFindsATone`, and the row's link by
`TestLaunchWithOpensTheTableViewerInBrowser` in
[internal/webui/table_view_test.go](../internal/webui/table_view_test.go).

**WEB-20** `rotari tableview PATH` serves the Table viewer (WEB-19) for one
delimited or JSON Lines table, Markdown or supported source/config text, PDF
document, or directory outside
any project: the directory, or the
file's, is the only root anything is read under and the only base its
relative media paths resolve from, through `os.Root`, with the same
refusals. For a directory, `child` names a file or directory inside it to
view instead, under the same root, so a table or document file it lists opens as
WEB-19 describes; `child` is refused for a file, and when it is not a
clean relative path, leaves the root through a symlink, or is missing. It listens on `--host` (default `127.0.0.1`) and `--port`
(default 8788, or the next free port when `--port` is not given), prints
`rotari tableview listening at http://ADDR`, serves the page's fonts and
design tokens itself, and, with `--auth-token` or `ROTARI_WEB_AUTH_TOKEN`,
requires the token as `rotari web` does. A missing path or no path fails
before listening. Implemented by
[cmd/rotari/tableview.go](../cmd/rotari/tableview.go) and `TableViewHandler`
in [internal/webui/table_view.go](../internal/webui/table_view.go); covered by
`TestTableViewCommandServesOnePath` in
[conformance/05-web/tableview_command_test.go](../conformance/05-web/tableview_command_test.go).

**WEB-30** The Table viewer has a current row, marked with `aria-current`.
`↓` and `↑` make the next or previous row current in the displayed order, under
the current filters and sort and across pages, stopping at either end; from a row
not on the page, they take the page's first row. A click on a row's own text
moves the same way. A click on a cell's file, or **Open row**, makes its row
current and opens the file. Focus does not stay
on the clicked control, so the keys below keep working.

One cell of the current row is framed: the one clicked, or at first the row's
first file (or first column). The frame keeps its column as the row moves. `←` and
`→` move it along the row, stopping at either end, and open nothing. `Enter` opens
the framed file, and `Shift+Enter` every file the row names, side by side. `Space`
plays or pauses the framed file, in its tab or, when it is not open, in a new tab
of the active pane; playing one pauses the others. On a cell without an audio or
video file, it plays or pauses the active pane's file. A row change never starts
playback.

Opening Describe scrolls the page to the table and the table to its head. Opening
a file with a click, **Open row**, `Enter`, or `Shift+Enter`, or a histogram, scrolls
the page to the panes; a file following a row move or opened by `Space` does not.

About 150 ms after moves stop, the active pane's current tab, if it was opened from
a row of this table, shows its column of the new row, keeping the layout and
resetting the file's time range, playback position, and channel. Other panes and
hidden tabs stay as they are. Moves scroll only the table to keep the row in view,
and the page only as far as the table cannot: when the part of the table in view has no
room for a row, or for a row near the table's end, which the page cuts off however far
the table scrolls. Only the last row is loaded after a held key. If the new row has no file there (a blank cell, a directory row, or a
file with an error), the tab says so and can go on following; it never keeps the
previous row's file.

The row number is kept in the URL fragment as `row`, and reloading marks it
without opening files. A value that is not a positive integer or not on the page
marks nothing. Filters, the sort, and paging keep it. A different separator, header
row, base, listing, expansion, or directory clears it, and tabs opened before that
no longer follow, because row numbers then mean something else.

Keys are left alone with a modifier held and when focus is in an input, select,
textarea, editable element, or audio or video element; `Enter` and `Space` also on
a button or link. `Space` scrolls the page as usual when there is nothing to play. The rules are the same in a live viewer and a static snapshot, and
there are no row keys for a document preview. Implemented in
[internal/webui/assets/table_view.js](../internal/webui/assets/table_view.js) and
[table_view_panes.js](../internal/webui/assets/table_view_panes.js); covered by
`TestTableViewerMovesBetweenRows` and, in Chrome, `TestTableViewerRowKeysInBrowser`
in [conformance/05-web/table_view_rows_test.go](../conformance/05-web/table_view_rows_test.go).

**WEB-32** A Table viewer tab dragged past the edge of its window, or a pane's **Open
in a new window** button, moves the file to a window of its own showing only panes,
at the current playback position; a blocked window leaves the tab and says so. There,
a pane's **Back to the table** button returns the file to the window with the table,
and says so if that window does not answer. A tab dragged between windows moves to the
pane it lands on, by the same zones as within a window, or to the active pane when
dropped elsewhere. Row moves change the file in the window last clicked in or moved
to, under WEB-30's rule for the active pane, and arrow keys in a window of its own move
the table. `Space` there plays its file, and the table's `Space` plays the file in that
window while it has the row moves. Playing with `Space` pauses other windows. Closing
the window closes its files, and row moves return to the table's window. These need a
`BroadcastChannel`; without one a window still opens, but does not follow or return a
file. Implemented in
[internal/webui/assets/table_view_windows.js](../internal/webui/assets/table_view_windows.js);
covered by `TestTableViewerMovesTabsToWindows` in
[conformance/05-web/table_view_rows_test.go](../conformance/05-web/table_view_rows_test.go),
which cannot use real windows or drags.

**WEB-31** `/tableview/api/export`, with the parameters of `/tableview/api/rows`
except `offset`, `limit`, and `base`, which are refused, downloads every row the
filters select, in the sort's order, under the same roots and refusals and limits
as `rows`, with whole cells. A delimited file gives records in its separator, with
its own first record as the header when `header=1`; quoting and line endings may
differ, but reading the output with the same settings gives the same cells. A JSON
Lines file gives each selected record's original line. A directory listing gives
CSV of the listing's columns and rows, covering expanded directories only. No rows
give the header alone. The file is named `<stem>.export<ext>`, or
`<directory>.export.csv`. Anything refused is refused before the first byte; a read
failing later aborts the connection, so a short file cannot pass for a whole one.
The page offers it as **Export rows** once the selected rows have loaded and at
least one matched. A static snapshot has no export. Implemented in
[internal/tableview/export.go](../internal/tableview/export.go) and
[internal/webui/table_view.go](../internal/webui/table_view.go); covered by
`TestTableViewerExportsRows` in
[conformance/05-web/table_view_export_test.go](../conformance/05-web/table_view_export_test.go).

**WEB-22** `rotari tableview --static-dir DIR FILE` writes a static snapshot
of the Table viewer for one delimited or JSON Lines table instead of serving,
reading under the file's directory as WEB-20 does. A delimited file is read
with the separator its extension suggests and a header row; a JSON Lines file
uses its record keys as columns. The snapshot embeds every row (at most
10,000, more fail), each column's ascending and descending order, and
Describe over the whole file, and copies each image, audio, video, or PDF file its cells name
that the live viewer would show (at most 10 MiB each and 100 MiB in all, or
the export fails) into `media/`, and prints how many rows and media files,
and how many bytes of them, it wrote. The page is named by the file's base name,
not its path. `table_view_static.js` answers the page's `api/info`,
`api/rows` (paging and sorting), and `api/describe` from the snapshot and
links media cells to their copies; the page disables the separator and header
controls for delimited files, and shows the reason in place of the filter
form. Filters, and delimited separator/header settings, cannot be changed;
a request with filters, another delimited separator or header, or another base
answers 400 with the reason. `--host`, `--port`, and `--auth-token` are
refused with `--static-dir`; an `auth-token` from `ROTARI_WEB_AUTH_TOKEN` or
config is ignored with a warning, as for `rotari web` (WEB-3). Also refused are a
Markdown/script/PDF document, a directory, and an output directory that contains the file, since the export
replaces it. Implemented by `GenerateStaticTableView` in
[internal/webui/table_view_static.go](../internal/webui/table_view_static.go)
and `table_view_static.js`; covered by
`TestTableViewCommandWritesAStaticSnapshot` in
[conformance/05-web/tableview_command_test.go](../conformance/05-web/tableview_command_test.go),
with the page checked by `TestStaticTableViewInBrowser` and the refusals by
`TestStaticTableViewRefuses` in
[internal/webui/table_view_static_test.go](../internal/webui/table_view_static_test.go).

**WEB-23** The live Recent jobs page (`/jobs/`) offers window presets
(`1h`, `6h`, and the default `24h`) and a range bar showing 1, 7 (the
default), or 30 days, with this basedir's runs from `/api/activity` behind
it; `‹` and `›` page it by its width, `›` disabled when it ends now.
Dragging on the bar selects one range, snapped to 15 minutes, an hour, or 6
hours by width; a click selects the local day (the hour on the 1-day bar)
under it; dragging the selection's ends resizes it and its middle moves it.
The page reloads with `from` and `to` (RFC 3339) or `window`, and the bar's
`view` and `end` when they are not the defaults; a range opened without them
is shown. The page then lists unfinished jobs and jobs that finished from
`from` up to `to`, or inside the window before now, and names which under
its title. With neither, it lists the 24 hours before now unless this
basedir's latest run, by the end of every run meeting the time since the
latest start, ended before them; then it opens on the 24 hours up to that end,
rounded up to the minute, so a quiet basedir shows its last work. The static
`jobs/index.html` lists the same default without the bar. `since`, which the page took before, answers 400 rather than
being ignored, as do another window or view, a window with a range, `from`
without `to` or the reverse, a malformed time, `to` not after `from`, `from`
not before now, a range over 30 days, a future `end`, and a repeated
parameter. A finished run that started after the range ended is not read for
the jobs it ran itself, which finished later; jobs carried from an earlier
run keep that run's times and are still listed by them. Implemented by
`parseJobsSelection` in
[internal/webui/jobs_range.go](../internal/webui/jobs_range.go) and
`joblist.CollectFinishedIn` in
[internal/joblist/joblist.go](../internal/joblist/joblist.go), with the bar
drawn by `renderJobsRange` in `jobs_template.html`; covered by
`TestRecentJobsSelectsFinishedJobsByRange` in
[conformance/05-web/recent_jobs_test.go](../conformance/05-web/recent_jobs_test.go),
which cannot age a run and so leaves the latest-run default to
`TestResolveJobsDefaultFollowsLatestRun` and
`TestWebJobsPageDefaultsToLatestRun`, with the bar checked by `TestJobsRangeBarInBrowser` in
[internal/webui/jobs_page_test.go](../internal/webui/jobs_page_test.go) and
carried jobs by `TestCollectFinishedInKeepsCarriedJobsOfLaterRuns` in
[internal/joblist/joblist_test.go](../internal/joblist/joblist_test.go).

**WEB-24** A job that has started shows its latest attempt's ID from
dispatch on, while it runs and before the run has a summary: in `/api/run`
and the run page as on the Recent jobs page, in `rotari jobs`, and in the run
view of `show --wide`. A job without an attempt in the run, as a carried job,
shows the attempt its recorded result names; a job not started yet shows none,
even when a retry will execute it again and it has an earlier attempt. All of
them take the ID from `jobstatus.AttemptID` in
[internal/jobstatus/job.go](../internal/jobstatus/job.go); covered by
`TestRunPageShowsRunningAttemptID` and
`TestRetriedJobsShowTheSameAttemptEverywhere` in
[conformance/05-web/running_attempt_test.go](../conformance/05-web/running_attempt_test.go)
and `TestLoadJobsShowsRunningAttemptID` in
[internal/web/loader_test.go](../internal/web/loader_test.go).

## Editing rules

- Edit HTML, CSS, and JavaScript in `internal/webui/assets/`, not in `webui.go`.
- Keep JavaScript additions in the responsibility file that owns the behavior.
- Prefer stable semantic classes and data attributes for UI behavior and tests;
  do not identify controls by their visible labels when a class or attribute can
  express the meaning.
- Tests should inspect final generated HTML for required hooks and obsolete
  vocabulary. Avoid assertions that depend on formatter whitespace or quote
  style.
- Scripts marked `// @ts-check` are checked against their JSDoc types by
  `npm run typecheck` ([internal/webui/tsconfig.json](../internal/webui/tsconfig.json),
  shared types in `assets/web_globals.d.ts`); CI and `scripts/check.sh` run it.
  Mark a script when rewriting it, and keep it passing.
- Run pre-commit on changed files, `go test ./...`, and `go build ./...` after
  Web asset changes.

## Generated Web pages

**WEB-34** Without `--allow-terminal` the terminal's routes answer 404 and pages do not offer
it, and the flag is rejected with `--allow-control=false` or `--static-dir`. With it, as with
`--allow-exec`, every route requires the Web token, generated and printed when none is set, and
`Host` and `Origin` are checked (WEB-14). The WebSocket at `/_terminal/ws` also requires HTTPS
or a direct loopback connection, even with a token, and so do `/_terminal/shells`, which lists the
shells on `GET` and starts one on `POST` with `rows` and `cols` (409 at the limit), and, for the Explorer
(WEB-42), `dir`, an absolute directory to start in instead of where the page's place would, refused with the
reason (400) when it is not an existing directory or comes with `run`, never started elsewhere, and
`/_terminal/shells/ID`, which ends one on `DELETE` (404 if it is not running) and names it on `PATCH` with
`{"name": ...}`, up to 64 characters on one line, or empty for none. `GET /_terminal/shells/ID/preview`
says where a shell is now and what it runs without connecting to it, so it takes the shell from no page and
leaves its output unseen: `"cwd"`, the shell's own working directory, or `"cwd_error"` saying why it could not be
found, and `"command"`, the command line of the program the shell runs in the foreground, absent while the shell
is at its prompt, or `"command_error"` saying why it could not be found, and `"processes"`, the processes below the shell with their
`pid`, `ppid`, `depth` (1 for its children), and `command`, each child followed by its own, or `null` with
`"processes_error"` saying why they could not be listed (404 if the shell is not running, 405 for other methods).
`GET /_terminal/projects` lists each project with shells once, with `basedir` and `project`: `runs`, how many runs it
has, and `last_run`, the run `rotari show` reads when given none (the last one started), with `run_id`, `run_name`, `lifecycle` as the run page labels
it, `tone`, its status colour, `started_at` and `finished_at` when its files say, and `jobs`, its jobs counted by the
run page's Jobs card groups (`run` in progress, `pending`, `bad` failed, `ok`, `warn` blocked, `off` other); or
`status_error` saying why its runs could not be read (405 for other methods). Shells of no project, or of a basedir's own, are not listed.
`GET /_terminal/window`, followed by
the path of a page such as `/project/p`, is the panel's page of its own (405 for other methods). `PUT /_terminal/order` with `{"ids":
[...]}` arranges the project of the first running shell named, with others of it following and shells that have
ended or are of another project ignored; the list then gives each project's shells in that order, with a new one
last. `POST` and
`/_terminal/here`, which says where a new shell would go, take a page's place: `project`, with
`basedir` for a mounted one (404 if unknown), and `run`, read on the server and never a path from the
page; `basedir` with no project, empty for the server's own, is the place of that basedir's own pages, and
`run` with no project is refused (400). A shell runs as `$SHELL -l` or `/bin/sh` with the server's environment less the Web token, bash and zsh by default as WEB-49 says. With
a project, it starts where `run`, or the project's latest run, started. If that cannot be found or
does not exist on this host it starts in the home directory and says why in its first line, escaped,
which replays with its output. A basedir's own shell, with `basedir` and no project, starts in the basedir's
parent when the basedir is named `.rotari-state` or the parent's `.rotari.toml` names it as `basedir`, a relative
value read from the parent, and otherwise in the home directory, saying nothing; when that parent does not exist
on this host or its `.rotari.toml` cannot be read, it starts in the home directory and says why, as above. With
neither it starts in the home directory, and belongs to no project. A project's shell has `ROTARI_BASEDIR` and
`ROTARI_PROJECT_NAME` set to its basedir and project, and a basedir's own shell `ROTARI_BASEDIR` set and
`ROTARI_PROJECT_NAME` removed, whatever the server's environment says. Up to eight shells run per project of a
basedir, as many for each basedir's own, and thirty-two in all, or what
`--terminal-max-shells-per-project` and `--terminal-max-shells` say. `--terminal-shell` replaces `$SHELL`, and
`--terminal-replay-size` the 256 KiB of output kept per shell. These flags and `--terminal-idle-timeout` are
rejected on the command line without `--allow-terminal`, and with a bad value from anywhere when the terminal is on,
before the server starts. `--allow-terminal`, `--allowed-host`, and these options may also come from the environment or
a config, as WEB-36 says. A shell's `unseen_output` says it has printed something since a connection last showed it, and starts false
until one has; its `command_finished` says a command it ran in its foreground ended, back at its prompt, while no
connection showed it, and a connection showing it clears it; its `running` says a command holds its foreground as
the list is made; its `started_at` is when it started and its `active_at` when it last printed or was typed into,
which the list's `now` dates by the server's clock. A shell's `id` is never reused while the server runs, its `number` is the lowest not in
use in its project when it started, and its `link` is the page that shows it: its project's page, its basedir's
All projects for a basedir's own shell, and `/activity` for one of no project. A connection names its shell with
`shell`, and never starts one: a shell not running closes it with 4005. A shell outlives its
connections and replays recent output to the next, except what came before a `clear` message. A new connection to it takes it over, closing
the last with code 4001; one with `takeover=0`, which a page reconnecting by itself sends, gets 4001
itself when another connection has that shell. Connections to different shells do not disturb each
other. A shell ends with its own exit or `DELETE` (4002 with how it ended). All of them end on
signing out and the server stopping (4004) or being killed; with no idle limit, the default, nothing else ends
them, however long no page is open (`TestTerminalShellsHaveNoIdleLimitByDefault`, `TestNoIdleLimitKeepsTheShells`).
`--terminal-idle-timeout` sets a limit, `0` meaning none and any other value at least two minutes: they then also end
after that long with no connection to any shell and no `POST` to `/_terminal/alive`, which every open page offering
the terminal sends each minute, whatever its panel shows and whether or not it is in view (`TestTerminalPanel`). The
limit is checked at the package level by `TestIdleLimitAppliesToTheShellsTogether` and
`TestKeepAliveHoldsTheIdleLimitOff`. A process that left its session or ignores the hangup,
including a run started in a shell, goes on. Covered by the tests in
[conformance/05-web/terminal_test.go](../conformance/05-web/terminal_test.go).

**WEB-51** With `rotari web --allow-terminal`, a right click in a pane opens the panel's menu by the pointer and keeps
the browser's from opening, unless Shift is held (the browser's opens) or the program asked for mouse tracking (it
gets the click). The menu offers **Cut**, on only for a selection wholly within the command line typed at the prompt
(from `B` to the line's end, counted as WEB-50 counts, a selection run past the text ending with it), which copies
the selection and, only if that succeeds, moves the cursor to its end as WEB-50 does and sends one `DEL` per character,
then clears the selection; **Copy**, of the terminal's selection, off without one; **Paste**, which reads the
clipboard and gives it to the terminal as Ctrl+V does (bracketed when the program asked, with no Enter of its own),
or, when the browser does not let the page read it, says to use Ctrl+V (Cmd+V on a Mac), and says when it holds no
text; **Select all**; and, on a command's prompt or output (WEB-48), that command's **Copy output** and **Copy
command**. Implemented in the panel in `web_shell.js`; covered by `TestTerminalPanel` and a step of
[`TestTerminalPanelInBrowser`](../conformance/05-web/terminal_panel_browser_test.go).

**WEB-50** With `rotari web --allow-terminal`, a click in the command line being typed at a shell's prompt moves the
cursor there. It applies to a press and release of the left button without Alt, Ctrl, Meta, or Shift, within 2 pixels
of each other, that leave no selection, with no link under the pointer and no mouse tracking asked for by the
program, in a pane whose last block (WEB-48) has `B` and neither `C` nor an end, outside the alternate screen, with
the view at the bottom, on a row from `B`'s to the cursor's or one wrapped on from it, and not before `B`. Its target
is counted in characters from `B`: the second cell of a wide character is the character's, a row ending without
wrapping is one character, and a click past a row's text goes to its end, or the cursor's place if further. The panel
then sends, as keystrokes, the line's character index after `ESC [ 9998 ~` and before `;` to a shell whose `A` said
`rotari_goto=1` (WEB-49), and otherwise the left or right arrow keys for the difference, as `ESC [` or, in
application cursor mode, `ESC O`; nothing when the cursor is already there, and nothing else in any case. Alt+click
stays xterm.js's. Implemented in the panel in `web_shell.js`; covered by `TestTerminalPanel` and a step of
[`TestTerminalPanelInBrowser`](../conformance/05-web/terminal_panel_browser_test.go).

**WEB-49** With `rotari web --allow-terminal` and `--terminal-shell-integration`, on by default, a shell whose program
is named `bash` or `zsh` starts with rotari's own startup files, written when the server starts into a new directory
only its user can read and removed when it stops; the user's files are not changed. bash runs with `--init-file`
naming rotari's script, which reads `/etc/profile` and the first readable of `~/.bash_profile`, `~/.bash_login`, and
`~/.profile`, as a login shell would. zsh runs with `-l` and `ZDOTDIR` naming rotari's directory, whose `.zshenv`,
`.zprofile`, `.zshrc`, and `.zlogin` each read the user's file of that name, at top level, from where zsh would have:
the `ZDOTDIR` the shell was to have, else `$HOME`, or a `ZDOTDIR` the user's `.zshenv` sets; afterwards `ZDOTDIR` is
as it was (unset if it was and nothing set it). Then each marks its prompts and commands with OSC 133 for the panel's
blocks (WEB-48): before every prompt, `D;STATUS` with the last command's exit status and `A`, after the user's own
prompt hooks, which see the command's status unchanged; `B` where the command line starts: in bash at the prompt's
end, put back when a hook rebuilt the prompt without it, and in zsh when the line editor starts (its `line-init`
hook), so that a theme rebuilding the prompt at any time keeps it, or at the prompt's end in a zsh before 5.3; and `C` before the first command run from a prompt (bash 4.4 or later by `PS0`, earlier bash by a
`DEBUG` trap unless the user's files set one, when there is no `C`; zsh by `preexec`). `C` says the command line as
kitty sends it, `C;cmdline_url=` and the line percent-encoded byte by byte: always in zsh, and in bash only when
history has just taken the line (its last entry changed since the prompt), so that a line history leaves out by
`HISTCONTROL` or `HISTIGNORE`, or with history off, gets a `C` that says none rather than the line before it. zsh also
binds `ESC [ 9998 ~` in its `emacs`, `viins`, and `vicmd` keymaps to a widget that reads up to ten digits and, on a
closing `;`, puts the cursor at that character index, capped at the buffer's end (so a suggestion shown past it is
not taken), and says so on every `A` as `A;rotari_goto=1` (WEB-50). Any other shell,
and every shell with `--terminal-shell-integration=false` (or `terminal-shell-integration = false` in a config, or
`ROTARI_WEB_TERMINAL_SHELL_INTEGRATION=false`, as WEB-36 reads them), starts as `-l` with nothing added. The option
is refused without `--allow-terminal` and with `--static-dir`, and a detached server reports it as a difference
when asked for otherwise (WEB-40). Implemented in `internal/terminal/integration.go` and the scripts beside it;
covered by [`TestTerminalShellIntegration`](../conformance/05-web/terminal_integration_test.go) and the unit tests
`TestBashIntegrationReadsTheLoginFilesAndMarks`, `TestOldBashIntegrationMarksCommandsByTheDebugTrap`,
`TestZshIntegrationReadsTheUsersFilesAndMarks`,
`TestShellCommandWithIntegration`, and `TestKeeperWritesAndRemovesTheIntegration`.

**WEB-48** With `rotari web --allow-terminal`, the panel reads the OSC 133 marks a shell or its prompt prints (bash
and zsh print them by default, WEB-49):
`A` (a prompt starts), `B` (its command line starts), `C` (the command's output starts), and `D` with an optional
exit code (the command ended). Each `A` starts a block; its command line is what `C` says after `cmdline_url=`, percent-decoded as
UTF-8 (as kitty sends it), or, without one or with one that does not decode, the screen's text from `B` to `C`,
wrapped lines joined, and its output the text from `C` to `D`, or to the next prompt, or to the cursor while it runs. A
prompt before `D` ends the last block without an exit code, as does `D` with none or with one that is not an
integer. Marks while the alternate screen is shown, and marks with no block to belong to, are ignored. A block is
dropped with its prompt's line, when it leaves the scrollback or **Clear** empties it, and every block when the pane
shows its shell again. Only the page keeps blocks: the server parses nothing, and the replay (WEB-33), keeping the raw
bytes, brings back the blocks in it, which have no times. A shell that has printed any `A` gets a gutter left of the
text, kept across its own replay and removed after a replay with none; there, each prompt whose command has started
(`C`) has a mark: green for exit 0, red for any other code, grey while it runs, and a ring without a code. Pointing at
a mark shows the command line, `Running`, `Exit N`, or why there is no code, and for a command whose `C` and `D` came
after the replay, how long it took. A separator is drawn above a prompt that follows a command's output, and a failed
command is marked on the scroll bar in the theme's red. The bar's **Commands** button, off with no shell in the
focused pane, lists that pane's blocks whose command has started, newest first, each with its state, command line,
and how it ended, narrowed by a filter on the command line, or says where commands come from when there are none;
choosing one by click or by the arrow keys and Enter scrolls the pane to its prompt and lights the prompt up for 1.5
seconds, and a row's **Copy output** and **Copy command** copy those without scrolling (output read when asked, so
far for one running; none printed says so). While the pointer is over a command's prompt or output, outside the
alternate screen, a **Copy output** button shows at the right end of its prompt's line, when that line is in view, and
copies the same way; a press on it starts no selection. The panel takes no key from the shell for blocks. Clicking a mark focuses its pane and offers **Copy output**, **Copy command**, **Select
output** (the terminal's selection), and **Type command again**, which types a one-line command line without Enter
under WEB-45's rule (only after the shell's preview shows nothing running at its prompt, saying why otherwise) and is
off for a command of more than one line. Implemented in the panel in `web_shell.js`; covered by `TestTerminalPanel`
and a step of [`TestTerminalPanelInBrowser`](../conformance/05-web/terminal_panel_browser_test.go).

**WEB-47** `rotari web` run at a terminal, or with `--disconnect-action detach`, starts its server as `--detach`
does (WEB-40), in a session of its own, and stays attached as its client: it shows the server's output, including
`rotari web listening at URL`, and then says on stderr `Press Ctrl-D to detach and keep the server running; Ctrl-C to
stop it.` (only when stdin is a terminal) and, with the terminal on, `Shells end D after the last page closes
(--terminal-idle-timeout), and when the server stops.` with the limit in effect written without zero units (`3h`,
`1h30m`), or, with no limit, `Shells end when the server stops or you sign out.` `rotari web --detach` prints the same idle-limit line, for a new server and for one it reports. Ctrl-C stops
the server and the client exits with its status. Ctrl-D detaches it: the server takes the host's lease, records
itself, and moves its output to the host's log, and the client prints `rotari web keeps running in the background at
URL (PID N)`, the log, and `rotari web --shutdown`, and exits 0. While another server is detached on the host, Ctrl-D
is refused, naming that server, and the client stays attached. SIGHUP or SIGTERM to the client, a terminal that went
away, or a client gone without a word (even by SIGKILL, which the server sees as the end of a pipe) follow
`--disconnect-action`: `stop` (the default) stops the server, and `detach` detaches it as Ctrl-D does, or, when it
cannot, stops it and says why in the log. `--disconnect-action detach` fails at start while a server is already
detached. The action comes from the flag, then `disconnect-action` under `[web]` in a config, then
`ROTARI_WEB_DISCONNECT_ACTION`. A top-level `disconnect-action` and `ROTARI_DISCONNECT_ACTION`, which are a run's, do
not apply. A bad value is an error naming its source. `--disconnect-action` with `--detach` or `--static-dir` is an
error. Started without a terminal and with `stop`, `rotari web` serves in its own process and starts none. Implemented
in `cmd/rotari/web_client.go` and the attached mode of `cmd/rotari/web_detach.go`; covered by the tests in
[conformance/05-web/web_client_test.go](../conformance/05-web/web_client_test.go) and the unit tests
`TestShortDuration` and `TestWatchWebClientInput`.

**WEB-46** With `rotari web --allow-terminal`, words a shell prints are links when they name something that exists.
`POST /_terminal/shells/ID/links` takes a JSON body `{"candidates": [...]}`, at most 64 candidates of at most 4096
bytes each (400 beyond, 415 for a body that is not JSON, unknown fields 400), and answers `links`, one for each
distinct candidate that names something, in the order asked, with its `text` and `kind`:
- `run`: a run ID (`YYYYMMDD-HHMMSS-` and eight lowercase hex digits) of a run of the shell's project, else of another
  project of its basedir, else, for a shell of no project, of a project of any registered basedir; the first project
  holding it gives `project` and `link`, its run page, under `/_basedir/ID` for a basedir other than the server's own.
- `job`: a job ID (nine lowercase hex digits, or an array task's `ID-N`) of the shell's project only, in its newest 20
  runs, newest first, linking to that run's page with `?job_id=`, or else in its queue, linking to the project's page.
  A shell of a basedir's own pages or of no project links no job, and a word of that form that is no such job is no
  link.
- `dir` or `file`: a candidate holding `/`, absolute or relative to the shell's working directory when asked, with a
  trailing `:LINE` or `:LINE:COL` left out, that is a directory or a regular file on this host now; `path` is its
  absolute, cleaned path.

`dir_error` says why relative paths could not be resolved, and `problems` what could not be read while looking (a
run's snapshot, the queue, a basedir's projects); neither turns the answer into an error. A shell not running is
404, a method but POST 405, and the route is served only where the terminal is (404 without it), behind its checks.
The panel finds candidates on the line under the pointer, its wrapped parts joined, leaving out trailing
punctuation and words holding `://`, which stay web links; it asks once per line, keeps answers for IDs for the
page's life and for paths a few seconds, and underlines only what is answered, a link inside a longer one giving
way to it. Pointing at a link says what it opens. Clicking a run or job takes the page there with the panel's
shells kept (with the panel in a window of its own, the page it belongs to); Ctrl- or Cmd-click opens it in a new
tab. A directory shows in the Explorer with Follow off, and a file opens in the Table viewer (WEB-42). When the
server cannot be asked, nothing is a link and a note says so once. Implemented in
`internal/webui/terminal_links.go` and the panel in `web_shell.js`; covered by
[`TestTerminalOutputLinks`](../conformance/05-web/terminal_links_test.go), `TestTerminalLinksAreWhatExists`,
`TestTerminalPanel`, and a step of `TestTerminalPanelInBrowser`.

**WEB-45** With `rotari web --allow-terminal`, the shells of one group (a project's, a basedir's own, or those of no
project, as WEB-34 groups them) share a history of where they have been. The server reads a shell's own working
directory once its output has paused at its prompt (the shell holding its terminal's foreground) after input with a
carriage return or line feed, or, when more such input comes before that, at once, before passing it on; a directory
other than the one it last read for that shell goes to the front of the group's history; where a shell starts enters when it starts, named without symbolic links. Each directory appears
once, newest first, at most 20. A directory changed by a program in front of the shell, such as a nested shell or
`ssh`, is not seen, nor one left before the prompt returns. The history is kept only in memory: a shell ending leaves
it, and it is forgotten whenever every shell ends (signing out, the idle limit, the server stopping). `GET
/_terminal/shells/ID/dirs` answers `history` (`dir`, `entered_at`); for a project's shell, `runs`, where its newest
runs started in `rotari show`'s order, read from at most the latest 20 runs and each directory once, at most five, with
`run` and `started_at` from the run ID, a run whose context cannot be read or names no directory listed with `error`
in place of `dir`, and `runs_error` beside the history when the runs cannot be listed; for a basedir's own shell, the
`workspace` it belongs to, if any, or its `error`. Every directory carries `not_utf8` (U+FFFD in place of bad bytes),
`missing` (not a directory on this host now), and `stat_error`. A shell not running is 404, a method but GET 405, and
the route is served only where the terminal is (404 without it), behind its checks. The panel's **Recent
directories** button, and the Explorer's path, open a dropdown of these, with a filter, under **Recent**, **Runs**,
and **Workspace**; arrow keys move over the rows that can be chosen, Enter or a click chooses, and Escape closes it
and gives the keys back to the shell. Choosing asks the shell's preview first and types `cd` and the path, quoted as
the send buttons quote one word, without Enter, through the send buttons' rules; when a command holds the
foreground, or that cannot be told, it types nothing and a note says why. A row also shows its directory in the
Explorer with Follow off, opens a new tab of the page's shells there (WEB-34's `dir`), or copies the path. A missing
directory can only be copied, and a name marked `not_utf8` or a run with an `error` offers nothing; a refused request
is shown in the list. Implemented in `Keeper.History` in `internal/terminal`, `internal/webui/terminal_dirs.go`, and
the panel in `web_shell.js`; covered by [`TestTerminalRecentDirectories`](../conformance/05-web/terminal_dirs_test.go),
`TestHistoryIsWhereAGroupsShellsHaveBeen`, `TestHistoryKeepsTwentyDirectories`,
`TestHistoryKeepsTheStartOfAShellMovedAtOnce`, `TestHistoryKeepsADirectoryLeftAtOnce`, `TestTerminalDirsOfAShellsGroup`,
`TestTerminalDirsReadTheNewestTwentyRuns`, `TestTerminalPanel`, and a step of `TestTerminalPanelInBrowser`.

**WEB-44** With `rotari web --allow-terminal`, the terminal's Explorer downloads and uploads. A row's menu has
**Download**, a file as it is and a folder as `name.zip`, and **Upload...**, into a folder or a file's folder, as
does the tree's empty part (into its top); the browser's file chooser takes one or more files. Files dragged in from
outside the page onto a folder, a file's row, or the tree's empty part upload there, a closed folder held over opening
after 700 ms; a folder dropped with them is not uploaded, saying so, and files dropped elsewhere in the panel are not
left to the browser to open, the Explorer saying where to drop them. A row dragged out of the browser carries
`DownloadURL` (followed by Chrome and Edge) unless its name holds `:`. `GET /_terminal/files/download?path=/ABS`
answers a regular file as `application/octet-stream` with ranges, and a folder as `application/zip` written while it
is read, its entries under the folder's name, folders included, symbolic links and special files left out; both as
attachments with the name. A failure partway through a zip, such as a file it cannot read, aborts the connection
rather than ending a short archive. Anything but a file or folder is 400, a path that cannot be read 422. `POST
/_terminal/files/upload?dir=/ABS&name=NAME` writes the body, which must be `application/octet-stream` (415), as the
new file, under a temporary name in that folder first and then renamed into place without replacing, so a broken
upload leaves nothing; a name taken is 409, a bad name (as WEB-43) 400, and other failures 422, each with the reason,
and success answers `{"path"}`. Uploads go one at a time through `XMLHttpRequest`, the footer naming the file and how far it has come, as a
percentage, the bytes sent of the total, and a bar, and a note says how many
uploaded and why any did not. Both routes are served only where the terminal is, behind its checks. Implemented in
`internal/fileops` (`WriteNew`, `WriteZip`), `serveTerminalDownload` and `serveTerminalUpload` in
`internal/webui/terminal_files.go`, and the Explorer in `web_shell.js`; covered by
[`TestTerminalExplorerDownloadsAndUploads`](../conformance/05-web/terminal_explorer_test.go), the tests of
`internal/fileops`, `TestTerminalFilesDownloadAndUpload`, and `TestTerminalPanel`.

**WEB-43** With `rotari web --allow-terminal`, the terminal's Explorer (WEB-42) changes files: **New File** and
**New Folder** in its head and menu, and **Rename** (F2), **Delete** (the Delete key), **Cut**, **Copy**, and
**Paste** (Ctrl+X, C, V) in a row's menu, with New File, New Folder, and Paste on the tree's empty part. A new or
pasted item goes into the folder chosen, a file's folder, or the tree's top. Names are asked with the browser's
prompt, and deletions and moves by dragging are confirmed first: a folder's deletion names how many entries it holds
directly, from one listing, and says everything below goes. A row dragged onto a folder, a file's row (its folder),
or the tree's empty part (its top) moves there after confirming; one held over a closed folder for 700 ms opens it;
a drop on a pane still types the path. Copy is for files: a folder's Copy is disabled, and Ctrl+C on one says to use
`cp -r`. The routes are `POST /_terminal/files/mkdir` and `create` with `path`, `move` with `from` and `to`, `copy`
with `from`, `to`, and `unique`, and `delete` with `path`, each a JSON body (415 otherwise; unknown fields 400),
served only where the terminal is (404 without it) behind its token, `Host` and `Origin` checks, and HTTPS or a
direct loopback connection, each answering `{"path"}`, the path made or changed. Paths are absolute, valid UTF-8,
and NUL-free (400), and a new name is refused when empty, `.`, `..`, or holding `/`, `\`, or a control character
(400). Nothing that exists is replaced (409, with `renameat2` and `RENAME_NOREPLACE` on Linux, or a check before
renaming where that is not supported); a move into itself (400) or to where it is (409) is refused, and a move
between file systems is refused, saying to use `mv`, rather than copied. Copying a folder or anything but a regular
file or a link is refused (400); a link is copied as the link, a file with its permission bits, and a failed copy
leaves nothing. With `unique`, a taken name gives `name copy.ext`, then `name copy 2.ext`; without it, 409. Delete
removes a folder with everything in it and a link without its target, refuses `/` (400), and a path not there is an
error (422), as is any other failure, with the reason. Implemented in `internal/fileops`,
`serveTerminalFileChange` in `internal/webui/terminal_files.go`, and the Explorer in `web_shell.js`; covered by
[`TestTerminalExplorerChangesFiles`](../conformance/05-web/terminal_explorer_test.go), the tests of
`internal/fileops`, `TestTerminalFilesChange`, and `TestTerminalPanel`.

**WEB-42** With `rotari web --allow-terminal`, the terminal panel has an Explorer: a tree of the files where the
focused pane's shell is: a column at the left of the panel's body wherever the panel is, beside the panes, which keep
at least 200 pixels however far its edge is dragged. It is open unless closed with the first button of the bar, and
the browser remembers that and its width.
While **Follow** is on, its root is the shell's directory, looked up on the server and listed again with the panel's
shells, shortly after Enter is typed in that pane, when another shell is shown there, and on **Refresh**; **Up**
shows the parent and turns Follow off, and nothing limits how far up or down it goes. `GET
/_terminal/files?shell=ID` lists the directory a running shell is in now (404 for a shell not running), and
`?path=/ABS` any directory; one of the two, and the path absolute and valid UTF-8 without NUL (400 otherwise). It is
served only where the terminal is (404 without it), behind its token, `Host` check, and HTTPS or a direct loopback
connection (WEB-34). The answer is the directory, its immediate entries, folders (and links to folders) first and
then by name, each with its kind (`dir`, `file`, `symlink`, `other`) and, for a link, its target and the kind of what
it reaches (`missing` if nothing), at most 2,000 of them, and `total`; a name or target that is not valid UTF-8 comes
with U+FFFD in place of its bad bytes and is marked `not_utf8` or `target_not_utf8`. A directory that cannot be
listed, and a shell whose directory is unknown or not valid UTF-8, are a 422 saying why, never an empty listing. The
tree shows such a refusal in place, says how many entries it leaves out with a link to the folder in the Table viewer,
shows control characters in names as escapes, and offers nothing for a name marked `not_utf8`. A row's buttons and
menu send its path to the terminal, relative to the shell's directory when under it and otherwise absolute, through
the send buttons' rules (one word, quoted as `shellQuote` does, refused with a note for control characters); open it
in the Table viewer; open a new tab of the page's shells in that folder, or a file's folder; or copy it whole or
from the shell's directory. A new tab asks `POST /_terminal/shells` with `dir` (WEB-34). A row dropped on a pane sends its path to that
pane. Opening asks `POST /api/tableview-sources` with `{"path": "/ABS"}`, which needs the terminal (403 without)
and HTTPS or a direct loopback connection (403 otherwise), takes either `path` or `shell` (400 for both), follows
symbolic links (400 for a path that does not resolve), and pins the folder, or a file's folder with the file's name
answered as `child`, as WEB-39 pins a shell's directory but reading under `/`; the page opens it in a tab opened at
once and closes that tab if refused. Implemented in `internal/webui/terminal_files.go`, `serveExplorerTableView` in
`internal/webui/table_view_shell.go`, and the Explorer in `web_shell.js`; covered by
[`TestTerminalExplorerListsAndOpensFiles`](../conformance/05-web/terminal_explorer_test.go), the unit tests
`TestTerminalFilesListADirectory`, `TestTerminalFilesSayHowManyTheyLeaveOut`, `TestTerminalFilesRefuseWithTheReason`,
and `TestTableViewOnAnExplorerDirectory`, and `TestTerminalPanel`.

The send buttons (WEB-33's panel, the copy buttons' send buttons) and the commands copy buttons show quote a value
with `shellQuote` in `web_shell.js`: in single quotes, with each `'` and `\` outside them as `\'` and `\\`, which
POSIX shells, tcsh, and fish read alike; a value of only letters, digits, and `_/.:=@%+,-` goes in as it is.

**WEB-41** A live server's `POST /_server/shutdown` stops it as SIGTERM does, whether it was detached or not: it
answers 202 first, then closes launches, shells, and the listener in the same order, and a detached server removes its
record. It needs job control (403 without), a JSON content type (415 otherwise, so a cross-site form cannot send it),
and `POST` (405 otherwise); with a token, it needs the token as every route does. A server built without it, such as a
static export, has no route (404). Live application and Recent jobs pages with job control show **Stop server** in
the Settings menu, above the version. It asks first, as signing out does: it names what runs in each shell when the
terminal is on, says when that could not be checked, and otherwise asks whether to stop the server at all. It then
sends the request and says beside the button that the server stopped, or why it did not, leaving the button usable
again. Implemented in `internal/webui/shutdown.go`, the shutdown wiring of `serveWeb` in `cmd/rotari/web.go`, and
`stopServer` in `web_shell.js`; covered by [`TestServerShutdownRoute`](../conformance/05-web/web_shutdown_test.go),
the unit test `TestShutdownRoute`, and `TestTerminalPanel`.

**WEB-40** `rotari web --detach` (command line only; rejected with `--static-dir`) starts the
live server in a session of its own, with no terminal and its output in `<masterdir>/web/<host>.log`, and returns only
once it listens, printing `rotari web listening at URL in the background (PID N)`, the log, and `rotari web --shutdown`
as the way to stop it; a server that exits or does not listen within 30 seconds makes it fail with the reason, the end of the log,
and the log's path, and is stopped. A token it needs and was not given is generated, printed once, and reaches the
server in its environment, never in its arguments; the server's record, `<masterdir>/web/<host>.json` (mode 0600),
holds a digest of the token, never the token, and the binary the server runs (CLI-31). There is at most one detached server per user and host: while one
holds the host's lease, `--detach` reports it (`already running since ...`, without the token) and starts none, or,
when the options given differ from its own, fails naming them and the PID. Liveness is the server's `flock` lease, so
a server killed without cleaning up does not count and its record is removed when found; only the current host's
files count, and a lease that cannot be probed is an error, never "not running". SIGTERM shuts it down as WEB-34 says
and removes its record. Its shells see neither the token nor the mark of the detached process. A `rotari web` without
`--detach` is unchanged and not counted, and says on stderr which detached server runs on the host. The log keeps
about 1 MiB and one rotated `.log.1`, rotated when a server starts and, while it runs, copied and truncated in place,
never relying on `rotari gc`. `rotari web --shutdown` (command line only, taking no other option but `--yes` and
`--config`; `--yes` alone is an error) stops this host's detached server: with none it says so and exits 0. Before
stopping, it lists the server's shells that run something in their foreground or in the background, read from the
process table, since it cannot ask the server, or says that this could not be checked. When there is anything to list
it asks at a terminal, and without one it fails unless `--yes` is given. It then sends SIGTERM and returns only once
the server's lease is released, saying it stopped, or fails naming the PID after 30 seconds. Implemented in
`cmd/rotari/web_detach.go`, `cmd/rotari/web_shutdown.go`, `internal/webdetach`, `internal/lease`, and
`ServerShells` in `internal/terminal`; covered by the tests in
[conformance/05-web/web_detach_test.go](../conformance/05-web/web_detach_test.go) and
[`TestWebShutdownStopsTheDetachedServer`](../conformance/05-web/web_shutdown_test.go), and the unit tests of those
packages.

**WEB-39** With `rotari web --allow-terminal`, with or without `--allow-exec`, the
Table viewer may show the directory a running shell is in. `POST
/api/tableview-sources` with `{"shell": ID}`, and nothing else (400 for another
field; a `path` instead is the Explorer's, WEB-42), looks the shell's directory up on the server as WEB-38
does and answers a random `source` id and that `dir`; it is refused, saying why,
for a shell not running (400) and a server without the terminal (403).
`/tableview/?source=ID` then shows that directory, pinned when the id was issued:
the shell changing directory or ending does not move the view. It reads only
under that directory (`child` as for `rotari tableview`), takes no project, run,
job, attempt, or entry beside `source` (400), and an id this server does not hold,
because it was never issued, was dropped as the least recently used beyond the
64 held, or was issued before a restart, is a 404 saying to open the viewer again.
The terminal's launch dialog (WEB-38) lists the Web UI's viewers that accept a
directory above the commands, as links that open in a new tab, and still lists
them, saying why commands are not offered, without `--allow-exec`. Implemented in
`internal/webui/table_view_shell.go` and the launch dialog in `web_shell.js`;
covered by [`TestTableViewerOnAShellsDirectory`](../conformance/05-web/launch_test.go),
`TestTableViewOnAShellsDirectory`, `TestShellTableViewsDropTheLeastRecentlyUsed`,
and `TestTerminalPanel`.

**WEB-38** With `rotari web --allow-terminal --allow-exec`, a launch (`POST
/api/launch`, `GET /api/launch-preview`) may name a running shell with `shell`
instead of an artifact: the target is the directory that shell is in now, looked
up on the server, launched as a directory with the command's own checks and
labelled with the shell's number and project. Such a request names nothing else
(400 with a project, run, job, attempt, entry, or child), and is refused, saying
why, for a shell not running, one whose directory cannot be found, and a server
without the terminal. **Launch here** in the terminal's bar, for the shell of the
pane with the keys, and on each All shells card open a dialog of the commands that
accept that directory, what one would run, and, once launched, a service's Open
link. Implemented by `shellLaunchStart` in `internal/webui/launches.go` and the
launch dialog in `web_shell.js`; covered by
[`TestLaunchOnAShellsDirectory`](../conformance/05-web/launch_test.go), the unit
test of that name, and `TestTerminalPanel`.

**WEB-37** The Settings menu of a live application or Recent jobs page ends with
which rotari serves it: the version `rotari version` prints, and the path of the
running binary with buttons to copy it and, with the terminal, to send it to the
terminal as one word, or why that path is not known. A static export says
neither. Implemented by `webAbout` in `cmd/rotari/web.go` and `aboutSection` in
`internal/webui/assets.go`; covered by
[`TestSettingsSayWhichRotariServes`](../conformance/05-web/about_test.go), and by
the unit test of that name and `TestTerminalPanel`.

**WEB-36** `--allow-terminal`, `--allow-exec`, `--allowed-host`, and the `--terminal-*` options of `rotari web` have
environment variables (`ROTARI_WEB_ALLOW_TERMINAL` and so on) and config keys, at the top level or in `[web]`, as other
options do. Because they let other people reach this host, `--allow-terminal`, `--allow-exec`, and `--allowed-host`
are read only from the command line, the environment, the global config, and a file named with `--config`; and so are
`--host` and `--auth-token` when the terminal or launchers end up on. A workspace, basedir, or project config's value
for them is ignored, the next place in the order (a trusted config, then the built-in default) supplying the value, and
unless the command line or the environment replaces it anyway, `rotari web` says on stderr, before serving, which key,
value, and file it ignored. It never shows a token. The `--terminal-*` limits, `--terminal-shell`, and `--terminal-shell-integration` are read from every
config. A value for these options from the environment or a config that does not parse is an error, and one out of
range is when the terminal is on; either names the variable or file it came from, and the server does not start. An
empty variable counts as unset. `rotari config` templates for a workspace, basedir, or project leave out the options
those configs cannot set. The Web UI can save the global config, so a save from it that would add, change, or remove
`auth-token`, at the top level or in `[web]`, in any config file, is refused, naming the file to edit directly, and the
file is left as it was; the terminal and launchers always require a token, which a page that can save config therefore
cannot choose. Implemented in [cmd/rotari/web_config.go](../cmd/rotari/web_config.go) and `keepsWebAuthToken` in
[internal/webui/webui.go](../internal/webui/webui.go); tested by `TestWebTrustedOptionsComeOnlyFromTrustedConfigs` and
`TestWebConfiguredBadValuesAreErrors` in [cmd/rotari/web_config_test.go](../cmd/rotari/web_config_test.go),
`TestWebSaveConfigCannotChangeAuthToken` in [internal/webui/webui_test.go](../internal/webui/webui_test.go), and through
the binary in [conformance/05-web/terminal_config_test.go](../conformance/05-web/terminal_config_test.go).

**WEB-33** `rotari web --allow-terminal` adds a Terminal button to the top bar of the
application and Recent jobs pages, which shows how many shells there are in all projects, as `Terminal (3)`,
with the panel open or not and without connecting to any, on a page in view; with no answer from the server it
shows no number. Each project row in the sidebar whose project has shells is marked with how many, and so is a
basedir's All projects row when the basedir has shells of its own. It opens a panel along the bottom, or down the left or right of the window as the browser
keeps from a menu in the panel's bar, with a tab for each
shell of the page's project; on a basedir's other pages, such as All projects and Recent jobs, of that basedir's
own; and on Activity and History search, of no project. A list names the projects with
shells, and choosing one goes to its page. A tab not shown whose `command_finished` is set shows a bell in place of
its unseen-output mark, as its All shells card does. A tab's tooltip and an All shells card say when the shell
started and how long it has been idle, by the server's clock. A card's line of what runs also says how many
processes the shell has started, background ones when nothing runs in its foreground, and opens into their tree. Signing out from a page with the terminal first asks, naming
each shell with a command `running` or processes below it and what runs there, since signing out ends every shell;
a shell whose processes cannot be listed is named as such, it asks too, saying why, when the shells cannot be
listed, and signs out at once when nothing runs. **All shells** in the bar opens a dialog over the page, as a log's is,
with the panel's room left to it: every running shell, grouped by project with the page's first, each with its tab
name, its shell's working directory now, or where it started, said as such, when that cannot be found, and the
command running in its foreground, if any, or that it is not known (WEB-34); it connects to none. Under each
project's name a line says what its last run is doing (WEB-34): its lifecycle, when it ended or, while it goes on, when it started, and its
jobs in progress, pending, and failed, as its run page's Jobs card counts them, or that it has no runs, or why it
could not be read; choosing the line closes the dialog and opens that run's page, a window taking its page along.
The projects are read each time the dialog is drawn, not with the list of shells. Choosing one
closes the dialog and shows the shell in the pane with the keys, as its tab would, or for another project has that
project's page show it in the pane with the keys and goes there, as the list does, a window taking its page along.
Each card also renames its tab and ends its shell, after asking and naming another project's shell by its project,
as a tab's menu and close button do, and **End all** beside a project's name and **End all shells** at its top end
that project's shells and every shell, after one question, leaving the dialog open and drawn again; a refusal is
told in the dialog. A
project's **+** starts another shell there, off at the limit and saying which, and **New shell in**, a list of every
basedir with no shell of its own, each of its projects with no shell, and no project, starts one where the page
of the choice would start it (WEB-34), shown in no pane but as a card with the keys; a basedir whose projects cannot be
listed is named.
It is a view of when it opened, or of **Refresh**, and Escape, **Close**, a click beside it, or hiding the panel
closes it. Opening the panel with no shell there starts one, **+**
starts another, and a run page's **Open a terminal here** button starts another, each where the page
says (WEB-34); closing a tab asks, then ends that shell. A tab's menu also closes the tabs to its right, all
of the project's tabs, and every shell of every project, each after one question saying how many shells and whose.
Every question to end shells, from a tab, its menu, or All shells, says how many processes run in them, naming up to
three commands, or that nothing does, or why that is not known. A page loaded with the panel left open starts
nothing, and offers to, so browsing leaves no shells behind. Shells live on the server: each page load
lists them and attaches to the one this browser tab last showed for the project, or the first,
replaying recent output into a pane that keeps 10,000 lines, so following a link ends none of them. Up to four panes show the project's shells, as columns each holding rows. **Split right** adds a column of
the whole height beside the pane with the keys, and **Split down** a row under it, showing a shell in no pane,
or a new one if there is none, in the new pane, which takes the keys. Neither starts a shell past the limit, and a
split that would leave a pane under 200 by 100 pixels is refused, saying so. **Close pane** closes the pane with the
keys and leaves its shell running, its room and the keys going to its neighbor. There is one row of tabs, and a tab
shows in the pane with the keys; a shell is never in two panes, so choosing another pane's tab exchanges the
panes' places without connecting again. Each pane says what its own connection is doing, and one taken
over or reconnecting does not disturb the others. The panes and their shares belong to the browser tab and
project, and come back with the next page, without any shell that has gone. Only shells shown in a pane are connected. A
shell ending moves its pane to a tab of the project in no pane, without taking it from another browser tab;
with none, the pane closes, and when the project's last one ends, the panel says how and a new shell is a choice. **Maximize** gives the
panel the whole window until **Restore**, and **A-** and **A+** set the text size between 8 and 32; the
browser keeps both from page to page. An `http` or `https` address in the output opens in a new tab on a
click, with no access to the page. Each one-line copy
button has a send button beside it that types into the shell of the pane with the keys with no Enter, opening the panel if
needed: a suggested command as its commands on one line, a job's command as it is, and any other value as
one word, quoted if needed. A dialog's text and a log go in as pasted text with no control characters but
line breaks and tabs, up to 256 KiB; with several lines, only where the program has turned on bracketed
paste, since elsewhere each line would run. It starts no shell, takes none from another browser tab, and refuses text with
control characters, saying why in each case; a page with no terminal has no such button. The panel's requests
go to the server's root on a page under a basedir's mount too.
At a side the panel has the window's height and a width of its own, from 320 pixels up to what leaves 400 beside the sidebar, and the page, its
top bar, which wraps rather than running under the panel, and its dialogs take the rest; under 1000 pixels wide it is at the bottom, and the choice stays for a wider window. Moving it changes no
connection, and panes stack when the panel is under 720 pixels wide, keeping their columns for when it has room.
**Window** in the same menu opens the panel in a browser window of its own, from the click, on the page's project
with the panes this browser tab had for each project; the window fills itself, has no resizer, Maximize, or Hide,
and its project list, as the panel's does, takes the page that opened it to that project's page, the window
switching with it. When the page moves to another project's page, or to a page of another basedir's own, such as
its All projects or Recent jobs, the window switches to that project or basedir; a page of no project, such as
Activity, or of the project or basedir it last followed, leaves it. The page lets go of its shells without
saying another tab took them, gives up the panel's room, and keeps whether the panel is open for other pages.
While the window is open, the Terminal button brings it to the front, a send button types into the window's pane
with the keys and names its project when it is not the page's, and **Open a terminal here** starts the shell
in the window, on the page's project. A link followed in that browser tab finds the window again rather than taking its
shells, and the panel comes back if the window does not answer within three seconds. Closing the window, or its
**Back to page**, puts the panel back where it was with the window's panes for the page's project. Closing the
browser tab that opened the window closes the window within a few seconds, leaving its shells running; following
links and reloading in that tab keep it. A window opened by its address has no such tab, and its **Back to page**
says it has nowhere to go. A blocked window leaves the panel where it was, saying why.
xterm.js loads when the panel first opens, under the page's nonce. A browser tab that is not visible
neither connects nor retries, and a page whose sign-in has gone stops retrying and says so
(`TestTerminalPanel`). `TestTerminalPanelInBrowser` checks the panel in
headless Chrome.

**WEB-29** Live application, Recent jobs, and Table viewer pages send a
`Content-Security-Policy` (`pagePolicy` in `internal/webui/page_policy.go`). Only
scripts with that response's nonce run, requests stay on the same origin, and
styles may be inline, since data-driven sizes and colours are. The nonce goes in
with the page's other values in one pass, so a value holding the placeholder's text
cannot obtain it. A launched service's responses under `/_launch/` and served
artifact files keep their own headers. A static export has no response headers, so
it has no policy or nonce. Pages have no inline handlers: a control names an action
in `data-action`, `data-change-action`, or `data-toggle-action`, with arguments as
JSON in `data-args`, and `web_shell.js` runs only the actions it registers
(`TestPageActionsAreRegistered`, `TestAssetsHaveNoInlineHandlers`).
`TestPagePolicyBlocksPlantedScript` checks in headless Chrome that a planted handler
and script do not run and are reported.

**WEB-28** Values recorded in state, logs, notes, and artifact files stay text.
They do not become markup or run, when a live page, a static export page, or the
Table viewer renders them or when a control on the page is clicked. Controls
carry such values as data for `web_shell.js` to pass on, not as JavaScript. `TestStoredValuesStayText` covers basedir paths;
project, job, stage, matrix, and array names; commands, working directories, and
executor options; logs, diagnoses, notes, and config files; artifact names and
contents; and launcher names, targets, and output, in jsdom. It also opens the
recovery dialog, whose reasons the CLI cannot make hostile.
`TestStoredValuesStayTextInBrowser` repeats this in headless Chrome, for the
values that attack markup and a quoted argument, and fails on any
Content-Security-Policy violation. It skips without Chrome, except where
`ROTARI_REQUIRE_BROWSER_TESTS` is set, as in CI.

**WEB-26** Values go into a page's template in one pass that never searches
what it inserted, so a project name, job command, or other value containing the
text of a template placeholder stays as written, on the live pages and in a
static export. A value inserted earlier and searched later could otherwise take
another placeholder's value, and that value's quotes could end the string
around it. Covered by `TestPagesKeepPlaceholderTextInValues`.

`cliDocsHTML` and `environmentHTML` are still generated by Go because they
iterate over Go metadata and escape dynamic values. The metadata comes from
the CLI through `webui.Options` (`Commands`, `Environments`). They are separate from the
main interactive Web assets and should not be folded into the JavaScript app.
