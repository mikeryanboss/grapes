# Grapes

An issue tracker built for AI agents. Issues are plain files — no APIs, no databases. A terminal UI gives humans a live view of what's happening.

![Grapes TUI demo](doc/demo.gif)

## How It Works

Issues live in a `.grapes/` folder as numbered directories. Each issue is three files:

```
.grapes/42/
  meta.toml       # title, status, priority, labels
  content.md      # description
  comments.md     # append-only log
```

Agents change issue status by editing a line in a TOML file. No client libraries, no authentication, no learning curve.

The TUI watches the filesystem and updates in real time — when an agent moves an issue to `in_progress`, you see the card slide across the board immediately.

With [vineyard](https://github.com/mikeryanboss/vineyard) installed, `a` on an issue opens vineyard there: it jumps to the agent session working on the issue, or starts one for it. Quitting vineyard returns to Grapes.

## Install

Download a binary from [GitHub Releases](https://github.com/Mibokess/grapes/releases), or install with Go:

```sh
go install github.com/Mibokess/grapes@latest
```

## Claude Code

Grapes ships with a [Claude Code](https://docs.anthropic.com/en/docs/claude-code) plugin that teaches agents the issue format so they can create, update, and manage issues out of the box.

```sh
/plugin marketplace add Mibokess/grapes
/plugin install grapes@grapes
```

**Worktree support** — The TUI shows which worktree is working on which issue, so you can see agent progress across parallel work streams.

Because `.grapes/` is tracked in git, every worktree checkout holds a full copy of every issue. Grapes therefore does not treat those copies as separate versions. It asks git what each branch has actually changed since it branched off, and shows only that:

- A worktree appears in the source filter only once it has changed an issue. Idle worktrees — however many you have — stay out of the way.
- An issue is marked on the board only when a worktree has a **newer** version than the main checkout, which is what "someone is working on this over there" means.
- A worktree that has merely fallen behind main is not "different". The comparison is against the point the branch started from, not against main as it is now.
- Edits are written to whichever checkout owns the issue — the main one, unless a worktree is ahead of it.

Attribution needs a branch to compare against. Grapes uses the remote's default branch, falling back to `main` and then `master`. Override it if that guesses wrong:

```toml
[sources]
default_branch = "develop"
```

Outside a git repository grapes works as usual on the local `.grapes/`, with worktree features simply absent.

## Skills

The `.agents/` directory contains workflow skills that can be copied into any project:

- `/issue` — plan, verify, research, or continue work on an issue
- `/handoff` — write a handoff plan so another agent can pick up work
- `/pr` — push and create a pull request


## CLI

| Command              | Description                                          |
| -------------------- | ---------------------------------------------------- |
| `grapes`             | Launch the TUI                                       |
| `grapes issue`       | Allocate next ID, create directory, stamp timestamps |
| `grapes issue <id>`  | Stamp timestamps on an existing issue                |
| `grapes validate`    | Validate all issues                                  |
| `grapes validate ID` | Validate specific issues                             |
| `grapes help`        | Show usage help (`--help`, `-h`)                     |
| `grapes version`     | Show the version (`--version`, `-v`)                 |

`grapes issue` scans the main project and every worktree, using file locking to prevent ID collisions. Unlike the TUI, it looks at all copies rather than only the ones git reports as changed: an ID already used on some branch must never be handed out twice.