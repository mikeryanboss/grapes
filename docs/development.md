# Development Guide

Use this page to find the smallest implementation and test surface for a change.

## Local Checks

The project requires the Go version declared in `go.mod`.

```sh
go test ./...
go vet ./...
go build .
```

Run a narrower package while iterating, then run the full checks before handoff:

```sh
go test ./internal/data
go test ./internal/tui/board
go test ./internal/tui -run TestApp_Refresh
```

Bare `grapes` needs an interactive terminal. Use CLI subcommands in automation.
CLI usage errors (unknown commands, extra arguments, non-positive or malformed
IDs) are rejected before filesystem discovery with exit status 2. `issue`
accepts zero or one positive ID; `validate` accepts zero or more positive IDs.
Help and version also reject trailing arguments.

## Common Change Paths

| Change | Production files | Tests |
| --- | --- | --- |
| Add or change a CLI command | `main.go` | add root-package tests if practical; run all tests |
| Change issue fields or sorting | `internal/data/issue.go` | data tests and affected view tests |
| Change parsing or source discovery | `internal/data/loader.go` | `internal/data/nextid_test.go` or a focused loader test |
| Change ID locking | `internal/data/flock_*.go`, `loader.go` | concurrency and worktree cases in `nextid_test.go` |
| Change validation | `internal/data/validate.go` | add focused data validation tests |
| Change writes/editor format | `internal/data/writer.go` | writer tests plus `app_refresh_test.go` when routed by TUI |
| Add a cross-screen action | `tui/common/messages.go`, `tui/app.go` | `app_refresh_test.go` and relevant interaction test |
| Change one screen | its package under `internal/tui/` | package interaction tests and golden tests |
| Change filter semantics | `internal/tui/filter/` | `filter_test.go`, picker/menu tests, then view tests |
| Change keys | `config.go`, `common/keys.go`, affected view | config and interaction tests |
| Change themes | `config.go`, `common/theme.go` | config, preset, settings, and golden tests |
| Change settings | `internal/tui/settings/settings.go` | settings interaction and golden tests |
| Change what a host program sees | `embedded/embedded.go`, `tui/app.go` | `embedded/embedded_test.go`, `app_embedded_test.go` |
| Change release version | `main.go` | `go test ./...`; confirm plugin metadata if releasing it too |

## TUI Testing

There are three complementary test styles:

- Unit tests cover data, configuration, filters, themes, and small model behavior.
- Interaction tests send Bubble Tea key and mouse messages to models and assert the
  resulting model or emitted command.
- Golden tests render fixed models and compare plain-text output in `testdata/`.

Shared deterministic fixtures and golden helpers live in
`internal/tui/testutil/testutil.go`. To regenerate golden files after an intentional
layout change:

```sh
go test ./internal/tui/... -update
go test ./internal/tui/...
```

Review every changed `.golden` file. Do not use `-update` merely to make an
unexpected rendering failure disappear.

The app-level refresh tests create real temporary issue directories and exercise
write/reload behavior. Add a case there when a UI action persists data, changes an
active source, or must survive reload.

## Adding TUI Behavior

Follow the existing message boundary:

1. Define a shared message in `internal/tui/common/messages.go` if the action crosses
   a package boundary.
2. Emit it from the child view's `Update` method.
3. Handle it in `tui.Model.Update` when it affects global state or the filesystem.
4. Route writes through `internal/data`; do not write issue files from a child view.
5. Reload or update all affected child models so board, list, and detail agree.
6. Add interaction tests for input and app-level tests for persistence/refresh.

Keep coordinate calculations local to views. When an overlay needs mouse hit
testing, the root computes and assigns its screen rectangle after layout.

## Changing the Issue Format

An issue-format change is cross-cutting. Check all of these deliberately:

- persisted `meta` and public `Issue`/`IssueSource` representations;
- `loadIssueMeta`, source conversion, and `Issue.SwitchSource`;
- `SerializeIssue` and `SaveIssueFromText`;
- validation and relationship rewiring;
- board, list, detail, search, filters, and settings that expose the field;
- `plugin/skills/grapes/SKILL.md`, CLI help, and public `README.md`;
- existing `.grapes` compatibility and tests.

Derived reverse fields such as `Children` and `Blocks` should remain in-memory only.

## Configuration and Compatibility

Defaults are applied before TOML unmarshalling, so partial configuration files retain
defaults for omitted fields. A parse error discards the entire parsed copy and falls
back to clean defaults. Preserve both properties when extending `Config`.

Keybindings have three representations that must stay aligned: fields in
`config.KeysConfig`, defaults in `config.Defaults`, and runtime bindings in
`internal/tui/common/keys.go`. Settings adds a fourth representation when the key is
editable in the UI.

`Config.Sources.DefaultBranch` is an optional comparison ref for worktree
attribution and is editable from the Sources settings category. Empty means
automatic repository detection; configured external directory globs remain
separate from Git attribution.

## Releases

The version variable is in `main.go`. A push to `main` that changes that file causes
`.github/workflows/auto-tag.yml` to create `v<version>` if absent. That tag triggers
`.github/workflows/release.yml`, which runs GoReleaser.

Programs that import `embedded` pin a released tag, so a change to that package
reaches them only after a release.

`.goreleaser.yaml` builds static Linux, macOS, and Windows archives for amd64 and
arm64. Linker flags replace `main.version` with the release tag.

The Claude plugin has separate version metadata in `.claude-plugin/marketplace.json`.
Update it intentionally when publishing plugin changes; it is not synchronized with
the binary version automatically.

## Documentation Maintenance

Update `docs/README.md` when files or task-routing advice move. Update
`docs/architecture.md` when ownership, runtime flow, persistence, or invariants change.
Update this guide when commands, tests, or release mechanics change. Prefer stable
symbol names over line numbers so the documents remain useful across small edits.
