// Package embedded runs the Grapes TUI inside another Bubble Tea program.
//
// The host program creates a Model, calls Init with its own, forwards every
// message it does not handle to Update, and draws View. Messages that Grapes'
// commands produce arrive in the host's Update like any other, so the host must
// keep forwarding them even while Grapes is not shown, or Grapes stops
// refreshing.
//
// Two messages are meant for the host and never need forwarding: CloseMsg,
// sent when the user presses quit, and SessionsMsg, sent when the user asks
// for the sessions working on an issue.
package embedded

import (
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Mibokess/grapes/internal/tui"
	"github.com/Mibokess/grapes/internal/tui/common"
)

// CloseMsg reports that the user pressed quit. The host should stop showing
// Grapes; Grapes keeps watching its files.
type CloseMsg = common.CloseMsg

// SessionsMsg asks the host for the sessions working on issue IssueID.
type SessionsMsg = common.SessionsMsg

// Model is the Grapes TUI, set up to run inside a host program.
type Model struct {
	tui tui.Model
}

// Issue is what a host needs to know about one issue.
type Issue struct {
	ID     int
	Title  string
	Status string
}

// New loads the issues in issuesDir, a .grapes directory, together with the
// copies in the repository's other git worktrees.
func New(issuesDir string) (Model, error) {
	m, err := tui.Load(issuesDir, version())
	if err != nil {
		return Model{}, err
	}
	return Model{tui: m.Embedded()}, nil
}

// version returns the Grapes module version the host was built with, without
// the module version's "v": the header adds its own, as for release builds.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/Mibokess/grapes" {
			if dep.Replace != nil {
				dep = dep.Replace
			}
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return "devel"
}

// Init starts Grapes' file watcher and periodic reload.
func (m Model) Init() tea.Cmd { return m.tui.Init() }

// Update handles one message.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.tui.Update(msg)
	m.tui = next.(tui.Model)
	return m, cmd
}

// View renders Grapes at the size of the last tea.WindowSizeMsg.
func (m Model) View() string { return m.tui.View().Content }

// OpenIssue shows the detail screen of issue id. Back returns to the screen
// shown before. An unknown id leaves the screen unchanged.
func (m Model) OpenIssue(id int) (Model, tea.Cmd) {
	return m.Update(common.OpenDetailMsg{ID: id})
}

// Issue returns issue id, as Grapes currently shows it.
func (m Model) Issue(id int) (Issue, bool) {
	for _, iss := range m.tui.Issues() {
		if iss.ID == id {
			return Issue{ID: iss.ID, Title: iss.Title, Status: string(iss.Status)}, true
		}
	}
	return Issue{}, false
}

// TouchedIssues returns the IDs of the issues that the branch checked out at
// worktreePath changed since it branched off, in ascending order. A path that
// is not a git worktree of the repository, or whose branch changed no issue,
// has none.
func (m Model) TouchedIssues(worktreePath string) []int {
	path := filepath.Clean(worktreePath)
	for _, wt := range m.tui.Worktrees() {
		if filepath.Clean(wt.Path) == path {
			return wt.Touched
		}
	}
	return nil
}
