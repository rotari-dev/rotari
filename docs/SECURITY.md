# Security model

Rotari assumes a trusted single-user or HPC/lab environment. The Web UI token
provides HTTP authentication, not encryption.

## State-file permissions

`--basedir/-b`, `--masterdir`, and their contents (queues, metadata, locks,
job output and status, wrapper scripts) default to shared `0755`/`0644`
permissions, because colleagues on the same cluster commonly share a job's
log path directly.

Set `ROTARI_PRIVATE_STATE=true` for owner-only `0700`/`0600` permissions to
keep job commands, working directories, and output private. This affects only
newly created paths, which are not re-chmodded later, and applies to the whole
`--basedir/-b`.

## Supervisor

A run's supervisor opens no socket or port. Only the `run` command that
started it talks to it over inherited pipes. This works where Unix sockets are
blocked and with base directories of any length.

While it runs, the supervisor holds
`<basedir>/projects/<project>/server.lock` and records its PID in `server.pid`
next to it. A failed start is recorded in `<basedir>/server.log`.

## Web UI authentication and transport

Without `ROTARI_WEB_AUTH_TOKEN` or `--auth-token`, bind the Web UI to
`127.0.0.1`. With a token, browser login and session cookies work over plain
HTTP only on a direct loopback connection: a browser on the same host opening
`http://localhost` or `http://127.0.0.1`, with no proxy headers. Everywhere
else, use HTTPS through a trusted reverse proxy; browser login is refused over
other plain HTTP. API token credentials are also unencrypted over HTTP, so use them only on a
trusted network or over HTTPS. Prefer the environment variable so the token
does not appear in the process list.

Environment variable values are never exposed over HTTP. Project and run
pages do show raw global/workspace/basedir/project config sources and saved
merged run file-config snapshots, which may contain secrets. Static exports
include those contents as well; share them only with trusted readers.

The notification settings UI visually masks a saved webhook URL and does not
return it from its read API. This does not encrypt the connection: a
replacement URL is sent to the Web server when saved. Use HTTPS through a
trusted reverse proxy whenever the browser-to-server network is not trusted.

### Why the session cookie has no `Secure` on direct localhost

The session cookie carries `Secure` over HTTPS and through an HTTPS proxy, but not on a
plain HTTP connection directly to `localhost`. This is deliberate, and is often mistaken
for a weakness, so here is the reasoning.

`Secure` is an instruction to the browser: use this cookie only over HTTPS. It exists to keep
a cookie from being read in transit, and traffic to `localhost` does not leave the machine.
It says nothing about who may connect. Rotari itself decides that on every request: a browser
session is accepted only over HTTPS or a direct loopback connection, whatever the cookie says.

Browsers differ here. Chrome and Firefox treat `localhost` as secure and accept a `Secure`
cookie over plain HTTP. Safari does not: it discards the cookie, so correct credentials return
to the sign-in form with no error, and we know of no Safari setting that changes this. Omitting `Secure` on
direct loopback makes Safari work, and leaves those browsers' behavior as it already was.

What this does not change:

- Another machine opening the server over plain HTTP is still refused.
- Another user of a shared host who connects to the server's loopback port still needs the
  token.

What to keep in mind: a browser's cookies do not distinguish ports, so a cookie for `localhost`
also goes to other services the same browser opens on `localhost`, as it already does in
Chrome and Firefox. With SSH forwarding, `localhost` means the machine running the browser, not
the server, so other users of the server are out of reach unless you forward a port of theirs
and open it. Do not open a service you do not trust on the same host name, or use HTTPS.

### Web event history

Live `rotari web` and `rotari tableview` servers append startup and
authentication outcomes to `<masterdir>/web-events.jsonl`. The file is mode
`0600`, rotates at 1 MiB, and is collected by `rotari bug-report`. It records
the server executable's identity and listener settings, and enough about a
sign-in decision to distinguish an accepted cookie from an absent, unknown,
expired, or transport-refused session.

The history never contains the Web token or submitted password, a session or
launch cookie value, the submitted user name, URLs or query strings, proxy
header values, client addresses, or artifact-root paths. It does include a
user agent cut to 256 bytes, the listener host and port, process ID, executable
SHA-256 and modification time, and counts of registered basedirs and artifact
roots. Anyone who can read the master directory can read this operational
metadata. Static exports write no event history.

## Script on Web UI pages

