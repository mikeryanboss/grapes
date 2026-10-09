package common

import (
	"charm.land/bubbles/v2/key"
	"github.com/Mibokess/grapes/internal/config"
)

// GlobalKeys are available on every screen.
type GlobalKeys struct {
	Quit     key.Binding
	Settings key.Binding
	// Sessions asks the host program for the agent sessions working on the
	// selected issue. It does something only when Grapes is embedded.
	Sessions key.Binding
}

var GlobalKeyMap = GlobalKeys{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Settings: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "config"),
	),
	Sessions: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "sessions"),
	),
}

// BoardKeys are specific to the Kanban board screen.
type BoardKeys struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Open          key.Binding
	EditIssue     key.Binding
	ToList        key.Binding
	Search        key.Binding
	Clear         key.Binding
	Filter        key.Binding
	CycleStatus   key.Binding
	CyclePriority key.Binding
	Labels        key.Binding
	CycleSort     key.Binding
	ReverseSort   key.Binding
	ToggleEmpty   key.Binding
}

var BoardKeyMap = BoardKeys{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/left", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/right", "right"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	EditIssue: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	ToList: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "list view"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Clear: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear search"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	CycleStatus: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
	CyclePriority: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "priority"),
	),
	Labels: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "labels"),
	),
	CycleSort: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "order"),
	),
	ReverseSort: key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "reverse"),
	),
	ToggleEmpty: key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("E", "empty cols"),
	),
}

// ListKeys are specific to the list screen.
type ListKeys struct {
	Up               key.Binding
	Down             key.Binding
	Open             key.Binding
	EditIssue        key.Binding
	ToBoard          key.Binding
	Filter           key.Binding
	StructuredFilter key.Binding
	Clear            key.Binding
	CycleStatus      key.Binding
	CyclePriority    key.Binding
	Labels           key.Binding
	CycleSort        key.Binding
	ReverseSort      key.Binding
	ScrollLeft       key.Binding
	ScrollRight      key.Binding
}

var ListKeyMap = ListKeys{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "down"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	EditIssue: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	ToBoard: key.NewBinding(
		key.WithKeys("B"),
		key.WithHelp("B", "board view"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	StructuredFilter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	Clear: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear filter"),
	),
	CycleStatus: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
	CyclePriority: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "priority"),
	),
	Labels: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "labels"),
	),
	CycleSort: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "order"),
	),
	ReverseSort: key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "reverse"),
	),
	ScrollLeft: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "scroll left"),
	),
	ScrollRight: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "scroll right"),
	),
}

// DetailKeys are specific to the detail screen.
type DetailKeys struct {
	Back          key.Binding
	ToBoard       key.Binding
	ToList        key.Binding
	CycleStatus   key.Binding
	CyclePriority key.Binding
	Labels        key.Binding
	AddComment    key.Binding
	EditIssue     key.Binding
}

var DetailKeyMap = DetailKeys{
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc/⌫", "back"),
	),
	ToBoard: key.NewBinding(
		key.WithKeys("B"),
		key.WithHelp("B", "board view"),
	),
	ToList: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "list view"),
	),
	CycleStatus: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
	CyclePriority: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "priority"),
	),
	Labels: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "labels"),
	),
	AddComment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment"),
	),
	EditIssue: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
}

// SettingsKeys are available on the settings screen.
type SettingsKeys struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Tab   key.Binding
	Enter key.Binding
	Save  key.Binding
	Back  key.Binding
}

var SettingsKeyMap = SettingsKeys{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/left", "back"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/right", "enter"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch pane"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "edit"),
	),
	Save: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "save"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
}

// The default keymaps, captured before any config is applied.
var (
	defaultGlobalKeys = GlobalKeyMap
	defaultBoardKeys  = BoardKeyMap
	defaultListKeys   = ListKeyMap
	defaultDetailKeys = DetailKeyMap
)

// KeyLabel returns the primary key of a binding, for status-bar hints.
// Hints are built from the live bindings so that rebinding a key in the config
// also changes what the status bar advertises.
func KeyLabel(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// rebind replaces a binding's primary key, keeping its fallback keys and help
// text in sync. An empty configured key leaves the default binding untouched,
// so a half-written config.toml cannot bind an action to nothing.
func rebind(b *key.Binding, configured, help, action string, extraKeys ...string) {
	if configured == "" {
		return
	}
	keys := append([]string{configured}, extraKeys...)
	*b = key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, action))
}

