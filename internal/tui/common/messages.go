package common

import (
	"github.com/Mibokess/grapes/internal/config"
	"github.com/Mibokess/grapes/internal/data"
)

// Screen identifies which view is active.
type Screen int

const (
	ScreenBoard Screen = iota
	ScreenList
	ScreenDetail
	ScreenSettings
)

// Messages for screen routing (sent by views, handled by app).
type OpenDetailMsg struct{ ID int }
type GoBackMsg struct{}
type SwitchScreenMsg struct{ Screen Screen }
type RefreshMsg struct{}

// WorkspaceLoadedMsg carries the result of a reload. Loading happens in a
// command rather than in Update, so a workspace with many worktrees cannot
// freeze the event loop while it is read.
type WorkspaceLoadedMsg struct {
	Workspace data.Workspace
	Err       error
}

// Messages for write operations.
type ShowPickerMsg struct {
	IssueID int
	Field   string // "status" or "priority"
}
type PickerResultMsg struct {
	IssueID int
	Field   string
	Value   string
}
type PickerCancelMsg struct{}
type ShowLabelPickerMsg struct{ IssueID int }
type LabelPickerResultMsg struct {
	IssueID int
	Labels  []string
}
type LabelPickerCancelMsg struct{}
type LaunchEditorMsg struct{ ID int }
type EditorFinishedMsg struct{ Err error }
type LaunchEditMsg struct{ ID int }
type EditFinishedMsg struct{ Err error }
type WriteErrMsg struct{ Err error }

// CloseMsg tells the program embedding Grapes that the user pressed quit. An
// embedded Grapes must not return tea.Quit, which would end the host program.
type CloseMsg struct{}

// SessionsMsg asks for the sessions working on an issue: from the program
// embedding Grapes, or, standalone, from vineyard.
type SessionsMsg struct{ IssueID int }

// VineyardFinishedMsg reports that vineyard, started for SessionsMsg, exited.
type VineyardFinishedMsg struct{ Err error }

// WatchErrMsg reports a file-watcher failure: live reload is degraded.
type WatchErrMsg struct{ Err error }
type CycleSortMsg struct{}
type ReverseSortMsg struct{}
type ToggleEmptyColumnsMsg struct{}
type ColumnSortMsg struct{ Mode data.SortMode }
type MoveIssueMsg struct {
	IssueID   int
	NewStatus data.Status
}

// Filter overlay messages.
type ShowFilterMenuMsg struct{}
type FilterMenuSelectMsg struct{ Field string }
type FilterPickerResultMsg struct {
	Field    string
	Selected []string
}
type FilterToggleChildrenMsg struct{}
type FilterToggleTopLevelMsg struct{}
type FilterCancelMsg struct{}
type ClearAllFiltersMsg struct{}

// Multi-source worktree messages.
type SwitchSourceMsg struct {
	IssueID   int
	SourceIdx int
}

// Settings screen messages.
type ConfigSavedMsg struct{ Config config.Config }
