### 2026-10-09T00:10
[STARTED] Reverting the TUI parts of `7e93819`, then adding `tui.Load`, the embedded mode, and the `embedded` package. Requested from vineyard, which will host grapes as a screen (vineyard issues to follow).

### 2026-10-09T00:55
[DECISION] The sessions key lives in `GlobalKeys` and is matched by the board, list, and detail views, since only they know the selected issue. It is not rebindable: `ApplyKeys` resets to defaults, so the new field keeps `a`. `embedded.OpenIssue` reuses `common.OpenDetailMsg`, so back returns to the previous screen.

### 2026-10-09T00:56
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass, including `embedded/embedded_test.go` (quit sends CloseMsg, `a` on board and detail sends SessionsMsg, OpenIssue, Issue, TouchedIssues with an uncommitted worktree change) and `internal/tui/app_embedded_test.go` (standalone quits and hides the hint). `grep -rn -i tmux --include=*.go . ; grep -rn -i tmux docs` print nothing. Manual, standalone binary in a 140x35 tmux pane: enter opens #52, the status bar ends `C config · q quit` with no sessions hint, `a` changes nothing, `q` exits. PASS.

### 2026-10-09T00:57
[DONE] Removed `internal/tmux/` and its TUI wiring (reverting the TUI files of `7e93819`). Added `tui.Load`, `Model.Embedded`, the sessions key, `common.CloseMsg`/`SessionsMsg`, and the public `embedded` package. Docs describe embedding; version bumped to 0.1.11 so merging releases it.

### 2026-10-09T01:40
[VERIFY] Running embedded inside vineyard showed the header as `grapes vv0.1.11-…`: the header adds "v" and the module version carries one. `embedded` now strips it; the vineyard run after the fix shows `grapes v0.1.11-…`. PASS.