// ApplyKeys updates all keybinding vars from a KeysConfig.
//
// It resets to the defaults first, so applying a config is idempotent: a key
// the user clears returns to its default instead of keeping whatever an earlier
// call installed.
func ApplyKeys(k config.KeysConfig) {
	GlobalKeyMap = defaultGlobalKeys
	BoardKeyMap = defaultBoardKeys
	ListKeyMap = defaultListKeys
	DetailKeyMap = defaultDetailKeys

	rebind(&GlobalKeyMap.Quit, k.Quit, k.Quit, "quit", "ctrl+c")
	rebind(&GlobalKeyMap.Settings, k.Settings, k.Settings, "config")

	rebind(&BoardKeyMap.Up, k.BoardUp, k.BoardUp+"/up", "up", "up")
	rebind(&BoardKeyMap.Down, k.BoardDown, k.BoardDown+"/down", "down", "down")
	rebind(&BoardKeyMap.Left, k.BoardLeft, k.BoardLeft+"/left", "left", "left")
	rebind(&BoardKeyMap.Right, k.BoardRight, k.BoardRight+"/right", "right", "right")
	rebind(&BoardKeyMap.Open, k.BoardOpen, k.BoardOpen, "open")
	rebind(&BoardKeyMap.EditIssue, k.BoardEdit, k.BoardEdit, "edit")
	rebind(&BoardKeyMap.ToList, k.BoardToList, k.BoardToList, "list view")
	rebind(&BoardKeyMap.Search, k.BoardSearch, k.BoardSearch, "search")
	rebind(&BoardKeyMap.Filter, k.BoardFilter, k.BoardFilter, "filter")
	rebind(&BoardKeyMap.CycleStatus, k.BoardStatus, k.BoardStatus, "status")
	rebind(&BoardKeyMap.CyclePriority, k.BoardPriority, k.BoardPriority, "priority")
	rebind(&BoardKeyMap.Labels, k.BoardLabel, k.BoardLabel, "labels")
	rebind(&BoardKeyMap.CycleSort, k.BoardSort, k.BoardSort, "order")
	rebind(&BoardKeyMap.ReverseSort, k.BoardReverse, k.BoardReverse, "reverse")
	rebind(&BoardKeyMap.ToggleEmpty, k.BoardEmpty, k.BoardEmpty, "empty cols")

	rebind(&ListKeyMap.Up, k.ListUp, k.ListUp+"/up", "up", "up")
	rebind(&ListKeyMap.Down, k.ListDown, k.ListDown+"/down", "down", "down")
	rebind(&ListKeyMap.Open, k.ListOpen, k.ListOpen, "open")
	rebind(&ListKeyMap.EditIssue, k.ListEdit, k.ListEdit, "edit")
	rebind(&ListKeyMap.ToBoard, k.ListToBoard, k.ListToBoard, "board view")
	rebind(&ListKeyMap.Filter, k.ListSearch, k.ListSearch, "search")
	rebind(&ListKeyMap.StructuredFilter, k.ListFilter, k.ListFilter, "filter")
	rebind(&ListKeyMap.CycleStatus, k.ListStatus, k.ListStatus, "status")
	rebind(&ListKeyMap.CyclePriority, k.ListPriority, k.ListPriority, "priority")
	rebind(&ListKeyMap.Labels, k.ListLabel, k.ListLabel, "labels")
	rebind(&ListKeyMap.CycleSort, k.ListSort, k.ListSort, "order")
	rebind(&ListKeyMap.ReverseSort, k.ListReverse, k.ListReverse, "reverse")

	rebind(&DetailKeyMap.Back, k.DetailBack, k.DetailBack+"/⌫", "back", "backspace")
	rebind(&DetailKeyMap.ToBoard, k.DetailToBoard, k.DetailToBoard, "board view")
	rebind(&DetailKeyMap.ToList, k.DetailToList, k.DetailToList, "list view")
	rebind(&DetailKeyMap.CycleStatus, k.DetailStatus, k.DetailStatus, "status")
	rebind(&DetailKeyMap.CyclePriority, k.DetailPriority, k.DetailPriority, "priority")
	rebind(&DetailKeyMap.Labels, k.DetailLabel, k.DetailLabel, "labels")
	rebind(&DetailKeyMap.AddComment, k.DetailComment, k.DetailComment, "comment")
	rebind(&DetailKeyMap.EditIssue, k.DetailEdit, k.DetailEdit, "edit")
}