Live Web UI and Table viewer pages send a `Content-Security-Policy` that runs only
their own scripts and limits requests to the same origin, as a second line of
defense if a stored value ever became markup. It does not cover a static export,
which has no response headers, or a launched service, which serves its own pages
on the same origin.

## Artifact files and static exports

`rotari run` reads the configuration files, shell scripts, and Python files a
job names, as the user running it, to find the paths the job uses. It records
only those paths in each attempt's `artifacts.json`, never the files'
contents.

The live Web UI serves the contents of recorded files and directories under
each job's working directory and under the directory holding its basedir.
Anyone who can open the Web UI can read them; with job control enabled, they
can also add a job that names another file there. The Web server reads files
as the user running it, whoever views the page.

Whoever can write a project's state decides what its jobs record. Therefore,
for a basedir another user can write (group-writable, or owned by someone
else), neither the working directory nor the basedir's parent is served; only
`--artifact-root` is served. Paths outside the allowed roots, and symlinks
leading out of them, are not served unless `rotari web --artifact-root DIR`
allows them. Keep such roots narrow. SVG files are shown only as images, so
their scripts do not run.

A static export contains only paths unless it is made with
`--static-artifact-contents`, which copies files into the export. Anyone who
can read the export, such as a published site, can then read those files. The
command reports how many files and bytes it copied.

`rotari tableview --static-dir` likewise copies the table's rows and the
image, audio, video, and PDF files its cells name into the snapshot, and
reports how many it copied.

## Launching services and commands

