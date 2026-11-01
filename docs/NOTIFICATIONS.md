# Notifications

<iframe src="https://www.youtube-nocookie.com/embed/LCco7Tl-z3g" title="Know the moment a run ends" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

Rotari supports local in-page and browser desktop notifications and outbound Webhook
notifications. Both channels are configured in `notifications.toml` and use
the same event and field vocabulary, while keeping independent settings.

The Web UI is the easiest way to change these settings. In `rotari web`, open
**Settings** on a project or run page and choose **Notification config**. It
edits both channels' events, fields, `max_jobs`, and Webhook destination as a
form, and saves to the file that is currently in effect. Selected fields are
listed in the order notifications show them: move one with its up and down
buttons, or choose **Use default order**. A newly checked field is added at the
end, and unselected fields follow in their default order. When no file exists
yet, it offers the global, basedir, and (on a project) project locations to
create one from the template; **Generate notification config** creates or
replaces a template at a chosen location. A saved Webhook URL is never sent
back to the browser: the field only says that one is set, and you can replace
or clear it. Saving and generating need a live server with control enabled
(the default); a server started with `--allow-control=false` and static
exports cannot write.

From the command line, generate the file with `rotari config --notifications`
and edit it directly. Rotari uses the first file it finds in the project, the
basedir, then the global config directory; scopes are not merged.

