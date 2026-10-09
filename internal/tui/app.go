package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sort"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Mibokess/grapes/internal/config"
	"github.com/Mibokess/grapes/internal/data"
	"github.com/Mibokess/grapes/internal/tui/board"
	"github.com/Mibokess/grapes/internal/tui/common"
	"github.com/Mibokess/grapes/internal/tui/detail"
	"github.com/Mibokess/grapes/internal/tui/filter"
	"github.com/Mibokess/grapes/internal/tui/labelpicker"
	"github.com/Mibokess/grapes/internal/tui/list"
	"github.com/Mibokess/grapes/internal/tui/picker"
	"github.com/Mibokess/grapes/internal/tui/settings"
	"github.com/charmbracelet/x/ansi"
	"github.com/fsnotify/fsnotify"
)

// clearStatusMsg is sent after a delay to clear transient status bar messages.
type clearStatusMsg struct{}
type workspacePollMsg struct {
	Changed bool
	Err     error
}

const workspacePollInterval = 5 * time.Second

func editorCommand(editor, path string) (*exec.Cmd, error) {
	parts, err := splitCommandLine(editor)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 || parts[0] == "" {
		return nil, errors.New("EDITOR is empty")
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...), nil
}

func splitCommandLine(s string) ([]string, error) {
	if runtime.GOOS == "windows" {
		return splitWindowsCommandLine(s)
	}
	return splitPosixCommandLine(s)
}

func splitPosixCommandLine(s string) ([]string, error) {
	var out []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in EDITOR")
	}
	flush()
	return out, nil
}

// splitWindowsCommandLine preserves backslashes because they are path
// separators on Windows. Quoting still groups paths with spaces; escaped
// quotes are not needed for the usual EDITOR executable-plus-arguments form.
func splitWindowsCommandLine(s string) ([]string, error) {
	var out []string
	var current strings.Builder
	var quote rune
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in EDITOR")
	}
	flush()
	return out, nil
}

// navEntry captures one frame in the navigation history.
type navEntry struct {
	screen common.Screen
	detail detail.Model // only meaningful when screen == ScreenDetail
}

type Model struct {
	version   string
	issues    []data.Issue
	issuesDir string
	width     int
	height    int
	screen    common.Screen
	navStack  []navEntry
	watcher   *fsnotify.Watcher
	sortMode  data.SortMode
	sortAsc   bool // ascending order (reversed from default)
	theme     common.Theme
	isDark    bool

	cfg      config.Config
	filters  filter.FilterSet
	board    board.Model
	list     list.Model
	detail   detail.Model
	settings settings.Model

	filterPicker   *filter.MultiPicker   // non-nil when a filter value overlay is active
	filterMenu     *filter.Menu          // non-nil when the filter category menu is active
	picker         *picker.Model         // non-nil when picker overlay is active
	labelPicker    *labelpicker.Model    // non-nil when label picker is active
	loader         *data.WorkspaceLoader // reused so its per-worktree cache survives reloads
	worktrees      []data.WorktreeInfo   // only worktrees that are working on something
	attributionErr error                 // why worktree attribution is off, when it is
	loading        bool                  // a reload is in flight
	refreshPending bool                  // a filesystem event arrived during a load
	pollScheduled  bool                  // one periodic activity probe is outstanding

	worktreeNames []string // sorted worktree names, for consistent color indexing

	// embedded is set when another Bubble Tea program hosts Grapes. Quitting
	// then emits CloseMsg instead of ending the program, and the status bar
	// offers the sessions key, which only the host can answer.
	embedded bool

	statusMsg      string // transient error/info message for status bar
	editingIssueID int    // issue ID for in-progress editor session
	editingTmpPath string // temp file path for editor
	editingMode    string // "comment" or "edit"
}

func sourceConfigChanged(a, b config.SourcesConfig) bool {
	if a.DefaultBranch != b.DefaultBranch || len(a.Dirs) != len(b.Dirs) {
		return true
	}
	for i := range a.Dirs {
		if a.Dirs[i] != b.Dirs[i] {
			return true
		}
	}
	return false
}

func NewModel(ws data.Workspace, loader *data.WorkspaceLoader, issuesDir string, cfg config.Config, version string) Model {
	issues := ws.Issues

	// Live reload is a headline feature, so a watcher that fails to start is
	// worth saying out loud rather than degrading to a silently static view.
	//
	// Watch active stores recursively only to their numeric issue directories.
	// Idle worktrees are represented by one root watch and discovered by the
	// periodic activity probe below.
	var watchErr error
	w, err := fsnotify.NewWatcher()
	if err != nil {
		watchErr = err
	} else {
		for i, dir := range ws.WatchDirs {
			if e := addWatchDirs(w, dir); e != nil && watchErr == nil && i == 0 {
				watchErr = e // only the canonical store failing disables reload
			}
		}
		for _, root := range ws.WatchRoots {
			_ = addWatchRoot(w, root)
		}
	}

	// Configured startup sort. An unknown name is reported rather than ignored.
	var sortErr string
	sortMode := data.SortByPriority
	if name := cfg.View.DefaultSort; name != "" {
		mode, ok := data.ParseSortMode(name)
		if ok {
			sortMode = mode
		} else {
			sortErr = "Unknown default_sort " + strconv.Quote(name) + " (using priority)"
		}
	}
	data.SortIssues(issues, sortMode, false)

	filters := filter.Default()
	var filtered []data.Issue
	for _, iss := range issues {
		if filters.Matches(iss) {
			filtered = append(filtered, iss)
		}
	}

	wtNames := ws.WorktreeNames()

	l := list.New(filtered)
	l = l.SetSortState(sortMode, false)

	theme := common.NewThemeFromConfig(cfg.Theme, true) // dark default until BackgroundColorMsg arrives

	// Apply configured default screen
	screen := common.ScreenBoard
	if cfg.View.DefaultScreen == "list" {
		screen = common.ScreenList
	}

	// Apply configured keybindings
	common.ApplyKeys(cfg.Keys)

	// Startup problems share one status line; the first one is shown.
	statusMsg := sortErr
	if watchErr != nil {
		statusMsg = "Live reload unavailable: " + watchErr.Error()
	}

	if loader == nil {
		loader = data.NewWorkspaceLoader()
	}

	return Model{
		loader:         loader,
		worktrees:      ws.Worktrees,
		attributionErr: ws.AttributionErr,
		version:        version,
		statusMsg:      statusMsg,
		issues:         issues,
		issuesDir:      issuesDir,
		screen:         screen,
		sortMode:       sortMode,
		filters:        filters,
		cfg:            cfg,
		theme:          theme,
		worktreeNames:  wtNames,
		pollScheduled:  true,
		board:          board.New(filtered).SetSortMode(sortMode).SetHideEmpty(cfg.View.HideEmpty()).SetTheme(theme).SetWorktreeNames(wtNames),
		list:           l.SetTheme(theme).SetWorktreeNames(wtNames),
		watcher:        w,
	}
}

// WithStatus sets the initial status bar message, for reporting problems the
// caller hit before the TUI took over the screen.
func (m Model) WithStatus(msg string) Model {
	m.statusMsg = msg
	return m
}

// Embedded returns the model set up to run inside another Bubble Tea program.
func (m Model) Embedded() Model {
	m.embedded = true
	return m
}

// hostHints returns the status bar hints that end the board, list, and detail
// screens: quit standalone, sessions and back when embedded.
func (m Model) hostHints() []string {
	hint, k, gk := m.theme.FormatKeyHint, common.KeyLabel, common.GlobalKeyMap
	if m.embedded {
		return []string{hint(k(gk.Sessions), "sessions"), hint(k(gk.Quit), "back")}
	}
	return []string{hint(k(gk.Quit), "quit")}
}

