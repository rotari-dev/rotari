# Plan: Launching from where a shell is

Created: 2026-11-09
Status: Done, not yet committed or tried by hand. See the [work log](work-log.md). The user asked whether the artifacts' launch feature, which opens TensorBoard and other viewers,
could work from the terminal too, from the directory a shell is in. (An earlier reading of "launch from the current
directory" as "start a new shell there" was built and then removed at the user's word.)

## Goal

With `rotari web --allow-terminal --allow-exec`, someone working in a shell can open TensorBoard, Jupyter Lab, or
another registered command on the directory the shell is in, or on something in it, and have it open through the
Web UI's port as a launch from an artifact does, without first finding a job whose artifact it is.

## Existing implementation

- **A launch names a command and an artifact.** `POST /api/launch` ([launches.go](../../internal/webui/launches.go))
  takes a project, run, job, attempt, and an `entry` (with an optional `child` under it), which `resolveArtifact`
  turns into a path inside the allowed roots (the job's working directory, the basedir's parent, `--artifact-root`).
  The command comes from `launchers.toml` by name; `Accepts` and `Matches` check the target's kind and name. A
  service opens under `/_launch/ID/`; **Launches** in the top bar lists and stops launches.
- **The roots limit what can be chosen, not what a command does** (SECURITY.md): commands run as the user.
- **The terminal knows where each shell is:** `Keeper.Dir(id)` (the shell overview's preview).
- **Shells get no Web token,** so a command typed in a shell cannot call the API as the page does.

## Proposals

1. **The target is named by the shell, not by a path.** A launch request may name a running shell instead of an
   artifact: `{"name": "tensorboard", "shell": "3", "child": "runs/exp1"}`. The server takes the shell's directory
   now (`Keeper.Dir`), joins `child` (checked as artifact children are: no `..`, no absolute path), and checks the
   command accepts it, as for an artifact. The labels name the shell's project, and the launch appears in
   **Launches** as any other. This adds nothing a shell cannot already do, since the shell runs as the same user
   and could start the program itself; it needs both `--allow-terminal` and `--allow-exec`.
2. **Where to start it:** a **Launch** icon in the terminal bar, for the shell of the pane with the keys, opening the
   same dialog the artifacts use: the commands that accept that directory, and a list of what is in it, each entry
   with the commands that accept it, as the artifact **Files** browser shows. Choosing one shows what will run, then
   **Launch**, as now. A card in All shells could have the same button.
3. **The listing** of the shell's directory reuses the artifact listing's shape (`/api/artifact-files` or its
   equivalent), served for a shell's directory only while that shell runs, and only names and kinds, not contents.
4. **Not a command in the shell** (`rotari launch tensorboard .`): the shell has no token by design, and giving it one
   would hand the Web API to anything run in it.

## Decided

On 2026-11-09 the user chose both places, the terminal bar and All shells' cards, and only the shell's directory
itself: no listing of what is in it, so proposal 3 is dropped and a launch on a shell takes no `child`.

## Steps

1. `POST /api/launch` and `/api/launch-preview` with `shell`, with unit and conformance tests, and WEB-38.
2. The bar's and the cards' Launch icon and a dialog of the terminal's own (the artifact dialog lives in the
   application page's scripts, which the Recent jobs page and the terminal's window lack); docs.

## Risks

- **The directory can change** between the check and the launch, as a path can for artifacts today; the launch
  records the path it used.
- **The artifact launch dialog** is written for an attempt's listing; making it take a shell's may need its code
  split from the attempt's.
