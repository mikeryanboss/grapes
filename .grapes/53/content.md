## Goal

Let vineyard show a session's grapes issue in a tab of its preview pane, next to the agent's screen and diff. Grapes renders the issue; vineyard only places it.

## Description

`embedded.Model` can draw only the whole grapes screen, at the size of the last `tea.WindowSizeMsg`. A pane needs one issue, at the pane's width, without grapes' header and status bar. Add:

```go
func (m Model) RenderIssue(id int, worktreePath string, width int) (string, bool)
```

It returns the issue as the detail screen draws it. When the worktree at `worktreePath` changed the issue, it shows that worktree's copy; otherwise the copy grapes shows. A session's tab should show the issue as its agent left it, not whichever copy is newest.

## Context

- `internal/tui/detail/detail.go`: `renderIssue` (unexported) builds the detail content; `detail.Model` puts it in a viewport. Planned: exported `detail.Render` wrapping it.
- `internal/tui/app.go`: `Model.Issues`, `Model.Worktrees`, and fields `theme`, `worktreeNames`. Planned: `Model.RenderIssue(id, worktree name, width)`.
- `data.Issue.Sources` holds every copy; `SwitchSource` makes one active. `data.WorktreeInfo` maps a worktree path to its name.
- `embedded.TouchedIssues` already matches worktree paths with `filepath.Clean`.

## Acceptance Criteria

- [x] `RenderIssue(id, path, width)` shows the copy in the worktree at `path` when that worktree changed the issue.
- [x] For a path without its own copy, it shows the copy `Issue` reports.
- [x] An unknown id returns `false`.
- [x] `docs/architecture.md` describes the call.

## Verify

Run from the repository root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

## Pass Criteria

No files listed by `gofmt`; vet and every test pass, including `TestRenderIssueShowsTheWorktreeCopy`.