// Issues returns the loaded issues, each showing its active source.
func (m Model) Issues() []data.Issue { return m.issues }

// Worktrees returns the worktrees whose branches changed at least one issue.
func (m Model) Worktrees() []data.WorktreeInfo { return m.worktrees }

// Load reads the configuration and every issue source for issuesDir and
// returns the model. Startup problems that do not stop Grapes, a broken config
// or a skipped issue, go to the status bar: the TUI owns the screen once it
// runs, so a warning printed before would be wiped by the alt-screen switch.
func Load(issuesDir, version string) (Model, error) {
	cfg, cfgErr := config.Load(issuesDir)
	// The loader is handed to the TUI rather than rebuilt per reload: it caches
	// what each worktree has changed, keyed on that worktree's HEAD.
	loader := data.NewWorkspaceLoader()
	ws, err := loader.Load(issuesDir, data.WorkspaceOptions{
		DefaultBranch: cfg.Sources.DefaultBranch,
		ExtraDirs:     cfg.Sources.Dirs,
	})
	if err != nil {
		return Model{}, fmt.Errorf("loading issues: %w", err)
	}
	problems := ws.Problems
	m := NewModel(ws, loader, issuesDir, cfg, version)
	switch {
	case cfgErr != nil:
		m = m.WithStatus("Config error (using defaults): " + cfgErr.Error())
	case len(problems) == 1:
		m = m.WithStatus("Skipped " + problems[0].Error())
	case len(problems) > 1:
		m = m.WithStatus(fmt.Sprintf("Skipped %s (+%d more)", problems[0].Error(), len(problems)-1))
	}
	return m, nil
}

// issueSourceDir returns the .grapes/ directory for the given issue ID.
func (m Model) issueSourceDir(issueID int) string {
	for _, iss := range m.issues {
		if iss.ID == issueID {
			if iss.SourceDir != "" {
				return iss.SourceDir
			}
			return m.issuesDir
		}
	}
	return m.issuesDir
}

// childUpdate holds info for cascading a status change to a child issue.
type childUpdate struct {
	dir string
	id  int
}

// childStatusUpdates returns the list of child issues to cascade to "done"
// when the given issue is being set to the given status. Returns nil if
// auto-close is disabled or the new status is not "done".
func (m Model) childStatusUpdates(issueID int, newStatus string) []childUpdate {
	if !m.cfg.View.AutoCloseSubs || newStatus != string(data.StatusDone) {
		return nil
	}
	var issue *data.Issue
	for i := range m.issues {
		if m.issues[i].ID == issueID {
			issue = &m.issues[i]
			break
		}
	}
	if issue == nil || len(issue.Children) == 0 {
		return nil
	}
	var updates []childUpdate
	for _, childID := range issue.Children {
		for _, iss := range m.issues {
			if iss.ID == childID && iss.Status != data.StatusDone && iss.Status != data.StatusCancelled {
				dir := iss.SourceDir
				if dir == "" {
					dir = m.issuesDir
				}
				updates = append(updates, childUpdate{dir: dir, id: childID})
				break
			}
		}
	}
	return updates
}

// sourcePickerTitle names the source filter, and says why there are no
// worktrees to choose from when attribution could not run. Without this, a
// project outside git looks identical to one where every worktree is idle.
func (m Model) sourcePickerTitle() string {
	if len(m.worktrees) == 0 && m.attributionErr != nil {
		return "Source — worktrees unavailable: " + m.attributionErr.Error()
	}
	return "Source"
}

// loadProblemSummary turns skipped issues into one status-bar line, naming the
// first problem and counting the rest. An issue that fails to parse disappears
// from the board, which is confusing if nothing says so.
func loadProblemSummary(problems []data.LoadProblem) string {
	switch len(problems) {
	case 0:
		return ""
	case 1:
		return "Skipped " + problems[0].Error()
	default:
		return fmt.Sprintf("Skipped %s (+%d more)", problems[0].Error(), len(problems)-1)
	}
}

// loadWorkspaceCmd reads the workspace off the event loop.
func (m Model) loadWorkspaceCmd() tea.Cmd {
	loader, dir, cfg := m.loader, m.issuesDir, m.cfg
	return func() tea.Msg {
		ws, err := loader.Load(dir, data.WorkspaceOptions{
			DefaultBranch: cfg.Sources.DefaultBranch,
			ExtraDirs:     cfg.Sources.Dirs,
		})
		return common.WorkspaceLoadedMsg{Workspace: ws, Err: err}
	}
}

