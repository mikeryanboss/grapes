## Goal

Let another Bubble Tea program, vineyard, run the grapes TUI as a screen of its own, and let a user jump from an issue to the vineyard agent sessions working on it. Remove grapes' own tmux sessions: running agents is vineyard's job.

## Description

Vineyard (github.com/mikeryanboss/vineyard) runs coding agents in tmux sessions, one git worktree each. It will import a new public package, `github.com/Mibokess/grapes/embedded`, show the grapes model full screen, and forward messages to it. Everything in `internal/` stays where it is; Go lets a package inside this module import `internal/...`, and the new package exposes only what a host needs.

Embedding needs three behavior changes, active only when the model is embedded:

- The quit key emits `CloseMsg` instead of `tea.Quit`, which would end the host program.
- A new "sessions" key (`a`) on the board, list, and detail screens emits `SessionsMsg{IssueID}` for the selected issue. Commands from an embedded model run in the host program, so the host receives the message directly. Standalone grapes ignores it and does not advertise the key.
- The status bar advertises `a sessions`.

## Context

Verified against `main` at `7098583` (v0.1.10):

- The tmux feature came in as one commit, `7e93819`. Nothing changed its TUI files afterwards. Its pieces are `internal/tmux/`, the `tmux*` fields, messages, and commands in `internal/tui/app.go`, the session rows and `StartSession` key in `internal/tui/detail/detail.go` and `internal/tui/common/keys.go`, the tmux messages in `internal/tui/common/messages.go`, its detail interaction tests, and lines in `docs/architecture.md` and `docs/development.md`. The commit's changes to `internal/data/workspace_test.go` and `worktree_test.go` (`commitAt`) are unrelated test helpers and stay.
- `main.go:89-108` builds the TUI: `config.Load`, `data.NewWorkspaceLoader`, `loader.Load`, `tui.NewModel`, then `WithStatus` for config errors and load problems. The embedded package needs the same steps, so they move into one planned function, `tui.Load(issuesDir, version string) (tui.Model, error)`, which `main.go` and the embedded package both call.
- The quit key returns `tea.Quit` at `internal/tui/app.go:617-621`.
- `tui.Model.View` returns a `tea.View`; a host needs its `Content` string.
- `data.WorktreeInfo` (`internal/data/workspace.go:26`) lists, per worktree path, the issues its branch changed (`Touched`). The host maps its sessions to issues with it.

Planned API of `github.com/Mibokess/grapes/embedded`:

```go
func New(issuesDir string) (Model, error) // issuesDir is a .grapes directory; the version comes from build info
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string
func (m Model) OpenIssue(id int) (Model, tea.Cmd) // show the issue's detail screen
func (m Model) Issue(id int) (Issue, bool)        // ID, title, status
func (m Model) TouchedIssues(worktreePath string) []int
type CloseMsg = common.CloseMsg
type SessionsMsg = common.SessionsMsg // IssueID int
```

Constraints: no new configuration; the sessions key is not rebindable. The package name avoids `embed`, which would shadow the standard library's `embed` in readers' minds.

## Acceptance Criteria

- [x] `internal/tmux/` is deleted and no Go file or doc mentions tmux sessions.
- [x] The `a` key on the issue detail screen no longer starts a tmux session.
- [x] `tui.Load` builds the model for both `main.go` and the embedded package.
- [x] Embedded, the quit key emits `CloseMsg` and never `tea.Quit`; standalone, it still quits.
- [x] Embedded, `a` on the board, list, and detail screens emits `SessionsMsg` with the selected issue's ID, and the status bar shows `a sessions`; standalone, the hint is absent.
- [x] `OpenIssue`, `Issue`, and `TouchedIssues` behave as documented, with tests.
- [x] `docs/README.md`, `docs/architecture.md`, and `docs/development.md` describe the embedded package and no longer describe tmux.
- [x] The version in `main.go` is bumped so that merging releases a new tag.

## Verify

Run from the repository root:

```bash
gofmt -l . && go vet ./... && go test ./...
grep -rn -i tmux --include=*.go . ; grep -rn -i tmux docs
```

Manual: run `grapes` in this repository, open an issue, press `a`: nothing happens and no `a` hint shows. Press `q`: grapes exits.

## Pass Criteria

`gofmt -l` prints nothing, vet and tests pass, and both greps print nothing. The manual check behaves as described. Embedded behavior is covered by tests in `embedded/`.
