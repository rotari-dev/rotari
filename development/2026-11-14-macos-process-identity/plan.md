# Plan: Verify recorded processes on macOS

Created: 2026-11-14
Status: Implemented; see [work-log.md](work-log.md). Not yet confirmed on macOS. Found by [the macOS CI leg](../2026-11-14-ci-macos/plan.md); see the `development/ISSUES.md` entry
"On macOS, rotari cannot verify where or whether a recorded process runs".

## Goal

On macOS, rotari tells a live recorded process (a run's coordinator, a local job's process group, a client, a launch
origin) from a gone or reused one as it does on Linux, so that `unlock` of an interrupted run needs no
`--assume-stopped`, and liveness, job control, and the binary note work.

## The problem

Every recorded process carries a host, a PID namespace, a PID, and a start time, and `state.ObserveProcess` /
`ObserveProcessGroup` decide from them once (contracts/04-coordination-and-safety.md, under COORD-1 to COORD-5). Both
of the platform facts come from `/proc`:

- `state.PIDNamespace()` reads `/proc/self/ns/pid`; empty on macOS.
- `state.ProcessStart(pid)` reads field 22 of `/proc/PID/stat`; empty on macOS.

So on macOS:

- `ProcessIdentity.Locate` never reports a verified location, since it needs a namespace on both sides. Callers that
  require one stop there: `project.coordinatorLocation` ("coordinator PID namespace cannot be verified") and
  `executor`'s execution probe ("local PID namespace does not match or cannot be verified").
- `ClassifyProcess` returns `ProcessUnverifiable` for any live PID, since a start time is missing on both sides.
  Only `ProcessGone` (ESRCH) is decided.

Seen in Go CI run 43575930113 on macOS: `unlock` refuses with `coordinator_unknown` through the CLI, MCP, and Web;
`info` counts every local job as unknown, even a gone one; `wait` prints no binary note. Not yet seen but using the
same verdicts: `cancel`, `suspend`, and `resume` of local jobs, client attachments, and launch origins.

## Proposals

1. **A PID namespace value for platforms without PID namespaces.** On a platform that has none (macOS and the
   BSDs), `state.PIDNamespace()` returns a fixed, non-empty value, `none`, rather than empty. Every process on such a
   host shares one PID space, so two records saying `none` on the same host are in the same space, and `Locate`
   verifies them as it verifies two equal Linux namespaces. No change to `Locate` itself.
   - A Linux reader sees `none` as another namespace than its own, which is right: such a record comes from another
     kind of host, which the host check catches first anyway.
   - Records written on macOS before this change keep an empty namespace and stay unverified, as records from an
     older rotari are today: `unlock` of such a run still needs `--assume-stopped` once.
   - Rejected alternative: treat an empty recorded namespace as verified on macOS. It would make old records and
     records that failed to read their namespace look verified, and it changes what an empty field means.
2. **Process start time on macOS** from `sysctl kern.proc.pid.<pid>` (`golang.org/x/sys/unix.SysctlKinfoProc`,
   field `kinfo_proc.kp_proc.p_starttime`), formatted as `seconds.microseconds`. It is compared only with values
   written on the same host, so its format need not match Linux's clock ticks. The BSDs other than macOS keep the
   empty value: their `kinfo_proc` differs, and no CI runs them.
3. **No change to the verdict rules.** `ObserveProcess`, `ClassifyProcess`, the coordinator policy (SAFE-13), and the
   execution evidence rules (SAFE-16) stay as they are; they now receive the facts they need on macOS.

## Out of scope

- The run lock's staleness check (`state.CheckLock`) uses `ProcessAlive(lock.PID)` without the start time on every
  platform; a PID reused after a crash can keep a run looking active. Separate, and not macOS-specific.
- The SSH executor's remote scripts read `/proc` on the remote host; a macOS remote host is not supported today.
- The load average (`/proc/loadavg`), absent on macOS.

## Interfaces and documents

- **CLI, MCP, Web, Python client:** no new options or fields. On macOS, `info --json` and `unlock --dry-run` report
  `pid_namespace: "none"` and a start time where they now report empty values, and verdicts change from unknown to
  live or stopped.
- **Persistent state:** `running.lock`, the run context's coordinator, attempt `job.json`, attachment sessions, and
  launch origins record `none` and the start time on macOS. Old records still load.
- **Contracts:** the COORD notes say what the PID namespace and start time are on a platform without `/proc`, and
  that `none` means a platform without PID namespaces. No change to SAFE-13 or SAFE-16 text beyond that.
- **Docs:** OPERATIONS.md (coordinator and local-attempt identity), FAQ entries naming PID namespaces where they
  imply Linux only.

## Steps

1. `internal/state`: per-platform `pidNamespace` and `processStart` files (Linux: `/proc`; darwin: `none` and
   `sysctl`; others: empty start time, `none` for the BSDs). Unit tests that run on both CI legs: the namespace is
   non-empty; this process's start time is non-empty and stable; a child started later has a different one; a
   gone PID has none.
2. `conformance/support.ProcessIdentity`: off Linux, use `state.PIDNamespace` and `state.ProcessStart`, as
   conformance already imports `internal` packages, instead of the empty values it returns now.
3. Run the macOS CI leg: the `unlock`, `info`, and binary-note failures above should pass. Any that do not are
   triaged under the CI plan.
4. Contracts, OPERATIONS.md, FAQ, ISSUES entry resolved, work log.

## Risks

- A wrong macOS start time (for example, one that changes while the process runs) would make a live coordinator
  read as reused, and `unlock` would then finalize a run that is still going. Step 1's stability test guards this,
  and it runs on macOS in CI only; nothing here can run it.
- `none` is a new value that older rotari binaries read as a namespace name. An older binary on the same macOS host
  then reads a new record as "same namespace" only if it also wrote `none`, which it never does: it sees an empty
  namespace of its own and stays unverified, as it is today.