// pruneWatchDirs drops watches that are no longer wanted: directories that have
// been deleted, and the issue directories of worktrees that have stopped
// working on anything. Store roots keep their numeric descendants; checkout
// roots are sentinel watches and must not keep every child directory alive.
func pruneWatchDirs(w *fsnotify.Watcher, storeRoots, checkoutRoots []string) {
	inStore := func(dir string) bool {
		for _, root := range storeRoots {
			if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	isCheckoutRoot := func(dir string) bool {
		for _, root := range checkoutRoots {
			if dir == root {
				return true
			}
		}
		return false
	}
	for _, dir := range w.WatchList() {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() || (!inStore(dir) && !isCheckoutRoot(dir)) {
			_ = w.Remove(dir)
		}
	}
}

// addWatchDirs watches the issues directory and all numeric subdirectories.
//
// A failure on issuesDir itself is returned, since that one disables live
// reload for the whole source. Failures on individual issue subdirectories are
// deliberately ignored: a directory can legitimately vanish between the ReadDir
// and the Add, and the parent watch still reports changes underneath it.
func addWatchDirs(w *fsnotify.Watcher, issuesDir string) error {
	if err := w.Add(issuesDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		_ = w.Add(filepath.Join(issuesDir, e.Name()))
	}
	return nil
}

func addWatchRoot(w *fsnotify.Watcher, root string) error {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s is not a directory", root)
	}
	return w.Add(root)
}

// watchCmd blocks on the fsnotify watcher and returns a RefreshMsg when files change.
// It debounces rapid events by draining for 100ms after the first event.
func (m Model) watchCmd() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	w := m.watcher
	return func() tea.Msg {
		for {
			select {
			case _, ok := <-w.Events:
				if !ok {
					return nil
				}
				// Debounce: drain events for 100ms
				timer := time.NewTimer(100 * time.Millisecond)
			drain:
				for {
					select {
					case _, ok := <-w.Events:
						if !ok {
							timer.Stop()
							return nil
						}
					case <-timer.C:
						break drain
					}
				}
				return common.RefreshMsg{}
			case err, ok := <-w.Errors:
				if !ok {
					return nil
				}
				// Live reload is degraded from here; say so instead of going
				// quietly static.
				return common.WatchErrMsg{Err: err}
			}
		}
	}
}
func (m Model) pollCmd() tea.Cmd {
	loader, dir, cfg := m.loader, m.issuesDir, m.cfg
	return tea.Tick(workspacePollInterval, func(time.Time) tea.Msg {
		changed, err := loader.ActivityChanged(dir, data.WorkspaceOptions{
			DefaultBranch: cfg.Sources.DefaultBranch,
			ExtraDirs:     cfg.Sources.Dirs,
		})
		return workspacePollMsg{Changed: changed, Err: err}
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.board.Init(), m.list.Init(), m.watchCmd(), m.pollCmd(), tea.RequestBackgroundColor)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := m.contentHeight()
		off := m.topOffset()
		m.board = m.board.SetTopOffset(off).SetSize(m.width, contentHeight)
		m.list = m.list.SetTopOffset(off).SetSize(m.width, contentHeight)
		m.detail = m.detail.SetTopOffset(off).SetSize(m.width, contentHeight)
		m.settings = m.settings.SetTopOffset(off).SetSize(m.width, contentHeight)
		return m, nil

	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.theme = common.NewThemeFromConfig(m.cfg.Theme, m.isDark)
		m.board = m.board.SetTheme(m.theme)
		m.list = m.list.SetTheme(m.theme)
		m.detail = m.detail.SetTheme(m.theme)
		m.settings = m.settings.SetTheme(m.theme).SetDark(m.isDark)
		return m, nil

	case tea.KeyPressMsg:
		// When filter overlays are active, route all input to them
		if m.filterPicker != nil {
			var cmd tea.Cmd
			fp := *m.filterPicker
			fp, cmd = fp.Update(msg)
			m.filterPicker = &fp
			return m, cmd
		}
		if m.filterMenu != nil {
			var cmd tea.Cmd
			fm := *m.filterMenu
			fm, cmd = fm.Update(msg)
			m.filterMenu = &fm
			return m, cmd
		}
		// When picker is active, route all input to it
		if m.picker != nil {
			var cmd tea.Cmd
			p := *m.picker
			p, cmd = p.Update(msg)
			m.picker = &p
			return m, cmd
		}
		// When label picker is active, route all input to it
		if m.labelPicker != nil {
			var cmd tea.Cmd
			lp := *m.labelPicker
			lp, cmd = lp.Update(msg)
			m.labelPicker = &lp
			return m, cmd
		}
		// Global quit — but not when filtering in list or board view
		if (m.screen == common.ScreenList && m.list.Filtering()) ||
			(m.screen == common.ScreenBoard && m.board.Filtering()) {
			break // fall through to screen-specific handler
		}
		if key.Matches(msg, common.GlobalKeyMap.Quit) {
			if m.embedded {
				// The host may show Grapes again, so the watcher keeps running.
				return m, func() tea.Msg { return common.CloseMsg{} }
			}
			if m.watcher != nil {
				m.watcher.Close()
			}
			return m, tea.Quit
		}
		// Open settings
		if key.Matches(msg, common.GlobalKeyMap.Settings) && m.screen != common.ScreenSettings {
			m.settings = settings.New(m.cfg, m.issuesDir, m.width, m.contentHeight(), m.theme).SetTopOffset(m.topOffset())
			m.navStack = append(m.navStack, navEntry{screen: m.screen})
			m.screen = common.ScreenSettings
			return m, nil
		}

	case tea.MouseReleaseMsg, tea.MouseWheelMsg:
		// When any overlay is active, swallow release/wheel events so they
		// don't leak to background views (e.g. board interpreting release as
		// a card click, which would appear to "close" the overlay).
		if m.filterPicker != nil || m.filterMenu != nil || m.picker != nil || m.labelPicker != nil {
			return m, nil
		}

	case tea.MouseClickMsg, tea.MouseMotionMsg:
		// When filter overlays are active, route all mouse events to them
		if m.filterPicker != nil {
			m.updateFilterPickerPosition()
			var cmd tea.Cmd
			fp := *m.filterPicker
			fp, cmd = fp.Update(msg)
			m.filterPicker = &fp
			return m, cmd
		}
		if m.filterMenu != nil {
			m.updateFilterMenuPosition()
			var cmd tea.Cmd
			fm := *m.filterMenu
			fm, cmd = fm.Update(msg)
			m.filterMenu = &fm
			return m, cmd
		}
		// When picker is active, route all mouse events to it
		if m.picker != nil {
			m.updatePickerPosition()
			var cmd tea.Cmd
			p := *m.picker
			p, cmd = p.Update(msg)
			m.picker = &p
			return m, cmd
		}
		// When label picker is active, route all mouse events to it
		if m.labelPicker != nil {
			m.updateLabelPickerPosition()
			var cmd tea.Cmd
			lp := *m.labelPicker
			lp, cmd = lp.Update(msg)
			m.labelPicker = &lp
			return m, cmd
		}

		if click, ok := msg.(tea.MouseClickMsg); ok {
			mouse := click.Mouse()
			if click.Button == tea.MouseLeft && mouse.Y == 0 {
				// Detect clicks on header tabs (Board / List / Settings)
				boardTabW := lipgloss.Width(m.theme.StyleTabInactive.Render("Board"))
				listTabW := lipgloss.Width(m.theme.StyleTabInactive.Render("List"))
				settingsTabW := lipgloss.Width(m.theme.StyleTabInactive.Render("Config"))
				totalTabsW := boardTabW + 1 + listTabW + 1 + settingsTabW // +1 for spaces
				tabsStart := m.width - totalTabsW
				x := mouse.X
				if x >= tabsStart && x < tabsStart+boardTabW {
					m.navStack = nil
					m.screen = common.ScreenBoard
					return m, nil
				}
				listStart := tabsStart + boardTabW + 1
				if x >= listStart && x < listStart+listTabW {
					m.navStack = nil
					m.screen = common.ScreenList
					return m, nil
				}
				settingsStart := listStart + listTabW + 1
				if x >= settingsStart && x < settingsStart+settingsTabW {
					m.settings = settings.New(m.cfg, m.issuesDir, m.width, m.contentHeight(), m.theme).SetTopOffset(m.topOffset())
					m.navStack = append(m.navStack, navEntry{screen: m.screen})
					m.screen = common.ScreenSettings
					return m, nil
				}
			}
			// Click on filter bar → open filter menu
			if click.Button == tea.MouseLeft && mouse.Y == common.AppHeaderHeight {
				if m.screen == common.ScreenBoard || m.screen == common.ScreenList {
					menu := filter.NewMenu(m.filters, len(m.collectAllLabels()), m.theme)
					m.filterMenu = &menu
					return m, nil
				}
			}
		}
		// Non-tab clicks fall through to active screen delegation

	case common.SessionsMsg:
		// Only a host program knows about sessions; it handles this message
		// before forwarding anything to Grapes.
		return m, nil

	case common.OpenDetailMsg:
		var iss *data.Issue
		for i := range m.issues {
			if m.issues[i].ID == msg.ID {
				iss = &m.issues[i]
				break
			}
		}
		if iss != nil {
			m.navStack = append(m.navStack, navEntry{screen: m.screen, detail: m.detail})
			m.screen = common.ScreenDetail
			m.detail = detail.New(*iss, m.issues, m.width, m.contentHeight(), m.theme).SetTopOffset(m.topOffset()).SetWorktreeNames(m.worktreeNames)
			return m, m.detail.Init()
		}
		return m, nil

	case common.SwitchSourceMsg:
		for i := range m.issues {
			if m.issues[i].ID == msg.IssueID {
				m.issues[i].SwitchSource(msg.SourceIdx)
				data.RewireRelationships(m.issues)
				filtered := m.filteredIssues()
				m.board = m.board.SetIssues(filtered)
				m.list = m.list.SetIssues(filtered)
				if m.screen == common.ScreenDetail {
					m.detail = detail.New(m.issues[i], m.issues, m.width, m.contentHeight(), m.theme).SetTopOffset(m.topOffset()).SetWorktreeNames(m.worktreeNames)
				}
				break
			}
		}
		return m, nil

	case common.GoBackMsg:
		if len(m.navStack) == 0 {
			m.screen = common.ScreenBoard
			return m, nil
		}
		top := m.navStack[len(m.navStack)-1]
		m.navStack = m.navStack[:len(m.navStack)-1]
		m.screen = top.screen
		if top.screen == common.ScreenDetail {
			m.detail = m.detail.SetTopOffset(m.topOffset()).SetSize(m.width, m.contentHeight())
		}
		return m, nil

	case common.SwitchScreenMsg:
		m.navStack = nil
		m.screen = msg.Screen
		return m, nil

	case common.ConfigSavedMsg:
		sourcesChanged := sourceConfigChanged(m.cfg.Sources, msg.Config.Sources)
		m.cfg = msg.Config
		m.theme = common.NewThemeFromConfig(msg.Config.Theme, m.isDark)
		m.board = m.board.SetHideEmpty(msg.Config.View.HideEmpty()).SetTheme(m.theme)
		m.list = m.list.SetTheme(m.theme)
		m.detail = m.detail.SetTheme(m.theme)
		common.ApplyKeys(msg.Config.Keys)
		if mode, ok := data.ParseSortMode(msg.Config.View.DefaultSort); ok {
			m.sortMode = mode
			m.sortAsc = false
			data.SortIssues(m.issues, m.sortMode, m.sortAsc)
			m.board = m.board.SetSortMode(m.sortMode)
			m.list = m.list.SetSortState(m.sortMode, m.sortAsc)
		}
		filtered := m.filteredIssues()
		m.board = m.board.SetIssues(filtered)
		m.list = m.list.SetIssues(filtered)
		// Go back to previous screen
		if len(m.navStack) > 0 {
			top := m.navStack[len(m.navStack)-1]
			m.navStack = m.navStack[:len(m.navStack)-1]
			m.screen = top.screen
		} else {
			m.screen = common.ScreenBoard
		}
		if sourcesChanged {
			if m.loading {
				m.refreshPending = true
			} else {
				m.loading = true
				return m, m.loadWorkspaceCmd()
			}
		}
		return m, nil

	case common.ThemeMsg:
		m.theme = msg.Theme
		m.board = m.board.SetTheme(m.theme)
		m.list = m.list.SetTheme(m.theme)
		m.detail = m.detail.SetTheme(m.theme)
		m.settings = m.settings.SetTheme(m.theme)
		return m, nil
	case common.CycleSortMsg:
		m.sortMode = m.sortMode.Next()
		m.sortAsc = false // reset direction when changing sort field
		data.SortIssues(m.issues, m.sortMode, m.sortAsc)
		filtered := m.filteredIssues()
		m.board = m.board.SetSortMode(m.sortMode).SetIssues(filtered)
		m.list = m.list.SetSortState(m.sortMode, m.sortAsc).SetIssues(filtered)
		return m, nil

	case common.ToggleEmptyColumnsMsg:
		filtered := m.filteredIssues()
		m.board = m.board.SetIssuesAndHideEmpty(filtered, !m.board.HideEmpty())
		return m, nil

	case common.ReverseSortMsg:
		m.sortAsc = !m.sortAsc
		data.SortIssues(m.issues, m.sortMode, m.sortAsc)
		filtered := m.filteredIssues()
		m.board = m.board.SetIssues(filtered)
		m.list = m.list.SetSortState(m.sortMode, m.sortAsc).SetIssues(filtered)
		return m, nil

	case common.ColumnSortMsg:
		if m.sortMode == msg.Mode {
			m.sortAsc = !m.sortAsc
		} else {
			m.sortMode = msg.Mode
			m.sortAsc = false
		}
		data.SortIssues(m.issues, m.sortMode, m.sortAsc)
		filtered := m.filteredIssues()
		m.board = m.board.SetSortMode(m.sortMode).SetIssues(filtered)
		m.list = m.list.SetSortState(m.sortMode, m.sortAsc).SetIssues(filtered)
		return m, nil

	case workspacePollMsg:
		m.pollScheduled = false
		if msg.Err != nil {
			m.statusMsg = "Workspace poll failed: " + msg.Err.Error()
			if m.loading {
				return m, nil
			}
			m.pollScheduled = true
			return m, tea.Batch(m.clearStatusAfter(5*time.Second), m.pollCmd())
		}
		if !msg.Changed {
			m.pollScheduled = true
			return m, m.pollCmd()
		}
		if m.loading {
			m.refreshPending = true
			return m, nil
		}
		m.loading = true
		m.pollScheduled = true
		return m, tea.Batch(m.loadWorkspaceCmd(), m.pollCmd())

	case common.RefreshMsg:
		// A second event while loading is coalesced into one follow-up load.
		// The watcher command is restarted because the command that delivered
		// this event has completed.
		if m.loading {
			m.refreshPending = true
			return m, m.watchCmd()
		}
		m.loading = true
		return m, tea.Batch(m.watchCmd(), m.loadWorkspaceCmd())

	case common.WorkspaceLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.loader.InvalidateActivity()
			m.statusMsg = "Reload failed: " + msg.Err.Error()
		} else {
			ws := msg.Workspace
			var problemCmd tea.Cmd
			if s := loadProblemSummary(ws.Problems); s != "" {
				m.statusMsg = s
				problemCmd = m.clearStatusAfter(5 * time.Second)
			}
			wtNames := ws.WorktreeNames()
			m.worktreeNames = wtNames
			m.worktrees = ws.Worktrees
			m.attributionErr = ws.AttributionErr
			issues := ws.Issues
			data.SortIssues(issues, m.sortMode, m.sortAsc)
			m.issues = issues
			filtered := m.filteredIssues()
			m.board = m.board.SetWorktreeNames(wtNames).SetIssues(filtered)
			m.list = m.list.SetWorktreeNames(wtNames).SetIssues(filtered)
			// Update detail view content if it's showing, preserving scroll position.
			if m.screen == common.ScreenDetail {
				found := false
				for _, iss := range issues {
					if iss.ID == m.detail.IssueID() {
						m.detail = m.detail.UpdateIssue(iss, m.issues).SetWorktreeNames(wtNames)
						found = true
						break
					}
				}
				if !found {
					// The active issue was deleted or became invalid. Do not
					// leave a dead editor/navigation target on screen.
					m.navStack = nil
					m.screen = common.ScreenBoard
					m.statusMsg = fmt.Sprintf("Issue #%d is no longer available", m.detail.IssueID())
				}
			}
			if m.watcher != nil {
				for _, dir := range ws.WatchDirs {
					_ = addWatchDirs(m.watcher, dir)
				}
				for _, root := range ws.WatchRoots {
					_ = addWatchRoot(m.watcher, root)
				}
				pruneWatchDirs(m.watcher, ws.WatchDirs, ws.WatchRoots)
			}
			if m.refreshPending {
				m.refreshPending = false
				m.loading = true
				if !m.pollScheduled {
					m.pollScheduled = true
					problemCmd = tea.Batch(problemCmd, m.pollCmd())
				}
				return m, tea.Batch(problemCmd, m.loadWorkspaceCmd())
			}
			if !m.pollScheduled {
				m.pollScheduled = true
				problemCmd = tea.Batch(problemCmd, m.pollCmd())
			}
			return m, problemCmd
		}
		if m.refreshPending {
			m.refreshPending = false
			m.loading = true
			return m, m.loadWorkspaceCmd()
		}
		if !m.pollScheduled {
			m.pollScheduled = true
			return m, tea.Batch(m.clearStatusAfter(5*time.Second), m.pollCmd())
		}
		return m, m.clearStatusAfter(5 * time.Second)

	case common.ShowPickerMsg:
		p := m.buildPicker(msg.IssueID, msg.Field)
		m.picker = &p
		return m, nil

	case common.MoveIssueMsg:
		srcDir := m.issueSourceDir(msg.IssueID)
		childUpdates := m.childStatusUpdates(msg.IssueID, string(msg.NewStatus))
		return m, writeFieldCmd(srcDir, msg.IssueID, "status", string(msg.NewStatus), childUpdates)

	case common.PickerResultMsg:
		m.picker = nil
		srcDir := m.issueSourceDir(msg.IssueID)
		childUpdates := m.childStatusUpdates(msg.IssueID, msg.Value)
		return m, writeFieldCmd(srcDir, msg.IssueID, msg.Field, msg.Value, childUpdates)

	case common.PickerCancelMsg:
		m.picker = nil
		return m, nil

	case common.ShowLabelPickerMsg:
		lp := m.buildLabelPicker(msg.IssueID)
		m.labelPicker = &lp
		return m, nil

	case common.LabelPickerResultMsg:
		m.labelPicker = nil
		srcDir := m.issueSourceDir(msg.IssueID)
		return m, func() tea.Msg {
			if err := data.UpdateLabels(srcDir, msg.IssueID, msg.Labels); err != nil {
				return common.WriteErrMsg{Err: err}
			}
			return nil // fsnotify will trigger refresh
		}

	case common.LabelPickerCancelMsg:
		m.labelPicker = nil
		return m, nil

	case common.ShowFilterMenuMsg:
		menu := filter.NewMenu(m.filters, len(m.collectAllLabels()), m.theme)
		m.filterMenu = &menu
		return m, nil

	case common.FilterMenuSelectMsg:
		m.filterMenu = nil
		picker := m.buildFilterPicker(msg.Field)
		m.filterPicker = &picker
		return m, nil

	case common.FilterToggleChildrenMsg:
		m.filterMenu = nil
		m.filters.ToggleHasChildren()
		m.propagateFilters()
		return m, nil

	case common.FilterToggleTopLevelMsg:
		m.filterMenu = nil
		m.filters.ToggleTopLevelOnly()
		m.propagateFilters()
		return m, nil

	case common.FilterPickerResultMsg:
		m.filterPicker = nil
		m.applyFilterSelection(msg.Field, msg.Selected)
		m.propagateFilters()
		return m, nil

	case common.FilterCancelMsg:
		m.filterMenu = nil
		m.filterPicker = nil
		return m, nil

	case common.ClearAllFiltersMsg:
		m.filterMenu = nil
		m.filters.Clear()
		m.propagateFilters()
		return m, nil

	case common.LaunchEditorMsg:
		tmpFile, err := os.CreateTemp("", "grapes-comment-*.md")
		if err != nil {
			m.statusMsg = "Error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		if err := tmpFile.Chmod(0600); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			m.statusMsg = "Error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		tmpFile.Close()
		m.editingIssueID = msg.ID
		m.editingTmpPath = tmpFile.Name()
		m.editingMode = "comment"

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		c, err := editorCommand(editor, m.editingTmpPath)
		if err != nil {
			os.Remove(m.editingTmpPath)
			m.statusMsg = "Editor error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return common.EditorFinishedMsg{Err: err}
		})

	case common.LaunchEditMsg:
		// Find the issue to serialize
		var issue *data.Issue
		for i := range m.issues {
			if m.issues[i].ID == msg.ID {
				issue = &m.issues[i]
				break
			}
		}
		if issue == nil {
			return m, nil
		}

		tmpFile, err := os.CreateTemp("", "grapes-edit-*.md")
		if err != nil {
			m.statusMsg = "Error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		if _, err := tmpFile.WriteString(data.SerializeIssue(*issue)); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			m.statusMsg = "Error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		if err := tmpFile.Chmod(0600); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			m.statusMsg = "Error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		tmpFile.Close()
		m.editingTmpPath = tmpFile.Name()
		m.editingMode = "edit"

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		c, err := editorCommand(editor, m.editingTmpPath)
		if err != nil {
			os.Remove(m.editingTmpPath)
			m.statusMsg = "Editor error: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return common.EditFinishedMsg{Err: err}
		})

	case common.EditFinishedMsg:
		if msg.Err != nil {
			m.statusMsg = "Editor error: " + msg.Err.Error()
			os.Remove(m.editingTmpPath)
			return m, m.clearStatusAfter(3 * time.Second)
		}
		body, err := os.ReadFile(m.editingTmpPath)
		if err != nil {
			os.Remove(m.editingTmpPath)
			m.statusMsg = "Error reading file: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		text := string(body)
		// Strip error banner (from previous validation retry) before checking emptiness
		cleaned := stripErrorBanner(text)
		if strings.TrimSpace(cleaned) == "" {
			os.Remove(m.editingTmpPath)
			m.statusMsg = "Edit cancelled."
			return m, m.clearStatusAfter(3 * time.Second)
		}

		issueID := m.editingIssueID
		tmpPath := m.editingTmpPath
		srcDir := m.issueSourceDir(issueID)

		saveErr := data.SaveIssueFromText(srcDir, issueID, cleaned)
		if saveErr == nil {
			os.Remove(tmpPath)
			return m, nil // fsnotify will trigger refresh
		}

		// On validation error, prepend the error to the file and re-open the editor
		var valErr *data.EditValidationError
		if errors.As(saveErr, &valErr) {
			// Strip any previous error banner before prepending a fresh one
			cleaned := stripErrorBanner(text)
			banner := "# ERROR: " + valErr.Message + "\n# Fix the issue above, then save and quit. Empty file to cancel.\n\n"
			if err := os.WriteFile(tmpPath, []byte(banner+cleaned), 0600); err != nil {
				os.Remove(tmpPath)
				m.statusMsg = "Editor error: " + err.Error()
				return m, m.clearStatusAfter(3 * time.Second)
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c, err := editorCommand(editor, tmpPath)
			if err != nil {
				os.Remove(tmpPath)
				m.statusMsg = "Editor error: " + err.Error()
				return m, m.clearStatusAfter(3 * time.Second)
			}
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return common.EditFinishedMsg{Err: err}
			})
		}

		// Non-validation error — clean up and show
		os.Remove(tmpPath)
		m.statusMsg = "Write error: " + saveErr.Error()
		return m, m.clearStatusAfter(3 * time.Second)

	case common.EditorFinishedMsg:
		if msg.Err != nil {
			m.statusMsg = "Editor error: " + msg.Err.Error()
			os.Remove(m.editingTmpPath)
			return m, m.clearStatusAfter(3 * time.Second)
		}
		body, err := os.ReadFile(m.editingTmpPath)
		os.Remove(m.editingTmpPath)
		if err != nil {
			m.statusMsg = "Error reading comment: " + err.Error()
			return m, m.clearStatusAfter(3 * time.Second)
		}
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" {
			return m, nil // empty comment, no-op
		}
		issueID := m.editingIssueID
		srcDir := m.issueSourceDir(issueID)
		return m, func() tea.Msg {
			if err := data.AppendComment(srcDir, issueID, trimmed); err != nil {
				return common.WriteErrMsg{Err: err}
			}
			return nil // fsnotify will trigger refresh
		}

	case common.WriteErrMsg:
		m.statusMsg = "Write error: " + msg.Err.Error()
		return m, m.clearStatusAfter(3 * time.Second)

	case common.WatchErrMsg:
		// Keep watching: fsnotify reports recoverable errors here too.
		m.statusMsg = "Live reload error: " + msg.Err.Error()
		return m, tea.Batch(m.watchCmd(), m.clearStatusAfter(5*time.Second))

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil
	}

	// Delegate to active screen
	var cmd tea.Cmd
	switch m.screen {
	case common.ScreenBoard:
		m.board, cmd = m.board.Update(msg)
	case common.ScreenList:
		m.list, cmd = m.list.Update(msg)
	case common.ScreenDetail:
		m.detail, cmd = m.detail.Update(msg)
	case common.ScreenSettings:
		m.settings, cmd = m.settings.Update(msg)
	}
	return m, cmd
}

