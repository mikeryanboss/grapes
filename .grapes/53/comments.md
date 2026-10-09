### 2026-10-09T09:05
[DECISION] A path without its own copy shows the active copy, not main's: a session whose agent has not edited the issue yet should see its latest state.

### 2026-10-09T09:10
[VERIFY] `gofmt -l .` lists nothing; `go vet ./...` and `go test ./...` pass. With the source switch disabled, `TestRenderIssueShowsTheWorktreeCopy` fails for one worktree, so it guards the behavior. PASS
