## Goal

Publish `embedded.Model.RenderIssue` (#53) as release v0.1.12, so vineyard pins a release instead of a commit.

## Description

#53 was squash-merged as `929e379`. Vineyard's `main` pins the pre-squash branch commit `e8b69f3`, which is not on grapes `main`. Bumping `main.go`'s version to 0.1.12 makes `.github/workflows/auto-tag.yml` tag `v0.1.12` on merge, and the tag triggers `release.yml`. Vineyard then pins `v0.1.12` in its own PR.

## Context

- `main.go:17`: `var version = "0.1.11"`; no other file carries the binary version.
- Latest tag: `v0.1.11` (`22a0c5b`). `main` is at `929e379`, which adds `RenderIssue`.

## Acceptance Criteria

- [x] `main.go` declares version 0.1.12.
- [ ] After merge, tag `v0.1.12` exists on the merge commit.

## Verify

Run from the repository root:

```bash
go build ./... && go test ./...
git ls-remote --tags origin v0.1.12   # after merge
```

## Pass Criteria

Build and tests pass; after merge, `ls-remote` prints one ref for `v0.1.12`.
