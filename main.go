package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/Mibokess/grapes/internal/config"
	"github.com/Mibokess/grapes/internal/data"
	"github.com/Mibokess/grapes/internal/tui"
)

var version = "0.1.11"

func main() {
	// Handle help/version, validate command arguments, and reject unknown
	// commands before touching the filesystem, so they work anywhere without
	// triggering the .grapes/ creation prompt below. issue/validate are
	// dispatched further down.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "--help", "-h":
			if len(os.Args) != 2 {
				fmt.Fprintf(os.Stderr, "%s does not accept arguments\n\n", os.Args[1])
				writeHelp(os.Stderr)
				os.Exit(2)
			}
			writeHelp(os.Stdout)
			os.Exit(0)
		case "version", "--version", "-v":
			if len(os.Args) != 2 {
				fmt.Fprintf(os.Stderr, "%s does not accept arguments\n\n", os.Args[1])
				writeHelp(os.Stderr)
				os.Exit(2)
			}
			fmt.Println(version)
			os.Exit(0)
		case "issue", "validate":
			if err := validateCommandArgs(os.Args[1], os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid arguments for %s: %v\n\n", os.Args[1], err)
				writeHelp(os.Stderr)
				os.Exit(2)
			}
			// handled after issuesDir is resolved below
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
			writeHelp(os.Stderr)
			os.Exit(2)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	issuesDir, err := data.FindIssuesDir(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No .grapes/ directory found.\n")
		fmt.Fprintf(os.Stderr, "Create one in %s? [y/N] ", cwd)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			os.Exit(1)
		}
		issuesDir = filepath.Join(cwd, ".grapes")
		if err := os.MkdirAll(issuesDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating .grapes/: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Created %s\n", issuesDir)
	}

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "validate":
			os.Exit(runValidate(issuesDir, os.Args[2:]))
		case "issue":
			os.Exit(runIssue(issuesDir, os.Args[2:]))
		}
	}

	model, err := tui.Load(issuesDir, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %v\n", err)
		os.Exit(1)
	}
	if err := runTUI(model); err != nil {
		writeProgramError(os.Stderr, err)
		os.Exit(1)
	}
}

var errInteractiveTerminalUnavailable = errors.New("interactive terminal unavailable")

const interactiveTerminalMessage = "Error: bare `grapes` tried to launch the interactive TUI, but no interactive terminal is available."

func runTUI(model tea.Model) error {
	ttyIn, ttyOut, err := tea.OpenTTY()
	if err != nil {
		closeTTY(ttyIn, ttyOut)
		return fmt.Errorf("%w: %v", errInteractiveTerminalUnavailable, err)
	}
	defer closeTTY(ttyIn, ttyOut)

	_, err = tea.NewProgram(model, tea.WithInput(ttyIn), tea.WithOutput(ttyOut)).Run()
	return err
}

func closeTTY(ttyIn, ttyOut *os.File) {
	if ttyIn != nil {
		_ = ttyIn.Close()
	}
	if ttyOut != nil && ttyOut != ttyIn {
		_ = ttyOut.Close()
	}
}

func writeProgramError(w io.Writer, err error) {
	if errors.Is(err, errInteractiveTerminalUnavailable) {
		fmt.Fprintln(w, interactiveTerminalMessage)
		fmt.Fprintln(w)
		writeHelp(w)
		return
	}
	fmt.Fprintf(w, "Error: %v\n", err)
}

func validateCommandArgs(command string, args []string) error {
	switch command {
	case "issue":
		if len(args) > 1 {
			return fmt.Errorf("issue accepts at most one ID")
		}
		if len(args) == 1 {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid issue ID %q", args[0])
			}
		}
	case "validate":
		for _, arg := range args {
			id, err := strconv.Atoi(arg)
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid issue ID %q", arg)
			}
		}
	}
	return nil
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `grapes — file-based issue tracker (TUI + CLI)

USAGE:
  grapes                    Launch the interactive TUI (needs a real terminal)
  grapes <command> [args]

COMMANDS:
  issue                     Allocate the next ID, create .grapes/<id>/, print the ID
  issue <id>                Bump timestamps on issue <id> (one positive ID only)
  validate [<id>...]        Validate all issues, or only the given positive IDs
  help, --help, -h          Show this help (no trailing arguments)
  version, --version, -v    Show the version (no trailing arguments)

ISSUE FILES (.grapes/<id>/):
  meta.toml                 title, status, priority, labels, dates
  content.md                issue description (markdown)
  comments.md               append-only comment log

  status:   backlog, todo, in_progress, done, cancelled
  priority: urgent, high, medium, low

TYPICAL AGENT WORKFLOW:
  id=$(grapes issue)        # create .grapes/$id/, capture the new ID
  # edit .grapes/$id/meta.toml and content.md
  grapes issue $id          # bump 'updated' after editing
  grapes validate $id       # check the issue is well-formed

NOTE:
  Bare 'grapes' opens a full-screen TUI and requires an interactive terminal.
  Automated/agent environments should use the subcommands above instead.
`)
}

func runIssue(issuesDir string, args []string) int {
	if err := validateCommandArgs("issue", args); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid arguments for issue: %v\n", err)
		return 2
	}
	cfg, err := config.Load(issuesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error (using defaults): %v\n", err)
	}
	if len(args) == 0 {
		// No ID: allocate next ID, stamp timestamps, print ID
		id, err := data.NextID(issuesDir, cfg.Sources.Dirs...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := data.StampTimestamps(issuesDir, id); err != nil {
			fmt.Fprintf(os.Stderr, "Error stamping timestamps: %v\n", err)
			return 1
		}
		fmt.Println(id)
		return 0
	}

	// With ID: stamp timestamps on existing issue.
	id, err := strconv.Atoi(args[0])
	if err != nil || id <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid issue ID: %s\n", args[0])
		return 2
	}

	// Create the directory if it doesn't exist. The path comes from the parsed
	// ID, not the raw argument, so "007" cannot create a directory beside #7.
	issueDir := filepath.Join(issuesDir, strconv.Itoa(id))
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		return 1
	}

	if err := data.StampTimestamps(issuesDir, id); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runValidate(issuesDir string, args []string) int {
	if err := validateCommandArgs("validate", args); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid arguments for validate: %v\n", err)
		return 2
	}

	var errs []data.ValidationError
	if len(args) > 0 {
		// Validate specific issue(s)
		for _, arg := range args {
			id, _ := strconv.Atoi(arg) // validateCommandArgs checked this
			errs = append(errs, data.ValidateIssue(issuesDir, id)...)
		}
	} else {
		// Validate all issues
		errs = data.ValidateAll(issuesDir)
	}

	if len(errs) == 0 {
		fmt.Println("All issues valid.")
		return 0
	}

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  %s\n", e.Error())
	}
	fmt.Fprintf(os.Stderr, "\n%d problem(s) found.\n", len(errs))
	return 1
}
