# Plan: Web UI hardening, then a built-in terminal

Created: 2026-11-04
Status: Phase 1 is done, with the gaps listed in
[its results](#phase-1-results). Phase 2 is next. Phases 1 and 2 are worth doing on
their own; [the gate](#gate-decide-whether-to-build-the-terminal) decides whether
phases 3 to 5 happen at all.

## Goal

Make script injection in the Web UI hard to introduce and harmless when it
slips through, then, if still wanted, add an interactive shell to `rotari web`
without making the Web UI's weakest page a route to the user's whole account.

The order is the point. Today an injected script can do what the Web UI can do:
cancel or edit jobs, read served files and configs, and start registered
launchers. With a terminal it could do anything the user's account can do.

## Existing implementation

- [Web assets](../../internal/webui/assets/) build most of the UI as HTML
  strings. They hold 91 lines using an HTML sink (`innerHTML`, `outerHTML`,
  `insertAdjacentHTML`), 70 lines generating inline event handlers, and 19 lines
  of inline handlers in `web_template.html` and `jobs_template.html`. Counts are
  grep line counts, for scale.
- `esc` ([web_app_bootstrap.js](../../internal/webui/assets/web_app_bootstrap.js))
  is correct for HTML text and attribute values. `jsArg`
  ([web_app_artifacts.js](../../internal/webui/assets/web_app_artifacts.js)) is
  correct for handler arguments, and `this.dataset` handlers avoid the problem.
- Live pages send no Content-Security-Policy. Only the login page
  ([auth.go](../../internal/webui/auth.go)) and served artifact files
  ([artifact_files.go](../../internal/webui/artifact_files.go)) do.
- [assets.go](../../internal/webui/assets.go) inlines all page scripts into the
  template, and static exports depend on being plain files.
- Session and launch cookies are `HttpOnly` and `SameSite=Strict`, and no page
  script holds the Web token.
- `WithExecGuard` ([launch_guard.go](../../internal/webui/launch_guard.go))
  checks `Host` and `Origin`, including WebSocket handshakes, but is installed
  only with `--allow-exec`.
- `internal/launcher` runs commands without stdin, under a wrapper that stops
  their process group. A terminal cannot reuse this process model, though
  `launchBaseEnvironment` already removes the Web token from a child's
  environment.
- Go has no server WebSocket in its standard library and `go.mod` has no
  WebSocket or PTY module. `http.Server` in
  [web.go](../../cmd/rotari/web.go) sets no timeouts, so long connections work,
  but `Shutdown` does not close hijacked connections.
- Launched services share the Web UI's origin
  ([SECURITY.md](../../docs/SECURITY.md#launching-services-and-commands)), so
  their pages can call the rotari API as the signed-in browser.

## Fixed vulnerability

A value containing `'` could break out of the `showArtifacts` and
`selectJobAttempt` handlers in
[web_app_core.js](../../internal/webui/assets/web_app_core.js) and run script
when clicked. `esc` is the wrong tool in that position: the browser decodes
`&#39;` back to `'` in the attribute value before parsing it as JavaScript.

Both handlers now use `jsArg`, covered by
[TestWebJobActionHandlersKeepIdentityAsData](../../internal/webui/job_action_handlers_test.go);
see [ISSUES.md](../ISSUES.md). A scan of multi-line generated handlers found no
other argument built with `esc` alone, but that is not the audit in phase 1.

## Scope and non-goals

In scope: the live Web UI, Table viewer, and static export scripts (phase 1);
CSP on live pages (phase 2); and a shell for the user running `rotari web`, on
that host (phases 3 to 5).

Non-goals:

- Multi-user terminals, per-user accounts, or running as another user.
- A sandbox. The shell has all the user's permissions.
- A shell on compute nodes or attaching to a running job.
- Session recording, sharing, or shells that outlive their connection. `tmux`
  inside the terminal covers persistence.
- CSP on static exports in this plan. They have no response headers, and a
  `<meta>` policy needs hashes; revisit after phase 2.
- Defending against another process under the same UID (see
  [Limits](../../docs/SECURITY.md#limits)).

## Decisions

1. **Hardening comes first and ships independently.** It changes no intended
   behavior and helps users who never enable the terminal.
2. **The terminal is off by default, behind its own `--allow-terminal`.**
   `--allow-exec` promises that the browser only names registered commands, and
   that promise stays true without this flag.
3. **`--allow-terminal` always requires a token and installs `WithExecGuard`**,
   even on `127.0.0.1`. It needs job control, and is rejected with
   `--allow-control=false` and `--static-dir`.
4. **The shell opens only over a direct loopback connection or HTTPS.** Reuse the
   browser-login rule. Unlike the rest of the API, an unencrypted token here
   would expose a shell, so API token credentials get no exception.
5. **Opening a terminal requires typing the Web token again.** A session cookie
   alone does not open a shell. This is the control that stops same-origin
   script, which `Origin` checks cannot: neither an injected script nor a
   launched service's page holds the token. A token embedded in the page would
   not work, since same-origin script can fetch the page and read it. An exchange
   gives a short-lived, single-use ticket that the WebSocket handshake presents.

   Residual risk: script already on the page can draw a fake prompt or read
   what the user types. CSP makes that harder, not impossible.
6. **The terminal is its own page**, `/terminal/`, opened in a new tab. It
   renders no job data, loads only xterm.js and a small script, and has the
   strictest CSP in the UI. This also avoids tying shells to the main page's
   rendering and navigation.
7. **Closing the connection ends the session.** No reattach in the first version.
8. **Only the two needed dependencies are added:** `creack/pty` and
   `coder/websocket`. Confirm before phase 3 that each still has no transitive
   dependencies and supports the release targets (Linux and macOS).

## Phase 1: Audit and fix HTML and script injection

Estimate: 2 to 4 days.

1. Classify every sink and generated handler in a table in this plan: file and
   line, the data reaching it, the escaping in use, and safe or fixed. Treat
   everything from state files, logs, configs, and the API as untrusted.
2. Check by output context, not just for the presence of `esc`:
   - HTML text and quoted attributes: `esc`.
   - JavaScript inside an event handler: never `esc` alone.
   - URLs in `href` and `src`: scheme and path encoding. The Markdown renderer
     allows only `http` and `https` links, and needs the same check everywhere.
   - JSON embedded in `<script>`: the static bootstrap and Table viewer.
   - Go-composed HTML in `assets.go`, `auth.go`, and `table_view_static.go`.
   - Dynamic `class` and `style` values, SVG, and chart labels.
3. Add a regression test. A jsdom table test renders hostile values (`'`, `"`,
   `<`, `&`, backtick, and handler text) and clicks each generated control. It
   asserts that no payload ran and that each handler received the exact original
   value, so legitimate names with apostrophes are covered too.

Cover each variant: project, run, job, stage, array task, matrix member, attempt
ID, carried job, command, working directory, note, log and diagnosis text,
notification, artifact and launcher name, table cell, and search result. Render
through the queue, run, job, activity, and search views, in both live and static
modes.

Done when: every sink is classified, no unsafe finding remains, and the test
fails on the code before the fix. Issues outside injection go to `ISSUES.md`.

### Phase 1 results

Two defects were found and fixed: handler arguments built with `esc` alone, and
sequential placeholder replacement (WEB-26). The review found nothing else; the
table below is by area, instead of the per-sink table this phase first planned,
since `TestStoredValuesStayText` (WEB-28) now checks the rendered pages directly.

On 2026-11-04, a TypeScript-parser scan listed the 333 non-literal values
concatenated into strings holding HTML tags, or assigned to an HTML sink. Most
were read at their source, with helpers followed.

| Area | Result |
| --- | --- |
| Handler arguments | `jsArg`, `this.dataset`, or fixed text |
| Template placeholders in Go | One pass that does not search inserted values |
| HTML text and attributes in the page scripts | `esc`, numbers, or fixed text |
| Status and timeline classes | From the `statusTones` table, with `off` as the fallback |
| `entry.id` and `basedirURL` | Base64url IDs made by the server, and `encodeURIComponent` paths |
| Markdown | Escaped text; links only `http` and `https` |
| Highlighted code | highlight.js output, or `highlightEscape` |
| Notification links | Server-built paths under the basedir route |
| Table viewer | DOM properties; `innerHTML` only for fixed icons, `pdfPreviewMarkup`, Markdown, and highlighted code |
| Login `next` | HTML-escaped; `safeWebReturnPath` allows only local paths |
| Static JSON | `json.HTMLEscape` |
| Recent jobs rows | `html.EscapeString` and `url.PathEscape` |

`TestStoredValuesStayText` records four hostile values through the CLI, one each
for HTML, single-quoted, double-quoted, and template-literal contexts. A single
value cannot do this: an early combined value was a syntax error inside a handler,
so the test missed a weakened `jsArg`. The test renders the live pages, every static
export page, and the Table viewer, and clicks their controls. It passes on the
current code. Weakening `esc`, `jsArg`, or the Markdown escape, or restoring the
original `showArtifacts` bug, makes it fail.

Gaps:

- It runs in jsdom, not a browser, and finds script by its effects. A construct
  jsdom does not run, such as an image's `onerror`, is found only as markup.
- It does not store hostile basedir paths, config file contents, diagnoses,
  executor options, or an interrupted run for the recovery dialog.
- POST requests are blocked, so views shown only after a successful one, such as
  a started launch, are not rendered.

## Phase 2: Content-Security-Policy on live pages

Estimate: 3 to 5 days, most of it replacing handlers.

1. Replace the inline handlers with delegated listeners and `data-*`. This is
   mechanical but large, and removes the risky pattern for good.
2. Send a per-response nonce on the inline scripts, keeping pages composed as
   they are now. Static exports keep working without headers.
3. Start from this policy and adjust from observed violations:

   ```text
   default-src 'none'; script-src 'nonce-...'; style-src 'self' 'unsafe-inline';
   img-src 'self' data: blob:; font-src 'self' data:; media-src 'self';
   frame-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self';
   frame-ancestors 'self'; object-src 'none'
   ```

   `style-src 'unsafe-inline'` stays because data-driven sizes and colors are
   set inline by [contract](../../contracts/05-web-assets-and-static-export.md).
   `connect-src 'self'` also limits sending data to another site by `fetch`.
4. Send it on every UI page, and never on `/_launch/` responses, which belong to
   the launched service, or artifact files, which have their own policy.
5. Run it as `Content-Security-Policy-Report-Only` while walking through every
   view, then enforce it.

jsdom does not enforce CSP, so this phase needs a real browser. Headless browsers
are not installed on the main development machine;
[scripts/web-browser.mjs](../../scripts/web-browser.mjs) has the DevTools driver,
which needs Chrome and Node 22 or later.

Done when: a real-browser run through every view shows no violations, and a
planted handler and inline script are blocked. Add a contract ID (next free after
WEB-25) and a conformance test for the header.

## Gate: decide whether to build the terminal

Stop here unless all of these hold:

- Phases 1 and 2 are complete.
- The need is confirmed by use. First try the terminal in the built-in Jupyter
  Lab launch, which already has token protection. Its packages are in this
  repository's `.venv`, but this has not been run. It needs an artifact to
  launch on and has no permanent place in the UI.
- The need is for a general shell. If it is really a few `rotari` operations,
  add them to the Web UI instead.
- The maintainer accepts decisions 2 to 7, including typing the token each time.

Do not substitute `ttyd` in `launchers.toml` on a shared host without checking
how it takes a secret. A service's loopback port is reachable by other users, so
it must require the launch token, which
[must not appear in a process's arguments](../../docs/SECURITY.md#launching-services-and-commands).

## Phase 3: Terminal sessions and transport

Estimate: 3 to 4 days.

- New `internal/terminal`: owns PTYs and sessions and knows nothing of HTTP,
  jobs, or runs. Add its rule to
  [internal/archtest](../../internal/archtest/boundaries_test.go).
- `internal/webui/terminal.go`: the ticket exchange, the WebSocket handshake, and
  frames. Terminal output is binary frames; resize is a control message.
- `cmd/rotari/web.go`: the flag and its combinations, including `--allowed-host`,
  and the guard. Extend the guard's messages, which now say "while launching is
  enabled".
- Start `$SHELL -l` in the home directory, and report a failed start as an error
  rather than falling back silently. Set `TERM`. Share the environment filter
  with launches, so the Web token is never inherited.
- Limit concurrent sessions and refuse with a clear error beyond it. Send
  keepalive pings so proxies do not drop idle shells. Apply backpressure by not
  reading the PTY while a WebSocket write is blocked.
- Log each session's open and close to the server's standard error, with no
  terminal content.

Lifecycle:

| Event | Behavior |
| --- | --- |
| Tab closed or connection lost | Close the PTY; the session gets `SIGHUP`. |
| Shell exits | Send a close frame with the exit status; the page shows it. |
| `rotari web` stops | `RegisterOnShutdown` closes sessions before exit. |
| `rotari web` is killed | The kernel closes the PTY and sends `SIGHUP`. |
| `nohup`, `setsid`, and rotari supervisors | Keep running, as after closing any terminal. |

This is deliberately weaker than launches, which always stop with the server.
Document the difference rather than hiding it.

## Phase 4: Terminal page

Estimate: 1 to 2 days.

- Vendor xterm.js and its fit addon with their licenses in `assets/vendor/`, as
  with highlight.js, served only on `/terminal/` so other pages do not grow.
- A **Terminal** entry shown only when enabled, the token prompt, resize, a clear
  state on disconnect, and no automatic reconnect.
- Check Japanese input and wide characters, copy and paste, `vim`, `less`, `top`,
  and `rotari run` with its Ctrl-C and Ctrl-D controls.

## Phase 5: Contracts and documentation

Estimate: 2 to 3 days.

- Contract notes with new WEB IDs, conformance through the binary, and the status
  table in `contracts/README.md`.
- [WEB.md](../../docs/WEB.md) (including the mode table and the statement about
  what starts runs), [SECURITY.md](../../docs/SECURITY.md), the CLI reference, and
  [ARCHITECTURE.md](../../docs/ARCHITECTURE.md). Include reverse proxy notes for
  WebSocket.
- `cli_spec.go` and the generated Python CLI through its generator. The flag is
  CLI only, and MCP is unaffected.

## Validation

Hardening:

- The phase 1 hostile-value test fails before the fixes and passes after.
- A real-browser walkthrough has no CSP violations; planted script is blocked.
- `go test ./conformance` and `scripts/check.sh` pass, and static export still
  opens from plain files.

Terminal, as conformance tests through the binary where possible:

| Case | Expected |
| --- | --- |
| Without the flag | Terminal routes are absent and the UI entry is hidden. |
| Invalid flag combinations | Rejected with an error, not ignored. |
| Session cookie, no ticket | Handshake refused. |
| Launched service page, or script without the token | No ticket and no shell. |
| Wrong token | Refused. |
| Ticket reused or expired | Refused. |
| Cross-origin, missing `Origin`, or unexpected `Host` | Refused. |
| Plain HTTP beyond direct loopback | Refused, including with API token credentials. |
| Shell environment | Has no `ROTARI_WEB_AUTH_TOKEN`. |
| Connection closed, server stopped, server killed | Session gets `SIGHUP`; no orphan from an ordinary shell. |
| Session limit reached | Clear error. |

Also run on Linux and macOS, and through an HTTPS reverse proxy once by hand.

## Estimate

| Part | Effort |
| --- | --- |
| Phases 1 and 2, hardening | 5 to 9 days |
| Phases 3 to 5, terminal | 6 to 9 days |

These are rough person-day estimates. The phase 1 audit could change them, since
it has only just begun to look at this code.

## Open questions

1. Is typing the token on each terminal open acceptable, or should the terminal
   have its own secret?
2. May `--allow-terminal` and `--allow-exec` be combined in the first version?
   Proposed: yes, given decision 5, since a launched page does not get the token.
   Forbidding the combination is the more cautious option if a review of the
   fake-prompt risk finds it too high.
3. Does the session limit need to be configurable? Proposed: a fixed limit first.