func (m Model) renderHeader() string {
	title := m.theme.StyleAppTitle.Render("grapes v" + m.version)

	// Active tab follows the current screen; detail inherits from origin screen.
	activeScreen := m.screen
	if activeScreen == common.ScreenDetail {
		activeScreen = m.originScreen()
	}

	renderTab := func(label string, screen common.Screen) string {
		if activeScreen == screen {
			return m.theme.StyleTabActive.Render(label)
		}
		return m.theme.StyleTabInactive.Render(label)
	}

	boardTab := renderTab("Board", common.ScreenBoard)
	listTab := renderTab("List", common.ScreenList)
	settingsTab := renderTab("Config", common.ScreenSettings)

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, boardTab, " ", listTab, " ", settingsTab)
	spacerW := m.width - lipgloss.Width(title) - lipgloss.Width(tabs)
	if spacerW < 0 {
		spacerW = 0
	}
	row := title + strings.Repeat(" ", spacerW) + tabs
	sep := m.theme.StyleSeparator.Render(strings.Repeat("━", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, row, sep)
}

// originScreen returns the non-detail screen that was active before
// entering the detail view chain. Used for tab highlighting.
func (m Model) originScreen() common.Screen {
	for i := len(m.navStack) - 1; i >= 0; i-- {
		if m.navStack[i].screen != common.ScreenDetail {
			return m.navStack[i].screen
		}
	}
	return common.ScreenBoard
}

