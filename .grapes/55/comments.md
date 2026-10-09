### 2026-10-09T09:50
[DECISION] Grapes hands the terminal to `vineyard --issue <id>` (vineyard #18, 0.1.2) instead of running agents: #52 made running agents vineyard's job, and vineyard imports grapes, so grapes cannot call vineyard code. `main.go` finds vineyard on `PATH`; without it, standalone behaves as before.

### 2026-10-09T10:05
[VERIFY] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass, including `TestStandalone_SessionsRunVineyard` (hint shown, a fake vineyard runs as `<repo> --issue 1` on grapes' standard streams, its last stderr line reaches the status bar) and the unchanged `TestStandalone_QuitsAndHidesSessions`. Manual, in a scratch repo with vineyard 0.1.2 on `PATH` and private tmux sockets: the board ends `a sessions · q quit`; `a` opened vineyard's `New session for #55`; creating it and attaching worked inside vineyard inside grapes; quitting vineyard returned to the board; with another vineyard running, `a` showed `Vineyard: another vineyard is already running for this repository`; after it quit, `a` jumped to the #55 session. A build without `c.Stdin, c.Stdout = os.Stdin, os.Stdout` failed to attach (`attach: exit status 1`), which confirms the tmux `/dev/tty` constraint. Screenshots in `.grapes/55/tmp/shots/`. PASS.

### 2026-10-09T10:06
[DONE] `internal/tui/app.go`: `WithVineyard`, `runVineyard`/`vineyardCommand`, standalone `SessionsMsg` handling, and the `a sessions` hint; `common.VineyardFinishedMsg`; `main.go` looks vineyard up. README and `docs/architecture.md` describe it. Version 0.1.13.
