# Work log: no silent fallbacks

## Plan and decisions

Commit: `1b6b6efa` (2026-11-11 15:48:42 +0900)

- **Change:** Added [plan.md](plan.md). It covers three open ISSUES.md items:
  bad config and environment values are ignored, a removed basedir reads as an
  empty `default` project, and a standing `auth-token` blocks static exports.
- **Reason:** This is the first of three stabilization plans agreed with the
  user on 2026-11-11, after a review of the open issues. The other two are
  shared-rule violations and test-suite flakiness.
- **Plan impact:** The user settled D3, D4, D6, and D7, each with the
  recommended option. A bad configured value is an error even when the command
  line overrides it. An empty environment variable means unset for typed
  options. A missing basedir is reported as missing by read-only commands and
  the Web UI. A static export ignores `auth-token` from config and the
  environment.
- **Validation:** Documentation only; no tests run.
- **Remaining:** Phases 1 to 5.

## Phase 1: survey and reproduce

Commit: recorded with this entry (plan and work log only).

- **Change:** Updated the findings in [plan.md](plan.md) with reproduced
  behavior, the list of project-listing implementations, and the owning
  package for the shared listing (`internal/state`). Phases 2 and 3 were
  refined to match.
- **Reason:** Phase 1 of the plan: confirm each case with the binary before
  changing behavior.
- **Plan impact:**
  - Bool, int, and duration values from config and the environment are
    dropped or truncated silently, as ISSUES.md said.
  - Choices and repeated values are not silent: the command rejects them
    later, but the error names `--option` and not the variable or file. A
    table given for a repeated option is dropped silently.
  - `configString` turns a list or table into its `fmt.Sprint` text.
  - No reader outside the helpers parses leniently; notification settings are
    already strict.
  - A missing basedir and an empty one give identical output from `check`,
    `projects`, `jobs`, `runs`, `notifications`, `wait`, `show`, `export`, and
    `lineage`. The cause is ten separate readers of `<basedir>/projects`.
- **Validation:** Built `rotari dev (1b6b6efa77e3)` with go1.25.8 into the
  session scratchpad. Ran it with `env -i`, an isolated `HOME`,
  `XDG_CONFIG_HOME`, and basedir, using `--help` defaults and real `add` /
  `run` / `config` / `export` invocations for each helper kind. Then ran the
  read-only commands above against a nonexistent and an empty basedir. The
  `/api/projects` behavior was not rerun; it is taken from the ISSUES.md entry
  of 2026-11-04.
- **Remaining:** Phases 2 to 5.