func (m Model) View() tea.View {
	header := m.renderHeader()

	var content string
	var helpParts []string
	dot := m.theme.StyleStatusSep.Render(" · ")

	contentHeight := m.contentHeight()

	sortArrow := "▼"
	if m.sortAsc {
		sortArrow = "▲"
	}
	sortLabel := m.sortMode.Label() + " " + sortArrow

	// Help hints read the live keymaps, so rebinding a key in the config screen
	// changes what the status bar advertises.
	hint := m.theme.FormatKeyHint
	k := common.KeyLabel
	bk, lk, dk, gk := common.BoardKeyMap, common.ListKeyMap, common.DetailKeyMap, common.GlobalKeyMap

	switch m.screen {
	case common.ScreenBoard:
		content = m.board.View()
		helpParts = []string{
			hint(k(bk.Left)+k(bk.Down)+k(bk.Up)+k(bk.Right), "navigate"),
			hint(k(bk.Open), "open"),
			hint(k(bk.EditIssue), "edit"),
			hint(k(bk.CycleStatus), "status"),
			hint(k(bk.CyclePriority), "priority"),
			hint(k(bk.Labels), "labels"),
			hint(k(bk.Filter), "filter"),
			hint(k(bk.Search), "search"),
			hint(k(bk.CycleSort)+"/"+k(bk.ReverseSort), sortLabel),
			hint(k(bk.ToggleEmpty), "empty cols"),
			hint(k(bk.ToList), "list"),
			hint(k(gk.Settings), "config"),
		}
		helpParts = append(helpParts, m.hostHints()...)
	case common.ScreenList:
		content = m.list.View()
		navHint := k(lk.Down) + k(lk.Up)
		if m.list.HScrollActive() {
			navHint = k(lk.ScrollLeft) + k(lk.Down) + k(lk.Up) + k(lk.ScrollRight)
		}
		helpParts = []string{
			hint(navHint, "navigate"),
			hint(k(lk.Open), "open"),
			hint(k(lk.EditIssue), "edit"),
			hint(k(lk.CycleStatus), "status"),
			hint(k(lk.CyclePriority), "priority"),
			hint(k(lk.Labels), "labels"),
			hint(k(lk.CycleSort)+"/"+k(lk.ReverseSort), sortLabel),
			hint(k(lk.StructuredFilter), "filter"),
			hint(k(lk.Filter), "search"),
			hint(k(lk.ToBoard), "board"),
			hint(k(gk.Settings), "config"),
		}
		helpParts = append(helpParts, m.hostHints()...)
	case common.ScreenDetail:
		content = m.detail.View()
		helpParts = []string{
			hint("jk", "scroll"),
			hint(k(dk.EditIssue), "edit"),
			hint(k(dk.CycleStatus), "status"),
			hint(k(dk.CyclePriority), "priority"),
			hint(k(dk.Labels), "labels"),
			hint(k(dk.AddComment), "comment"),
			hint(k(dk.Back)+"/⌫", "back"),
			hint(k(gk.Settings), "config"),
		}
		helpParts = append(helpParts, m.hostHints()...)
	case common.ScreenSettings:
		content = m.settings.View()
		if m.settings.PickerActive() {
			helpParts = []string{
				m.theme.FormatKeyHint("jk", "navigate"),
				m.theme.FormatKeyHint("enter", "select"),
				m.theme.FormatKeyHint("esc", "cancel"),
			}
		} else {
			helpParts = []string{
				m.theme.FormatKeyHint("jk", "navigate"),
				m.theme.FormatKeyHint("tab", "pane"),
				m.theme.FormatKeyHint("enter", "edit"),
				m.theme.FormatKeyHint("ctrl+s", "save"),
				m.theme.FormatKeyHint("esc", "back"),
			}
		}
	}

	// Pad content to fill the content area (before overlays need it)
	contentLines := strings.Count(content, "\n") + 1
	if contentLines < contentHeight {
		content += strings.Repeat("\n", contentHeight-contentLines)
	}

	// Settings enum picker overlay
	if m.screen == common.ScreenSettings && m.settings.PickerActive() {
		content = overlayCenter(content, m.settings.PickerView(), m.width, contentHeight)
	}

	// Picker overlay: composite the picker box on top of the real content
	if m.picker != nil {
		content = overlayCenter(content, m.picker.View(), m.width, contentHeight)
		helpParts = []string{
			m.theme.FormatKeyHint("jk", "navigate"),
			m.theme.FormatKeyHint("enter", "select"),
			m.theme.FormatKeyHint("esc", "cancel"),
		}
	}

	// Label picker overlay
	if m.labelPicker != nil {
		content = overlayCenter(content, m.labelPicker.View(), m.width, contentHeight)
		helpParts = []string{
			m.theme.FormatKeyHint("jk", "navigate"),
			m.theme.FormatKeyHint("space", "toggle"),
			m.theme.FormatKeyHint("enter", "apply"),
			m.theme.FormatKeyHint("esc", "cancel"),
		}
	}

	// Filter overlays
	if m.filterPicker != nil {
		content = overlayCenter(content, m.filterPicker.View(), m.width, contentHeight)
		helpParts = []string{
			m.theme.FormatKeyHint("jk", "navigate"),
			m.theme.FormatKeyHint("space", "toggle"),
			m.theme.FormatKeyHint("enter", "apply"),
			m.theme.FormatKeyHint("esc", "cancel"),
		}
	} else if m.filterMenu != nil {
		content = overlayCenter(content, m.filterMenu.View(), m.width, contentHeight)
		helpParts = []string{
			m.theme.FormatKeyHint("jk", "navigate"),
			m.theme.FormatKeyHint("enter", "select"),
			m.theme.FormatKeyHint("esc", "cancel"),
		}
	}

	// Build help bar, wrapping to multiple rows if needed
	var helpText string
	if m.statusMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149"))
		helpText = "  " + errStyle.Render(m.statusMsg)
	} else {
		helpText = wrapHelpParts(helpParts, dot, m.width-2) // -2 for status bar padding
	}
	bar := m.theme.StyleStatusBar.Width(m.width).Render(helpText)

	// Render filter bar between header and content (always visible, clickable)
	// Trim content if the bar wraps to multiple lines, so total height stays correct
	if extraLines := strings.Count(helpText, "\n"); extraLines > 0 {
		lines := strings.Split(content, "\n")
		trim := len(lines) - extraLines
		if trim < 1 {
			trim = 1
		}
		content = strings.Join(lines[:trim], "\n")
	}

	// Render filter bar between header and content when filters are active
	filterBar := filter.RenderBar(m.filters, m.width, m.theme)
	full := lipgloss.JoinVertical(lipgloss.Left, header, filterBar, content, bar)

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// writeFieldCmd writes one field on an issue, then cascades "done" to the given
// children. A failure anywhere is reported: a sub-issue that quietly refuses to
// close is worse than one that says why.
func writeFieldCmd(dir string, issueID int, field, value string, children []childUpdate) tea.Cmd {
	return func() tea.Msg {
		if err := data.UpdateField(dir, issueID, field, value); err != nil {
			return common.WriteErrMsg{Err: err}
		}
		for _, cu := range children {
			if err := data.UpdateField(cu.dir, cu.id, "status", string(data.StatusDone)); err != nil {
				return common.WriteErrMsg{Err: fmt.Errorf("closing sub-issue #%d: %w", cu.id, err)}
			}
		}
		return nil // fsnotify will trigger refresh
	}
}

