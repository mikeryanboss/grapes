## Goal

Let a host program read an issue's labels, parent, children, and blockers through `embedded.Issue`, so vineyard can write an agent's first prompt from them (vineyard issue #28).

## Description

Vineyard starts an agent on an issue with a prompt rendered from a template. The template picks its wording from the issue's labels (a `bug` template for bugs), names the parent for context, lists the sub-issues, and warns about open blockers. `embedded.Model.Issue` returns only `ID`, `Title`, and `Status` today, although grapes loads all of these.

## Context

- `embedded/embedded.go`: `type Issue` has `ID`, `Title`, `Status`; `Model.Issue` copies them from `tui.Model.Issues()`.
- `internal/data/issue.go`: `data.Issue` has `Labels []string`, `Parent *int`, `Children []int` (rebuilt by `RewireRelationships` in `loader.go`), and `BlockedBy []int`.
- `embedded/embedded_test.go`: `TestIssue` compares with `!=`, which slices make impossible.
- Programs that import `embedded` pin a released tag (`docs/development.md`, Releases), so `version` in `main.go` goes from 0.1.13 to 0.1.14.

## Acceptance Criteria

- [x] `embedded.Issue` has `Labels`, `Parent` (0 for a top-level issue), `Children`, and `BlockedBy`, copied from the issue grapes shows.
- [x] `TestIssue` covers each new field for a parent with a child that has labels and a blocker.
- [x] `version` is 0.1.14.

## Verify

Run from the worktree root:

```bash
gofmt -l . && go vet ./... && go test ./...
```

## Pass Criteria

`gofmt` prints nothing; vet and tests pass.