This is deliberately different from ordinary command configuration, which
merges global → cwd workspace → basedir → project files (see
[Configuration](CONFIGURATION.md#configuration-files)). Notifications have no
workspace scope. The selected file replaces all lower-scope notification
settings: omitted fields use notification built-in defaults, not values from
lower files. In particular, a project notification file with no `webhook.url`
does not inherit a global webhook destination. `ROTARI_WEBHOOK_URL` remains an
explicit environment override. Notification snapshots are recorded separately
from a run's merged command configuration.

## Browser notifications

Each run records its job results and its completion in the master directory
as they happen, whether or not a Web page is open. The live Web UI shows these
records in an in-page inbox, so a run that finished while no page was open is
there when you open one. Records are kept for 10 days. A record that cannot be
written is a warning and does not change the run's result. This is entirely
local and never sends data to an external service.

### In-page inbox

The live Web UI and Recent jobs page have an **Inbox (N)** button. The number
is the unread count; opening the inbox marks its entries read. It shows the
latest 50 entries of the selected basedirs, including those recorded before
the page was opened. Read and cleared entries stay that way after a reload or
a restart of `rotari web`, and in every other page and browser that uses the
same master directory. Each basedir keeps its own read and cleared state, so
opening or clearing the inbox affects only the basedirs it shows: turning on
another basedir's bell later still shows that basedir's unread entries. Use
**Clear notifications** to remove the shown entries, or Escape to close the
inbox. A server started with `--allow-control=false`
shows the inbox but cannot save read or cleared entries; they then last only
until the page is reloaded.

Entries that arrive while a page is open are also announced to screen readers
and, when enabled, shown as desktop notifications. Entries that were already
recorded when the page opened, or when a basedir's bell was turned on, are
not.

This works without the desktop Notification API or its permission, including
in embedded browsers such as VS Code's internal browser and over HTTP.
Desktop notification permissions, the desktop on/off toggle, and
`--notifications=false` do not disable the inbox.

The inbox reads records from the master directory that `rotari web` resolves
(`ROTARI_MASTERDIR`, `$XDG_STATE_HOME/rotari/master`, or
`~/.local/state/rotari/master`). A run whose process resolves another master
directory, such as one started on a host with a different home directory, is
not shown. Runs started by a rotari version without this feature record
nothing. A run whose process was killed records no completion when it stops.
When `rotari unlock` later recovers it, the run is reported then (and sent to
the Webhook): as finished when unlock finalizes it from its complete summary,
or as interrupted (`rotari: run interrupted`) when it can only release it.

### Reading notifications from the command line

`rotari notifications` lists the same notifications as the inbox, oldest
first, with their time, project, run ID, and the inbox text:

```sh
rotari notifications              # every known state directory, last day
rotari notifications demo --since 7d
rotari notifications --unread     # only what the inbox has not marked read
rotari notifications --json
```

Like `rotari runs`, it covers every state directory known to the master
directory unless `--basedir` names one, and `PROJECT` or `--project-name`
narrows it to one project. `--since` (default `1d`) selects how far back to
look; records are kept for 10 days. Listing never marks anything read: the
read state belongs to the Web inbox, and `--unread` and the `(unread)` marks
(the `unread` field in JSON) only report it. A record that cannot be read is
reported as a warning on stderr.

### Enabling browser notifications

Open `rotari web` and click `Enable notifications` in the top bar.
After browser permission is
granted, the button becomes a `Notification on`/`Notification off` toggle.
The toggle is stored in the browser's `localStorage`, scoped by protocol,
host, and port.

Use `--notifications=false` or `ROTARI_WEB_NOTIFICATIONS=false` on `rotari web`
to make the initial toggle state off. Once a browser has changed the toggle,
that explicit choice takes precedence over the server default.

Browsers allow notifications only on HTTPS or a localhost address, so a page
opened as `http://HOST:PORT` from another machine cannot enable them; the
button is then disabled and its tooltip says why. Leave `rotari web` on its
default loopback host, forward its port over SSH, and open the page on
localhost instead:

```bash
ssh -L 8787:localhost:8787 HOST
```

Then open `http://localhost:8787`. Use the port that `rotari web` prints if it
differs.

### Browser events and details

The `[browser]` section controls local notifications:

```toml
[browser]
job_failure = true
job_success = false
run_failure = true
run_success = true
fields = ["project", "job_name", "job_status", "exit_code", "diagnosis_name", "command", "started_at", "run_name", "run_status", "run_id", "attempt_id", "link"]
max_jobs = 10
```

A run counts as a success only when its exit code is 0; a failed or cancelled
run, and an interrupted run that `rotari unlock` releases, is reported under
`run_failure`. A job is reported only after its retries
end, and blocked jobs and jobs cancelled before they start are failures. As
with Webhooks, a run's job results within ten seconds of the
first are combined into one notification, and the run's completion joins the
pending one and records it at once. `max_jobs` limits how many jobs one
notification lists, and the body is truncated at 1000 characters. The
`[browser]` settings are applied when the inbox is shown, so a saved change
also applies to entries already recorded.

The defaults include `run_id`, `job_name`, and `attempt_id` so a notification
identifies its run, job, and exact execution without repeating the job ID.
Fields appear in the order listed, so the defaults lead with the project, job
name, outcome, command, and start time and leave the IDs last. Those fields remain selectable. In
browser notifications, the default `link` opens the exact run page; removing it
disables click navigation.

Each basedir in the sidebar has a bell that selects it for notifications;
only the current basedir starts selected. With several basedirs registered,
the bell beside the **Registered basedirs** heading selects all of them at
once, or clears them when all are selected; it looks mixed while only some
are selected. The selection lasts until `rotari web` restarts. The main Web UI
always includes the basedir it shows; Recent jobs uses only the selected bells.

The inbox is checked every two seconds. Static exports have no server to load
notification settings or records, so they provide neither the editor nor an
inbox. They show a local on/off
toggle on both the main pages and Recent jobs; `--notifications` selects its
initial value. Live-server options
(`--host`, `--port`, `--auth-token`, and `--allow-control`) cannot be combined
with `--static-dir`; use them only when starting the HTTP server.

## Webhook notifications

Rotari can send JSON `POST` requests for job results and run completions.
Generic JSON, Slack, Microsoft Teams, and Discord formats are supported.
Keep the URL out of the file by setting `ROTARI_WEBHOOK_URL`, which overrides
`webhook.url`.

### Webhook events and details

The `[webhook]` section controls outbound notifications:

```toml
[webhook]
url = "https://example.invalid/hook"
format = "json"
web_url = "https://rotari.example.com"
job_failure = true
job_success = false
run_failure = true
run_success = true
fields = ["project", "job_name", "job_status", "exit_code", "diagnosis_name", "command", "started_at", "run_name", "run_status", "success_count", "failure_count", "run_id", "attempt_id"]
max_jobs = 20
```

Set `web_url` to the root URL of the `rotari web` server. Job notifications
include a direct link that opens the run and focuses the matching job;
run-completion notifications open the run. Each link adds the route of the
run's own basedir (`/_basedir/<id>/`), so a global or basedir
`notifications.toml` links runs from any basedir; the server must know that
basedir to open the link. If the server is behind a reverse-proxy path, include
that prefix in `web_url`. The default `webhook.fields` includes `link`; remove
it to omit links, or leave `web_url` empty to disable link generation. In the
Web UI notification settings, **Use current Web URL** fills the field with the
root of the currently open Rotari Web server, without the path of the page or
basedir being viewed.

A job is reported once after its retries end. Blocked jobs and jobs cancelled
before they start are failures. Events within ten seconds of the first pending
event are combined, and run completion flushes the pending batch immediately.
`max_jobs` limits the listed jobs; aggregate success and failure counts still
cover the whole run.

Delivery errors are warnings and do not change the run result. Rotari keeps no
delivery record and does not retry. A run whose process stopped before
reporting its completion is sent when `rotari unlock` recovers it: a
`run.finished` event when unlock finalizes the run from its complete summary,
or a `run.interrupted` event, selected by `run_failure`, when it releases the
run. A `run.interrupted` event carries the run's identity, `run_status`
`interrupted`, and its start time when one was recorded; it has no exit code
or counts.

### Slack

Create a Slack app, enable **Incoming Webhooks**, and use its URL:

```toml
[webhook]
url = "https://hooks.slack.com/services/T.../B.../..."
format = "slack"
web_url = "https://rotari.example.com"
```

The message uses Slack Block Kit and displays the configured fields. When
`web_url` is set, each event includes an **Open in Rotari** link to its run or
the exact job row.

### Microsoft Teams

Create an Incoming Webhook for the target channel:

```toml
[webhook]
url = "https://example.webhook.office.com/..."
format = "teams"
```

The message uses a MessageCard and displays the configured fields.

### Discord

Create a webhook in the target channel:

```toml
[webhook]
url = "https://discord.com/api/webhooks/..."
format = "discord"
```

The message uses embeds and displays the configured fields.

### Testing a Webhook

Keep `run_failure = true` and run a command expected to fail:

```sh
rotari add --project-name demo false && rotari run --project-name demo
```

### Generic JSON

For Make, Zapier, Pipedream, or an internal HTTP service, use the default
`json` format. Each event contains the configured fields available to it:

```json
{
  "schema_version": 1,
  "created_at": "2026-09-30T17:41:02Z",
  "events": [
    {
      "event": "job.finished",
      "fields": [
        {"name": "project", "value": "demo"},
        {"name": "run_id", "value": "20260930-174102-9c8bb0d5"},
        {"name": "job_name", "value": "train"},
        {"name": "job_status", "value": "failed"},
        {"name": "exit_code", "value": 1},
        {"name": "diagnosis_name", "value": "Permission denied"},
        {"name": "diagnosis_suggestion", "value": "Check file ownership and permissions."}
      ]
    },
    {
      "event": "run.finished",
      "fields": [
        {"name": "project", "value": "demo"},
        {"name": "run_id", "value": "20260930-174102-9c8bb0d5"},
        {"name": "run_status", "value": "failed"},
        {"name": "exit_code", "value": 1},
        {"name": "success_count", "value": 0},
        {"name": "failure_count", "value": 1}
      ]
    }
  ]
}
```

## Events and fields

Both channels independently select job failures, job successes, run failures,
and run successes. `fields` selects values in display order; the default
order leads with the project, job name, outcome, command, and start time and
ends with `run_id`, `job_id`, `attempt_id`, and `link`. Fields that do not
apply to an event are omitted, and multiple diagnoses produce repeated
diagnosis fields. The event type, payload schema version, and creation time are
always present in generic JSON.

Browser and Webhook notifications share the following vocabulary. In browser
notifications, `link` enables click navigation to a run. In Webhook
notifications, `link` includes the URL generated from `webhook.web_url` in the
selected output fields.

| Category | Fields |
| --- | --- |
| Identity | `project`, `run_id`, `run_name`, `job_id`, `job_name`, `stage`, `array_task_id`, `attempt_id` |
| Result | `run_status`, `job_status`, `exit_code`, `error`, `success_count`, `failure_count`, `total_count` |
| Time | `started_at`, `finished_at`, `duration` |
| Execution | `executor`, `hosts`, `working_directory`, `command` |
| Diagnosis | `diagnosis_status`, `diagnosis_name`, `diagnosis_evidence`, `diagnosis_suggestion`, `diagnosis_rules`, `diagnosis_outdated` |
| Navigation | `link` |

`working_directory` and `diagnosis_evidence` can contain sensitive or lengthy
values, so they are disabled by default. `command` is enabled by default for
both channels; remove it from `webhook.fields` if command lines must not leave
the host. Environment variables and executor options cannot be selected.