// clearStatusAfter returns a command that clears the status message after a delay.
func (m Model) clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// buildPicker creates a picker model for the given issue field.
func (m Model) buildPicker(issueID int, field string) picker.Model {
	var issue *data.Issue
	for i := range m.issues {
		if m.issues[i].ID == issueID {
			issue = &m.issues[i]
			break
		}
	}

	switch field {
	case "status":
		var opts []picker.Option
		current := 0
		for i, s := range data.AllStatuses {
			if issue != nil && issue.Status == s {
				current = i
			}
			opts = append(opts, picker.Option{
				Value: string(s),
				Label: s.Label(),
				Icon:  common.StatusIcon(s),
				Style: m.theme.StatusStyle(s),
			})
		}
		return picker.New("Status", opts, current, issueID, field, m.theme)

	case "priority":
		var opts []picker.Option
		current := 0
		for i, p := range data.AllPriorities {
			if issue != nil && issue.Priority == p {
				current = i
			}
			opts = append(opts, picker.Option{
				Value: string(p),
				Label: p.Label(),
				Icon:  strings.TrimSpace(common.PriorityIcon(p)),
				Style: m.theme.PriorityStyle(p),
			})
		}
		return picker.New("Priority", opts, current, issueID, field, m.theme)
	}

	// Fallback (shouldn't happen)
	return picker.New(field, nil, 0, issueID, field, m.theme)
}

