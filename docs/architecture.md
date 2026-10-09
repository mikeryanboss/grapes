# Architecture

This document describes stable responsibilities and data flow. It is a navigation
aid, not a substitute for checking the implementation named in each section.

## Process Entry Points

`main.go` is the only executable entry point.

1. Help, version, unknown commands, and invalid command arguments are handled
   before filesystem discovery.
2. `data.FindIssuesDir` walks toward parent directories first, Git-style, so running
   grapes from a subdirectory finds the project's store. It falls back to searching
   descendants up to ten levels deep.
3. If no `.grapes` directory exists, interactive startup offers to create one.
4. `grapes issue` delegates ID allocation and timestamps to `internal/data`.
5. `grapes validate` delegates schema and relationship checks to
   `internal/data/validate.go`.
6. Bare `grapes` loads config, loads all issue sources, constructs `tui.Model`, and
   runs the Bubble Tea program.

## On-Disk Model

`internal/data/issue.go` defines the public in-memory model. `meta`, the private TOML
shape in `internal/data/loader.go`, defines the persisted metadata.

Persisted fields are title, status, priority, labels, parent, blocked-by IDs, and
timestamps. Description and comments live in separate Markdown files. `Children`
and `Blocks` are reverse relationships rebuilt in memory by `RewireRelationships`.

Valid statuses, in board order, are `backlog`, `todo`, `in_progress`, `done`, and
`cancelled`. Valid priorities are `urgent`, `high`, `medium`, and `low`.

## Loading and Worktree Ownership

`.grapes` is tracked in Git, so every worktree checkout holds a complete copy of
every issue. Those copies are not versions anyone chose to make, and treating them
as such is what made loading cost `O(worktrees × issues)` and put a source badge on
nearly every card. Git decides which copies mean anything.

The primary load path is `data.WorkspaceLoader.Load`, in `internal/data/workspace.go`:

1. `resolveLayout` finds the canonical `.grapes` through Git's common directory, so
   launching from a worktree gives the same view as launching from the main checkout.
2. `Checkouts` lists the main checkout and every worktree.
3. `GatherClaims` asks each worktree what it changed *relative to its merge-base with
   the default branch*, concurrently. This is the whole design: a worktree that has
   merely fallen behind differs from main for a reason that carries no information,
   while its diff against its own base is exactly the work it has done.
4. `LoadAllIssues` reads the main checkout once. Worktrees contribute only the issues
   Git reports them as having changed, so idle worktrees cost no file reads.
5. `mergeExternal` expands optional `[sources].dirs` globs for stores outside this
   repository, which get no attribution and are loaded whole.
6. `resolveOwners` picks the winning copy: the most recent real change, with the main
   checkout winning ties. `Issue.SwitchSource` copies that source's fields up.
7. Relationships are rebuilt from the owning versions, and results are sorted by ID.

If the main checkout's attribution claim fails, loading records the error and
falls back to the un-attributed baseline rather than combining partial ownership
claims with missing main-checkout timestamps.

A "real change" is a commit date, or the mtime of an *uncommitted* edit. It is never
the mtime of a checked-out file: Git stamps those when it writes the working copy, so
a freshly created worktree would otherwise own every issue in the project.

`WorkspaceLoader` caches per-worktree claims on `(HEAD, default branch tip)` and the
repository layout for the process lifetime, so a reload only re-runs `git status`.
Reuse one loader; do not construct one per refresh.

Loading deliberately tolerates some failures: malformed issue directories and
unreadable secondary sources can be skipped. Being outside a Git repository is not a
failure — it is recorded in `Workspace.AttributionErr` and surfaced where a user would
look for worktrees, not reported as a problem on every load. Validation is the strict
path.

## ID Allocation

`data.NextID` finds the main repository through Git's common directory, locks the
canonical issue store's `.lock`, scans the main source plus all known
worktree/configured sources, and creates `max ID + 1` in the local source.
Unlike a normal load it scans every copy rather than only the changed ones: an ID
already used on some branch must never be handed out twice. It runs under the
lock and off the render path, so the full scan is affordable there.

Nested issue stores and relative invocation paths are resolved against the
repository root before worktree discovery, so every checkout is scanned at the
same relative store path.
Platform-specific locking lives in `flock_unix.go` and `flock_windows.go`.

Keep the scan and directory creation inside the lock. Moving either operation out
of the critical section reintroduces collisions between parallel agents.

## Persistence

All production write helpers live in `internal/data/writer.go`:

- `UpdateField` changes status or priority and stamps timestamps.
- `UpdateLabels` rewrites parsed metadata and updates `updated`.
- `StampTimestamps` is the canonical timestamp operation used by the CLI.
- `AppendComment` appends a timestamped Markdown section.
- `SerializeIssue` creates the combined document passed to an external editor.
- `SaveIssueFromText` validates the edited frontmatter, then separates it back into
  metadata, description, and comments.

