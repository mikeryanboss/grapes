## Goal

Let a user start, or jump to, a vineyard agent session for an issue from standalone grapes, as they already can from grapes embedded in vineyard. Both programs then lead to each other.

## Description

Embedded, the sessions key (`a`) sends `SessionsMsg` and vineyard answers it. Standalone, grapes ignores the message and hides the key (#52), so a user browsing issues in plain `grapes` has no way to put an agent on one.

Grapes still runs no agents itself; #52 removed its tmux sessions because that is vineyard's job. Instead, standalone grapes hands the terminal to `vineyard --issue <id>` (vineyard issue #18, vineyard 0.1.2), which opens vineyard at the issue: its session, a picker among several, or the new-session dialog filled in for the issue. Quitting vineyard returns to grapes.

## Context

- `internal/tui/app.go`: `case common.SessionsMsg` returns `nil`; `hostHints` shows `a sessions` only when `m.embedded`.
- `main.go` builds the standalone model with `tui.Load` and runs it on `tea.OpenTTY` descriptors.
- The editor flow already hands the terminal over with `tea.ExecProcess`; Bubble Tea keeps a command's preset `Stdin`, `Stdout`, and `Stderr`.
- Vineyard attaches to sessions with tmux, which refuses a descriptor opened through `/dev/tty` ("can't use /dev/tty"). So vineyard must get grapes' standard streams, not the `OpenTTY` files; vineyard opens the TTY itself when they are not terminals.
- Vineyard runs from the directory holding the `.grapes` directory; it finds the main checkout from any worktree.
- A vineyard already running for the repository holds its lock; `vineyard --issue` then exits 1 with `Error: another vineyard is already running for this repository` on stderr. Grapes shows that line in its status bar.
- Planned: `Model.vineyard`, the path of the vineyard binary, set by `main.go` with `WithVineyard` from `exec.LookPath("vineyard")`. Without vineyard on `PATH`, standalone behaves as today.

## Acceptance Criteria

- [x] Standalone with vineyard on `PATH`, `a` on the board, list, or detail screen runs `vineyard --issue <id>` from the `.grapes` directory's parent, on grapes' standard streams, and grapes resumes when it exits.
- [x] When vineyard fails, the status bar shows the last line it wrote to stderr, or the exit error when it wrote nothing.
- [x] Standalone with vineyard on `PATH`, the status bar advertises `a sessions`; without it, the key is ignored and not advertised.
- [x] Embedded behavior is unchanged.
- [x] README and docs describe the key in standalone grapes; `version` is bumped.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

Manual: in a tmux pane, in a scratch repository with an issue and vineyard 0.1.2 on `PATH`, run grapes, press `a`, capture vineyard's new-session dialog, quit vineyard, capture grapes again. Repeat with a vineyard already running for the repository and capture the status bar.

## Pass Criteria

`gofmt` prints nothing; vet and tests pass. The captures show vineyard's `New session for #<id>`, grapes back on its board after quitting, and `another vineyard is already running for this repository` in grapes' status bar.