// updatePickerPosition computes the centered screen position of the picker
// overlay and stores it on the picker model for mouse hit-testing.
func (m *Model) updatePickerPosition() {
	if m.picker == nil {
		return
	}
	pickerView := m.picker.View()
	pickerLines := strings.Split(pickerView, "\n")
	fgH := len(pickerLines)
	fgW := 0
	for _, l := range pickerLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	contentH := m.contentHeight()
	offsetY := common.AppHeaderHeight + filter.BarHeight(m.filters)
	m.picker.ScreenX = (m.width - fgW) / 2
	m.picker.ScreenY = offsetY + (contentH-fgH)/2
	m.picker.ScreenW = fgW
}

// buildLabelPicker creates a label picker for the given issue.
func (m Model) buildLabelPicker(issueID int) labelpicker.Model {
	var issueLabels []string
	for _, iss := range m.issues {
		if iss.ID == issueID {
			issueLabels = iss.Labels
			break
		}
	}
	return labelpicker.New(issueID, m.collectAllLabels(), issueLabels, m.theme)
}

// updateLabelPickerPosition computes the centered screen position of the label picker
// overlay and stores it on the model for mouse hit-testing.
func (m *Model) updateLabelPickerPosition() {
	if m.labelPicker == nil {
		return
	}
	pickerView := m.labelPicker.View()
	pickerLines := strings.Split(pickerView, "\n")
	fgH := len(pickerLines)
	fgW := 0
	for _, l := range pickerLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	contentH := m.contentHeight()
	offsetY := common.AppHeaderHeight + filter.BarHeight(m.filters)
	m.labelPicker.ScreenX = (m.width - fgW) / 2
	m.labelPicker.ScreenY = offsetY + (contentH-fgH)/2
	m.labelPicker.ScreenW = fgW
}

// updateFilterMenuPosition computes the centered screen position of the filter
// menu overlay and stores it on the model for mouse hit-testing.
func (m *Model) updateFilterMenuPosition() {
	if m.filterMenu == nil {
		return
	}
	view := m.filterMenu.View()
	lines := strings.Split(view, "\n")
	fgH := len(lines)
	fgW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	contentH := m.contentHeight()
	offsetY := common.AppHeaderHeight + filter.BarHeight(m.filters)
	m.filterMenu.ScreenX = (m.width - fgW) / 2
	m.filterMenu.ScreenY = offsetY + (contentH-fgH)/2
	m.filterMenu.ScreenW = fgW
}

// updateFilterPickerPosition computes the centered screen position of the filter
// multi-picker overlay and stores it on the model for mouse hit-testing.
func (m *Model) updateFilterPickerPosition() {
	if m.filterPicker == nil {
		return
	}
	view := m.filterPicker.View()
	lines := strings.Split(view, "\n")
	fgH := len(lines)
	fgW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	contentH := m.contentHeight()
	offsetY := common.AppHeaderHeight + filter.BarHeight(m.filters)
	m.filterPicker.ScreenX = (m.width - fgW) / 2
	m.filterPicker.ScreenY = offsetY + (contentH-fgH)/2
	m.filterPicker.ScreenW = fgW
}

// topOffset returns the number of screen lines above the view content
// (app header + optional structured filter bar).
func (m Model) topOffset() int {
	return common.AppHeaderHeight + filter.BarHeight(m.filters)
}

// contentHeight returns the available height for view content, accounting for
// the app header, status bar, and optional filter bar.
func (m Model) contentHeight() int {
	h := m.height - 3 // header(2) + status bar(1)
	h -= filter.BarHeight(m.filters)
	if h < 0 {
		h = 0
	}
	return h
}

// wrapHelpParts arranges help hints into rows that fit within maxWidth,
// wrapping to additional lines as needed.
func wrapHelpParts(parts []string, dot string, maxWidth int) string {
	if len(parts) == 0 {
		return ""
	}
	const indent = "  "
	dotW := ansi.StringWidth(dot)
	indentW := ansi.StringWidth(indent)

	var rows []string
	row := indent
	rowW := indentW

	for i, part := range parts {
		partW := ansi.StringWidth(part)
		if i == 0 {
			row += part
			rowW += partW
			continue
		}
		needed := dotW + partW
		if rowW+needed > maxWidth {
			rows = append(rows, row)
			row = indent + part
			rowW = indentW + partW
		} else {
			row += dot + part
			rowW += needed
		}
	}
	rows = append(rows, row)
	return strings.Join(rows, "\n")
}

