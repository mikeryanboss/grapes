### 2026-10-09T21:25
[DONE] `embedded.Issue` now carries `Labels`, `Parent` (0 when top-level), `Children`, and `BlockedBy`. `Model.Issue` returns copies of the slices, so a host cannot change what grapes shows. `version` is 0.1.14.

### 2026-10-09T21:25
[VERIFY] From the worktree root: `gofmt -l .` printed nothing; `go vet ./...` passed; `go test ./...` passed, including `TestIssue` for a parent and a labelled, blocked child. PASS.
