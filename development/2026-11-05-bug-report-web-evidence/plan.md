# Plan: Web sign-in and startup evidence in `rotari bug-report`

Created: 2026-11-05
Status: Implemented in the working copy. The independent agent check remains to be run from a
deliberately regressed temporary workspace; all product, contract, documentation, and automated
test work below is complete.

## Goal

A `rotari bug-report` bundle should show what happened when a browser could not get into
`rotari web`, and how that server was started and built, so that a report can be diagnosed from
facts rather than from general knowledge.

## Why

Safari could not sign in on `http://localhost`: it discarded the session cookie because of its
`Secure` attribute, and a correct password led back to the sign-in form with no error
([Resolved entry](../ISSUES.md#resolved)). An independent agent given the bundle and the symptom
guessed the cause at 60 to 70%, from general browser knowledge alone. The bundle held nothing
about `rotari web` or sign-in. The same gap was found checking the history search fix; see the
Open item "`bug-report` has no evidence of Web requests or registered basedirs" in
[ISSUES.md](../ISSUES.md).

The two records this plan adds would have read roughly like this for the Safari bug:

```json
{"event":"sign_in","outcome":"accepted","transport":"direct_loopback","cookie_secure":true,"user_agent":"... Safari/605.1.15"}
{"event":"session_missing","cookie":"absent","transport":"direct_loopback","user_agent":"... Safari/605.1.15"}
```

A sign-in that succeeded, followed by a request with no cookie, points at the browser dropping
the cookie.

## Existing implementation

- `rotari web` has no log of its own. It writes startup messages to stderr, and
  [auth.go](../../internal/webui/auth.go) records nothing about sign-in. The only `log.Printf` in
  `internal/webui` is the Table viewer's export error.
- `WithAuthToken` ([auth.go](../../internal/webui/auth.go)) and `handleWebLogin` decide every
  outcome this plan records. `rotari tableview`
  ([tableview.go](../../cmd/rotari/tableview.go)) uses the same `WithAuthToken`, so it has the
  same sign-in behavior. `webSessions.accepts` returns only a bool, so telling an absent cookie
  from an unknown or expired one needs a change.
- [internal/bugreport](../../internal/bugreport/) reads `server.log` from the basedir. `rotari
  web` serves many basedirs, so a Web log does not belong to any one of them.
  [notificationlog](../../internal/notificationlog/log.go) already keeps server-wide history in
  the master directory (`state.ResolveMasterDir`), which is the model to follow.
- `bug-report` with no project or no run still gives an environment report
  (`locateBugReport`), so Web evidence can be collected without a run.
- `versionString` ([version.go](../../cmd/rotari/version.go)) already appends the commit to a
  `dev` build through `vcs.revision`. It reported a bare `rotari dev` for the Safari bundle
  because the binary was built in a secondary Jujutsu workspace, which has no `.git`, so Go
  stamps no revision. Comparing a server's build with `bug-report`'s own therefore needs
  something other than the version string.

## Decisions

1. **A Web event log in the master directory.** JSON Lines, one record per event, mode `0600`,
   at `<master>/web-events.jsonl`. Every `rotari web` and `rotari tableview` server on the host
   appends to it, each record with a single write, and each record carries the server's PID.
2. **Sign-in events, not an access log.** Recording every request is a larger decision and is
   left out; see [Non-goals](#non-goals).
3. **Writing it never affects a request.** Sign-in and page requests do not wait on the file or
   fail because of it, since the master directory may be on a slow filesystem. The first write
   failure is printed on stderr once, and `bug-report` notes a missing or unreadable log.
4. **Bounded against someone hammering the sign-in form.** An unauthenticated client can cause
   these events, so the size is capped: past 1 MiB the file becomes `web-events.jsonl.1`,
   replacing the previous one. A per-event-kind rate limit with a `suppressed` count on the next
   record keeps a flood from pushing out earlier evidence. The numbers are provisional.
5. **Allow-listed fields only.** Records are built field by field, never from a request or from
   flags as a whole. See [What is never recorded](#what-is-never-recorded).
6. **The executable's identity, not just its version.** The start record and the `bug-report`
   environment both give the executable's SHA-256 and modification time, which works for every
   build and shows whether the server and `bug-report` were the same binary.

## Events

Every record has `time`, `event`, `pid`, and `command` (`web` or `tableview`).

| Event | Written when | Other fields |
|---|---|---|
| `server_start` | The listener is up | `version`, `executable_sha256`, `executable_modified`, `host`, `port`, `auth` (`none`, `flag_or_env`, `generated`), `allow_exec`, `allow_terminal`, `allow_control`, `notifications`, `artifact_root_count`, `registered_basedir_count` |
| `sign_in` | A login `POST` is decided | `outcome` (`accepted`, `wrong_credentials`, `malformed`, `cross_origin`), `transport`, `cookie_secure` when accepted, `user_agent` |
| `sign_in_refused` | The login path answers 403 for its transport | `transport`, `user_agent` |
| `session_missing` | An HTML navigation is sent to the sign-in form | `cookie` (`absent`, `unknown`, `expired`, `transport_refused`), `transport`, `user_agent` |
| `credential_rejected` | A request carries an API token that does not match | `transport` |
| `sign_out` | Signing out succeeds | `transport`, `user_agent` |

`transport` is one of `direct_loopback`, `https`, `forwarded_https`, or `other`, from the checks
`isDirectLoopbackRequest` and `isSecureAuthRequest` already make. A `401` to an API request with
no credentials is not recorded: a page left open polls, and its polling is not evidence.

`server_start` shows a restart, which explains `cookie: unknown` for sessions issued before it.

## What is never recorded

- The token, the password, a session or launch cookie's value, and any `Authorization` or
  `X-Rotari-Token` header.
- The submitted user name. People paste a token into the wrong field, so only `outcome` says
  whether it was wrong.
- URLs, paths, `next`, and query strings.
- `Forwarded` and `X-Forwarded-*` values, and client addresses. `transport` is enough for this
  purpose.
- Flag values other than `host` and `port`: `artifact-root` paths are counted, not listed.

`user_agent` is untrusted text: it is cut to 256 bytes and JSON-encoded, so it cannot forge a
record, and it is treated as text wherever it is shown.

## Implementation

### Step 1: the event log

Add a small package, for example `internal/weblog`, owning the file, the single-write append,
rotation, the rate limit, and the allow-listed record types. `cmd/rotari` opens it, and
`internal/webui` receives a narrow recording interface, so that `webui` does not learn where the
file lives. Add the package to [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

### Step 2: record the outcomes

Give `WithAuthToken` the recorder, replacing its variadic `onSignOut` with an options value now
that it takes two extras. Have `webSessions` report why a session was not accepted. Record the
events above at each decision, for `rotari web` and `rotari tableview`. Behavior, status codes,
and redirects stay as they are.

### Step 3: record the start

Write `server_start` from `web.go` and `tableview.go` once the listener is up. Add one helper in
`cmd/rotari` for the executable's identity, reading it once at start. A `--static-dir` export
serves nothing and records nothing.

### Step 4: collect it

Give `bugreport.Options` the Web log's path, resolved in `cmd/rotari`, and the executable's
identity. Collect the last 1 MiB of the log, reading its rotated file when the current one is short,
through the same path redaction as other files, with a hostname `host` value replaced under
`Redact`. Show the last lines in the summary, and say so when there is no log. List the
registered basedirs and whether each exists, under their placeholders, which closes that part of
the Open item.

Reading the log must keep working when the master directory cannot be resolved: that is a note in
the summary, not a failed report.

### Step 5: promise and document

- Add a WEB contract (the next free number; WEB-34 is the latest at this writing) for what is
  recorded and what never is, with its row in [contracts/README.md](../../contracts/README.md).
- Extend CLI-26 with the new evidence.
- Add the log and its contents to [docs/SECURITY.md](../../docs/SECURITY.md).
- Update the `bug-report` answer in [docs/FAQ.md](../../docs/FAQ.md) with what to add by hand for
  a Web problem, since rotari cannot collect it: browser and version, a private window or not, and
  the browser console and network panel's content.
- Update the Open item in [ISSUES.md](../ISSUES.md) with what remains.

## Non-goals

- **An access log or recording Web API requests.** The Open item keeps this question. Decide it
  separately, once sign-in evidence has shown whether more is needed.
- **Browser-side collection.** A page reporting its console or cookies to the server would be a
  feature of its own, with its own security questions.
- **Terminal events.** The panel's stuck `Connecting...` with many old tabs would have benefited
  from connection and take-over records, and the same log can hold them. They are left for a
  later change.
- **A configurable location or an off switch.** Add them if someone needs them. The file holds no
  secrets and is bounded.

## Tests and validation

- **Secrets stay out (the main test).** Through the binary, sign in with a known token, with the
  token as the user name, a wrong password, a cookie, a bearer token, and a `next` holding a marker.
  Require that none of these values appears in `web-events.jsonl`, or in a bundle with or without
  `--no-redact`. Extend `TestBugReportNeverIncludesSecrets` or add its sibling.
- **Each outcome records its event:** every row of the table, for `web` and for `tableview`,
  with status codes and redirects unchanged. The tests in
  [webui_test.go](../../internal/webui/webui_test.go) cover the unchanged behavior.
- **`session_missing` tells its four cases apart:** absent, unknown after a restart, expired, and
  refused for its transport.
- **Hostile values:** a `User-Agent` with newlines, a forged record, and 1 MiB of text stays one
  record of bounded size.
- **Limits:** a flood stays under the size cap, keeps earlier records through the rate limit, and
  reports `suppressed`. Two servers appending at once leave whole lines.
- **Failure:** an unwritable master directory leaves sign-in working and prints one warning, and
  `bug-report` notes the absent log.
- **`bug-report`:** the bundle works with no project and no run, and names both executables' hashes.
- **Independent check (done last).** Following [ISSUES.md](../ISSUES.md), restore `Secure` on
  loopback in a temporary worktree and sign in with Go's cookie jar, which also withholds a
  `Secure` cookie on plain HTTP, using a Safari `User-Agent`. Give an agent that has not seen this
  plan only the bundle and the symptom, and record what it found and what it still lacked. It
  should now name the discarded cookie from the bundle. A real Safari run would confirm the
  `User-Agent` text, but is not required.

## Open questions for the implementer

- Whether `registered_basedir_count` and the basedir list should reuse `registry.BaseDirs` as it
  is, given the Open item that a removed basedir reads as an empty `default` project.
- Whether the rate limit is worth having given the size cap, once it is seen in code.