Launching is off by default. `rotari web --allow-exec` lets anyone who can
sign in run the built-in services (TensorBoard, Jupyter Lab, MLflow, Aim,
Streamlit, Datasette, marimo, Panel, Gradio, VS Code) and commands registered in your global `launchers.toml`, as you,
against the job artifacts the Web UI may serve. See
[Launching commands](WEB.md#launching-commands).

The browser and API only name a registered command and an artifact, or, with
`--allow-terminal` as well, a running shell, whose directory now the server looks
up; that runs nothing the shell could not run itself, as the same user. Command
text, working directories, and modes come only from `launchers.toml`, which
rotari refuses unless no other user can write or replace it. The listed
artifact roots limit what can be selected, not what a command does: a command
has all your file permissions, and a path can change between the check and
the command opening it.

With launching enabled, `rotari web` always requires a token, even on
`127.0.0.1`, since other users of a shared host can reach loopback ports. It
accepts only `localhost`, `127.0.0.1`, `[::1]`, a specific `--host`, and
`--allowed-host` names as the `Host` header, against DNS rebinding. It rejects
a POST from another origin, or one without `Origin` unless it authenticates
with `X-Rotari-Token` or a bearer token, which a browser never adds by itself.
Launched commands get neither the Web token nor stdin.

A launched service is served from the Web UI's own origin under `/_launch/ID/`,
behind the same token. Requests reach it without the `Authorization` or
`X-Rotari-Token` header. Its pages, however, run with the Web UI's origin and
can call the rotari API as the signed-in browser. Treat a registered service
as trusted code, like the command itself.

The proxy connects only to the launch's own `127.0.0.1` port. Before each new
connection, it checks that the listening process belongs to the launch
(through `/proc` on Linux, `/usr/sbin/lsof` on macOS).

A launched service also listens on that loopback port, which other users of
the host can reach directly without rotari's token. A service that can require
a secret should use the launch's `ROTARI_LAUNCHER_TOKEN`, which the proxy
presents through `proxy_headers`, as is or as HTTP Basic credentials, and can
read it from the private `ROTARI_LAUNCHER_TOKEN_FILE`. The built-in Jupyter Lab, marimo, and VS Code do use the token, so
what runs code serves only through the proxy. VS Code also puts the token in a
browser cookie that its page reads, which other pages from the same rotari
address may be able to read. TensorBoard, MLflow, Aim, Streamlit,
Datasette, Panel, and Gradio cannot, so another user who finds their port can read what they show.
No built-in passes the token in a process's arguments, which other users can
read.
WebSocket handshakes, used by services such as Jupyter and Streamlit, need
the same `Origin` as other mutating requests.

Launched-command output stays in a private `0700` directory under `$TMPDIR`
until the Web server stops. A later `rotari web --allow-exec` removes
directories that a killed server left behind. `rotari web --allow-exec` does
not start when another user could replace that directory: `$TMPDIR` and every
directory above it, following symlinks, must be owned by you or root and not
writable by group or others, unless sticky like `/tmp`.

## The Web terminal

The terminal is off by default. `rotari web --allow-terminal` gives anyone who can sign in a shell
as you, with all your file permissions, so a leaked token is worth more with it on. Like launching,
it always requires a token and checks `Host` and `Origin`. Its connection also requires HTTPS or
a direct loopback connection, even with an API token, which the rest of the API accepts over plain
HTTP. The shell's environment leaves out the Web token.

Who can turn it on matters as much. `allow-terminal`, `allow-exec`, and `allowed-host` are read from
the command line, the environment, the global config, and a file named with `--config`, never from a
workspace `.rotari.toml`, a basedir config, or a project config, which other people or a cloned
repository may have written. With the terminal or launchers on, the same goes for `host` and
`auth-token`, so such a file cannot expose the server on another address or behind a token someone
else knows. `rotari web` warns of each value it ignores. Keep the global config writable only by you.
The Web UI's Settings can save the global config, but never its `auth-token`, so whoever can reach
that page cannot choose the token that the terminal and launchers require.

Signing in is the entrance: there is no second prompt. A page's script could reach the shell if a
stored value ever became script, which [the page policy](#script-on-web-ui-pages) exists to prevent,
and a launched service's own pages, which that policy does not cover, can reach it just as they can
call the API. Treat them as trusted code either way.

The terminal panel's Explorer lists any directory the page names and lets the Table viewer open any file, by
path, under the same token, `Host` check, and HTTPS-or-loopback rule as the shell. That reads nothing a shell
on the same server could not, so it widens nothing; without `--allow-terminal`, neither is offered. The
Explorer's changes to files (making, renaming, moving, copying, deleting) are the same: `POST` requests with a JSON
body, which a page on another site cannot send without a preflight the server never grants, behind the same
checks, and nothing a shell could not do. Downloads and uploads are too: a download is a `GET` that a page on another
site can start but not read, and an upload needs an `application/octet-stream` body, which a form cannot send.

**Recent directories** (`GET /_terminal/shells/ID/dirs`) answers, behind the same checks, the directories a
project's shells have been in and where its runs started. The server keeps those directories in memory only, never
on disk, and forgets them when every shell ends; it records nothing that is typed.

Links in a shell's output (`POST /_terminal/shells/ID/links`) are found the same way: the page sends words from the
screen, at most 64 at once, behind the same checks and with a JSON body, and the server answers which name a run, a
job, or a path that exists. That tells nothing a shell on the server could not.

So that bash and zsh print those marks, rotari starts them with its own startup files (`--terminal-shell-integration`,
on by default), written when the server starts into a directory only you can read and removed when it stops. They
read your usual startup files and add prompt hooks that print the marks; they change none of your files, and run
nothing a shell would not. `--terminal-shell-integration=false` starts bash and zsh as plain login shells.

Blocks are read in the page alone, from the OSC 133 marks in the output it already shows; the server parses and
keeps nothing for them, so it still records nothing that is typed. zsh's `C` mark carries the command line,
percent-encoded, which passes through the server as all output does and is kept in the replayed output as its echo
already is. Any program can print the marks, and a file
shown with `cat` can draw false ones. They change only the page's display: **Copy output** and **Copy command**
write to the clipboard only when chosen, and **Type command again** types the text shown, never with Enter and only
while the shell is at its prompt, so a false mark can at worst put text in front of you.

Shells outlive their connections, like a launched VS Code server, so ones left open stay reachable
to anyone with your session for as long as the server runs, or, with `--terminal-idle-timeout`, until that
long after the last page closes, and recent
output is replayed to them. A page left open keeps them for as long as its sign-in lasts, up to 12
hours. Signing out ends them all.

A server started with `rotari web --detach` outlives your login, and with the terminal on it stays a way
into the host, behind its token, until you stop it. Running `rotari web --detach` again shows whether one
is running; stop one you no longer use with `rotari web --shutdown`. The token it generates reaches the server in its
environment, not its command line, and its record under the master directory holds only a digest of it.

Anyone who may control jobs can also stop the server: **Stop server** in Settings sends `POST /_server/shutdown`,
which needs job control, and the token when the server has one. It also needs a JSON content type, which a page on
another site cannot send without a preflight the server never grants.

## Limits

This model does not defend against another user with access to your own UID
(for example, root or anyone who can read your home directory). It protects
only against other unprivileged users on a shared machine.