// filteredIssues returns issues matching the current structured filters.
func (m Model) filteredIssues() []data.Issue {
	if m.filters.IsEmpty() {
		return m.issues
	}
	var out []data.Issue
	for _, iss := range m.issues {
		if m.filters.Matches(iss) {
			out = append(out, iss)
		}
	}
	return out
}

// collectAllLabels extracts unique labels from all loaded issues (unfiltered).
func (m Model) collectAllLabels() []string {
	seen := make(map[string]bool)
	var labels []string
	for _, iss := range m.issues {
		for _, l := range iss.Labels {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
	}
	sort.Strings(labels)
	return labels
}

// buildFilterPicker creates a MultiPicker for the given filter field.
// Only values that exist in the current issue set are shown.
func (m Model) buildFilterPicker(field string) filter.MultiPicker {
	switch field {
	case "status":
		present := make(map[data.Status]bool)
		for _, iss := range m.issues {
			present[iss.Status] = true
		}
		var opts []filter.PickerOption
		var preSelected []string
		for _, s := range data.AllStatuses {
			if !present[s] {
				continue
			}
			opts = append(opts, filter.PickerOption{
				Value: string(s),
				Label: s.Label(),
				Icon:  common.StatusIcon(s),
				Style: m.theme.StatusStyle(s),
			})
		}
		for _, s := range m.filters.Statuses {
			preSelected = append(preSelected, string(s))
		}
		return filter.NewMultiPicker("Status", "status", opts, preSelected, m.theme)

	case "priority":
		present := make(map[data.Priority]bool)
		for _, iss := range m.issues {
			present[iss.Priority] = true
		}
		var opts []filter.PickerOption
		var preSelected []string
		for _, p := range data.AllPriorities {
			if !present[p] {
				continue
			}
			opts = append(opts, filter.PickerOption{
				Value: string(p),
				Label: p.Label(),
				Icon:  strings.TrimSpace(common.PriorityIcon(p)),
				Style: m.theme.PriorityStyle(p),
			})
		}
		for _, p := range m.filters.Priorities {
			preSelected = append(preSelected, string(p))
		}
		return filter.NewMultiPicker("Priority", "priority", opts, preSelected, m.theme)

	case "labels":
		var opts []filter.PickerOption
		for _, l := range m.collectAllLabels() {
			opts = append(opts, filter.PickerOption{
				Value: l,
				Label: l,
				Style: m.theme.StatusStyle(data.StatusTodo), // neutral color
			})
		}
		return filter.NewMultiPicker("Label", "labels", opts, m.filters.Labels, m.theme)
	case "source":
		// FilterSet.Matches accepts any loaded source on an issue, not only
		// the winning owner. Count each issue once per source so picker
		// counts describe the same set the filter will match.
		sourceCounts := make(map[string]int)
		for _, iss := range m.issues {
			seen := make(map[string]bool)
			if len(iss.Sources) == 0 {
				name := iss.Worktree
				if name == "" {
					name = "main"
				}
				seen[name] = true
			} else {
				for _, src := range iss.Sources {
					name := src.Name
					if name == "" {
						name = "main"
					}
					seen[name] = true
				}
			}
			for name := range seen {
				sourceCounts[name]++
			}
		}
		// A hand-built workspace may provide worktree activity without
		// per-issue source copies. Keep those active sources selectable too.
		for _, wt := range m.worktrees {
			if wt.Name != "" {
				if _, ok := sourceCounts[wt.Name]; !ok {
					sourceCounts[wt.Name] = len(wt.Touched)
				}
			}
		}
		names := make([]string, 0, len(sourceCounts))
		for name := range sourceCounts {
			if name != "main" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		var opts []filter.PickerOption
		if n := sourceCounts["main"]; n > 0 {
			opts = append(opts, filter.PickerOption{
				Value: "main",
				Label: fmt.Sprintf("%s main (%d)", common.MainIcon(), n),
				Style: m.theme.StyleSubtitle,
			})
		}
		for _, name := range names {
			c := m.theme.WorktreeColorFor(name, names)
			opts = append(opts, filter.PickerOption{
				Value: name,
				Label: fmt.Sprintf("%s %s (%d)", common.WorktreeIcon(), name, sourceCounts[name]),
				Style: lipgloss.NewStyle().Foreground(c),
			})
		}
		return filter.NewMultiPicker(m.sourcePickerTitle(), "source", opts, m.filters.Sources, m.theme)
	}

	return filter.NewMultiPicker(field, field, nil, nil, m.theme)
}

// applyFilterSelection updates the filter set from a multi-picker result.
func (m *Model) applyFilterSelection(field string, selected []string) {
	switch field {
	case "status":
		m.filters.SetStatuses(selected)
	case "priority":
		m.filters.SetPriorities(selected)
	case "labels":
		m.filters.SetLabels(selected)
	case "source":
		m.filters.SetSources(selected)
	}
}

// propagateFilters sends filtered issues to both views and adjusts sizes.
func (m *Model) propagateFilters() {
	filtered := m.filteredIssues()
	m.board = m.board.SetIssuesAndStatusFilter(filtered, m.filters.Statuses)
	m.list = m.list.SetIssues(filtered)
	contentHeight := m.contentHeight()
	off := m.topOffset()
	m.board = m.board.SetTopOffset(off).SetSize(m.width, contentHeight)
	m.list = m.list.SetTopOffset(off).SetSize(m.width, contentHeight)
}

// stripErrorBanner removes a leading "# ERROR: ..." banner that was prepended by
// a previous validation failure, so it doesn't accumulate on repeated retries.
func stripErrorBanner(text string) string {
	const prefix = "# ERROR: "
	const instruction = "# Fix the issue above, then save and quit. Empty file to cancel."
	if !strings.HasPrefix(text, prefix) {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || lines[1] != instruction {
		return text
	}
	i := 2
	for i < len(lines) && lines[i] == "" {
		i++
	}
	return strings.Join(lines[i:], "\n")
}

// overlayCenter composites fg (a small box) centered on top of bg (the full content).
// Uses ANSI-aware truncation to preserve the background content on both sides
// of the overlay box, so board columns / list rows stay visible around the picker.
func overlayCenter(bg, fg string, bgWidth, bgHeight int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	// Ensure bg has enough lines
	for len(bgLines) < bgHeight {
		bgLines = append(bgLines, "")
	}

	// Measure fg box width
	fgWidth := 0
	for _, line := range fgLines {
		if w := lipgloss.Width(line); w > fgWidth {
			fgWidth = w
		}
	}

	// Calculate centering offsets
	startY := (bgHeight - len(fgLines)) / 2
	startX := (bgWidth - fgWidth) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Splice fg lines into bg lines, preserving left and right bg content
	for i, fgLine := range fgLines {
		y := startY + i
		if y >= len(bgLines) {
			break
		}
		bgLine := bgLines[y]

		// Left portion: first startX visible chars of the bg line
		left := ansi.Truncate(bgLine, startX, "")
		// Pad left if the bg line is shorter than startX
		leftW := lipgloss.Width(left)
		if leftW < startX {
			left += strings.Repeat(" ", startX-leftW)
		}

		// Right portion: bg content after the fg box ends
		rightStart := startX + fgWidth
		right := ansi.TruncateLeft(bgLine, rightStart, "")

		bgLines[y] = left + fgLine + right
	}

	return strings.Join(bgLines[:bgHeight], "\n")
}
