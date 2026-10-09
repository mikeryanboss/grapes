# Grapes Project Guide

This is the short orientation page for contributors and coding agents. Read it
before exploring the repository; follow its links only as far as the task requires.

## What Grapes Is

Grapes is a Go terminal application for tracking issues as files. Humans work in a
live Bubble Tea interface; agents work with ordinary TOML and Markdown files. There
is no server, API, or database.

An issue is a numeric directory:

```text
.grapes/<id>/
  meta.toml       title, status, priority, labels, relationships, timestamps
  content.md      Markdown description
  comments.md     append-only Markdown comments
```

The application combines issues from the main checkout, Git worktrees, and configured
external source directories. Because `.grapes` is tracked, every worktree holds a copy
of every issue. A worktree is attributed only when Git reports that its branch changed
the issue relative to its branch base; idle copies do not become source versions. Where
a branch and the main checkout genuinely disagree, the UI presents one issue with
switchable source versions. External sources are loaded without Git worktree
attribution.

## Runtime Mental Model

```text
main.go
  -> locate .grapes and load config
  -> load and merge issues from every source
  -> start the root TUI model and filesystem watcher
  -> route view messages through internal/tui/app.go
  -> persist changes through internal/data
  -> reload all sources after file changes
```

The CLI subcommands (`issue`, `validate`, `help`, and `version`) also enter through
`main.go`, but do not start the TUI.

## Where to Look

| Task | Start here | Then read |
| --- | --- | --- |
| CLI behavior or startup | `main.go` | `internal/data/loader.go`, `internal/config/config.go` |
| Issue schema or relationships | `internal/data/issue.go` | `internal/data/loader.go`, `internal/data/validate.go` |
| File loading | `internal/data/loader.go` | `internal/data/flock_*.go`, `internal/data/nextid_test.go` |
| Worktree attribution or ownership | `internal/data/workspace.go` | `internal/data/worktree.go`, `internal/data/workspace_test.go` |
| File writes or editor round-trip | `internal/data/writer.go` | write cases in `internal/tui/app.go` |
| App-wide navigation or refresh | `internal/tui/app.go` | `internal/tui/common/messages.go` |
| Board behavior | `internal/tui/board/board.go` | board interaction and golden tests |
| List behavior | `internal/tui/list/list.go` | list interaction and golden tests |
| Issue details | `internal/tui/detail/detail.go` | detail interaction and golden tests |
| Filters | `internal/tui/filter/` | filter unit tests |
| Settings, keys, or themes | `internal/tui/settings/`, `internal/tui/common/` | `internal/config/config.go` |
| Agent workflows | `plugin/skills/grapes/`, `.agents/skills/` | the selected skill only |
| Hosting Grapes in another program | `embedded/embedded.go` | the Embedding section of `architecture.md` |
| Release behavior | `.github/workflows/`, `.goreleaser.yaml` | version declaration in `main.go` |

## Repository Map

```text
main.go                  CLI dispatch and TUI bootstrap
embedded/                public API for running the TUI inside another program
internal/config/         config schema, defaults, load, save
internal/data/           issue model, discovery, loading, writing, validation
internal/tui/app.go      root Bubble Tea model and cross-view coordination
internal/tui/board/      Kanban screen
internal/tui/list/       table screen
internal/tui/detail/     Markdown issue screen
internal/tui/filter/     structured filter state and overlays
internal/tui/settings/   interactive configuration screen
internal/tui/common/     messages, key bindings, themes, shared types
plugin/skills/           distributable Grapes agent instructions
.agents/skills/          repository workflow skills
.grapes/                 this repository's own tracked issues
doc/                     README screenshots, demo recordings, and VHS scripts
docs/                    contributor and agent documentation
```

## Invariants Worth Knowing

- Issue IDs are repository-wide across known worktrees and configured sources.
- `created` and `updated` timestamps are managed by Grapes; agents should run
  `grapes issue <id>` after editing issue files.
- TUI writes must target the issue's active `SourceDir`, not always the main
  `.grapes` directory.
- `Children` and `Blocks` are derived reverse relationships. Do not persist them.
- The root TUI model owns cross-screen state, writes, overlays, source switching,
  and reloads. Child views emit messages instead of writing files directly.
- Normal loading may skip malformed or unreadable issue sources; use
  `grapes validate` when correctness matters. Validation is the strict path and
  accepts either all issues or explicit positive IDs.

## Verify a Change

```sh
go test ./...
go vet ./...
```

For the detailed runtime design, read [architecture.md](architecture.md). For common
change paths and testing guidance, read [development.md](development.md).