Metadata replacements use the shared atomic-write helper and preserve the
existing file mode. Per-issue writes also take a persistent advisory lock, so
separate grapes processes cannot overwrite each other's read-modify-write
updates. Editor saves validate the complete metadata before any component file is
replaced; a failed validation leaves the issue unchanged.

Multi-file editor and comment writes stage all replacements before renaming and
roll back replacements when a later rename or directory sync fails.
Comments are append-only and receive UTC timestamps.

The root TUI resolves an issue's owning source before calling these helpers, via
`issueSourceDir`. Preserve that behavior: writes follow ownership, which means the
main checkout unless a worktree is genuinely ahead of it. Electing the write target by
file mtime, as an earlier version did, sends edits into a stale worktree copy where
they are lost when that branch is deleted.

## TUI Ownership and Message Flow

`internal/tui/app.go` contains the root `tui.Model`. It owns:

- the complete issue slice and active screen;
- board, list, detail, and settings child models;
- navigation history, global sorting, and filters;
- picker and filter overlays;
- configuration and theme state;
- filesystem watching, external-editor sessions, writes, and status messages.

Screens live in separate packages and implement local navigation and rendering.
They communicate upward with the types in `internal/tui/common/messages.go`.

```text
keyboard/mouse/fs event
  -> active child model
  -> common message (open, move, edit, filter, switch, save)
  -> root tui.Model.Update
  -> data write or app-state transition
  -> common.RefreshMsg / filesystem event
  -> loadWorkspaceCmd (a tea.Cmd, never Update)
  -> common.WorkspaceLoadedMsg
  -> refreshed child models
```

Cross-view behavior belongs in the root model. Screen-specific selection, layout,
and rendering belong in the screen package. Shared message types, key maps, and theme
primitives belong in `internal/tui/common`.

## Embedding

The public package `embedded` lets another Bubble Tea program, such as vineyard, run
the TUI as one of its screens. `embedded.New` calls `tui.Load`, the same startup path
as `main.go`, and marks the model embedded with `Model.Embedded`. Everything else
stays in `internal/`: a package inside this module may import it, and the
`embedded` API exposes only what a host needs.

An embedded model changes three things:

- The quit key emits `common.CloseMsg` instead of `tea.Quit`, which would end the
  host. The file watcher keeps running, because the host may show Grapes again.
- The sessions key (`a`) on the board, list, and detail screens emits
  `common.SessionsMsg` with the selected issue's ID. A host's agent sessions are the
  only sessions Grapes knows about; standalone, the root model ignores the message.
- The status bar offers `a sessions` and `q back` instead of `q quit`.

A host also reads issues without showing the screen. `RenderIssue` returns an
issue as the detail screen draws it, at a width the host chooses, for a pane of the
host's own. Given a worktree path, it shows that worktree's copy of the issue, so a
host showing an agent's session shows the issue as that agent left it.

Commands from an embedded model run in the host program, so their messages, including
the file watcher's and the periodic reload's, arrive in the host's `Update`. The host
must forward every message it does not handle, even while Grapes is hidden.

Loading runs in a command, not in `Update`. Keep it there: reading the workspace
synchronously froze the event loop for the whole load, which is what made grapes
unresponsive once a project had many worktrees.

The filesystem watcher follows `Workspace.WatchDirs` and `Workspace.WatchRoots`.
`WatchDirs` contains the canonical store, configured external stores, and the
numeric issue directories of worktrees that are actually working on something.
`WatchRoots` contains one non-recursive checkout-root watch per repository
worktree, so idle worktrees can become active without an issue-directory watcher
for every copy. A successful load seeds the activity baseline, so the first poll
compares against the startup snapshot instead of discarding edits made during
startup. The poll runs the cheap attribution check and triggers a full reload only
when checkout activity changes; it also expands or prunes the bounded set of
directory watches. Watching every worktree
recursively would mean thousands of descriptors following copies that never
change. A failed canonical watch is surfaced in the status bar rather than
silently disabling live reload.

## Configuration

`internal/config/config.go` defines the TOML schema and defaults. Configuration is
loaded from `.grapes/config.toml` and covers:

- startup screen, sort mode, child auto-close, and empty board columns;
- extra source directory globs, and the default branch worktrees are compared
  against (`[sources].default_branch`, empty means detect);
- dark/light theme mode, preset, and color overrides;
- global and screen-specific keybindings.

A missing file returns defaults. A malformed file returns clean defaults plus an
error; the TUI displays that error in its status bar. Settings writes use
`config.Save`, then send `common.ConfigSavedMsg` so the root can apply the result.
Changes to source globs or the default branch also schedule a workspace reload so
new sources and attribution rules take effect without restarting the TUI.

## Package Dependency Direction

```text
main
  -> config, data, tui
tui/app
  -> config, data, tui child packages, tui/common
tui child packages
  -> data, tui/common
data and config
  -> standard library and format/system dependencies
```

Avoid importing the root `tui` package from child views. Use a type or message in
`tui/common` to keep coordination flat and prevent import cycles.
