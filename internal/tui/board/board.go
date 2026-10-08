package board

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Mibokess/grapes/internal/data"
	"github.com/Mibokess/grapes/internal/tui/common"
	"github.com/charmbracelet/x/ansi"
)

// Card and column layout. Cards are a fixed height so that rendering, scrolling
// and mouse hit-testing all agree on where a row starts.
const (
	// cardHeight is border(2) + ID(1) + title(2) + labels(1) + date(1).
	cardHeight = 7
	// colHeaderHeight is the column title plus its underline.
	colHeaderHeight = 2
	// minColWidth is the narrowest a column may get before one is dropped.
	minColWidth = 22
)

type column struct {
	status data.Status
	issues []data.Issue
}

type Model struct {
	allIssues []data.Issue
	columns   []column
	filter    textinput.Model
	filtering bool
	curCol    int
	curRow    int
	scrollCol int // first visible column index (horizontal scroll)
	scrollRow int // first visible row in current column (vertical scroll)
	width     int
	height    int
	visCols   int // number of visible columns
	sortMode  data.SortMode
	theme     common.Theme

	statusFilter []data.Status // non-empty when user has a status filter active
	hideEmpty    bool          // hide columns with no issues
	topOffset    int           // screen lines above this view's content (app header + filter bar)

	worktreeNames []string // sorted worktree names for consistent color assignment

	// Drag-and-drop state
	mouseDown    bool // left button is held (pending drag)
	dragging     bool // true only once the mouse moves while held
	dragIssueID  int
	dragFromCol  int
	dragOverCol  int // column cursor is hovering over (-1 = none)
	dragX, dragY int // current cursor position (screen coords)
}

// SetWorktreeNames sets the sorted worktree names for color assignment.
func (m Model) SetWorktreeNames(names []string) Model {
	m.worktreeNames = names
	return m
}

func (m Model) SetTheme(t common.Theme) Model {
	m.theme = t
	return m
}

func (m Model) SetTopOffset(n int) Model {
	m.topOffset = n
	return m
}

// SetStatusFilter sets the active status filter for column hiding.
func (m Model) SetStatusFilter(statuses []data.Status) Model {
	m.statusFilter = statuses
	return m.regroup()
}

// SetHideEmpty controls whether columns with no issues are hidden.
func (m Model) SetHideEmpty(hide bool) Model {
	m.hideEmpty = hide
	return m.regroup()
}

// regroup rebuilds the columns from the current issues and options, then clamps
// the cursor into the new layout. Every setter that changes what the columns
// contain goes through it, so the view can never disagree with the options.
func (m Model) regroup() Model {
	m.columns = groupByStatus(m.filteredIssues(), m.statusFilter, m.hideEmpty)
	if len(m.columns) > 0 && m.visCols == 0 {
		m.visCols = 1
	}
	if m.curCol >= len(m.columns) {
		m.curCol = max(0, len(m.columns)-1)
	}
	m.clampRow()
	m.ensureRowVisible()
	m.ensureColVisible()
	return m
}

// SetIssuesAndStatusFilter updates both inputs before regrouping once.
func (m Model) SetIssuesAndStatusFilter(issues []data.Issue, statuses []data.Status) Model {
	m.allIssues = issues
	m.statusFilter = statuses
	return m.regroup()
}

// SetIssuesAndHideEmpty updates both inputs before regrouping once.
func (m Model) SetIssuesAndHideEmpty(issues []data.Issue, hide bool) Model {
	m.allIssues = issues
	m.hideEmpty = hide
	return m.regroup()
}

// currentIssue returns the issue under the cursor, if there is one.
func (m Model) currentIssue() (data.Issue, bool) {
	col, ok := m.currentColumn()
	if !ok || m.curRow < 0 || m.curRow >= len(col.issues) {
		return data.Issue{}, false
	}
	return col.issues[m.curRow], true
}

// currentColumn returns the column under the cursor, if there is one. Every
// column access goes through it: with hide-empty on (the default), a filter
// that matches nothing leaves no columns at all.
func (m Model) currentColumn() (column, bool) {
	if m.curCol < 0 || m.curCol >= len(m.columns) {
		return column{}, false
	}
	return m.columns[m.curCol], true
}

// HideEmpty returns whether empty columns are currently hidden.
func (m Model) HideEmpty() bool {
	return m.hideEmpty
}

func New(issues []data.Issue) Model {
	ti := textinput.New()
	ti.Placeholder = "Search all fields..."
	ti.CharLimit = 100

	m := Model{allIssues: issues, filter: ti, visCols: 3, theme: common.NewTheme(true)}
	m.columns = groupByStatus(issues, nil, false)
	return m
}

func (m Model) Filtering() bool { return m.filtering }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	if m.width >= 160 {
		m.visCols = min(5, len(m.columns))
	} else if m.width >= 120 {
		m.visCols = min(4, len(m.columns))
	} else {
		m.visCols = min(3, len(m.columns))
	}
	if len(m.columns) > 0 && m.visCols == 0 {
		m.visCols = 1
	}
	m.ensureColVisible()
	return m
}

func (m Model) SetIssues(issues []data.Issue) Model {
	m.allIssues = issues
	return m.regroup()
}

func (m Model) SetSortMode(mode data.SortMode) Model {
	m.sortMode = mode
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.filtering {
			switch {
			case key.Matches(msg, common.BoardKeyMap.Clear):
				m.filtering = false
				m.filter.SetValue("")
				m.filter.Blur()
				m.columns = groupByStatus(m.allIssues, m.statusFilter, m.hideEmpty)
				m.curCol = 0
				m.curRow = 0
				m.scrollRow = 0
				m.ensureColVisible()
				return m, nil
			case msg.Code == tea.KeyEnter:
				m.filtering = false
				m.filter.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				m.columns = groupByStatus(m.filteredIssues(), m.statusFilter, m.hideEmpty)
				m.curCol = 0
				m.curRow = 0
				m.scrollRow = 0
				m.ensureColVisible()
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, common.BoardKeyMap.Search):
			m.filtering = true
			m.filter.Focus()
			return m, textinput.Blink
		case key.Matches(msg, common.BoardKeyMap.Left):
			for ci := m.curCol - 1; ci >= 0; ci-- {
				if len(m.columns[ci].issues) > 0 {
					m.curCol = ci
					m.clampRow()
					m.scrollRow = 0
					m.ensureRowVisible()
					m.ensureColVisible()
					break
				}
			}
		case key.Matches(msg, common.BoardKeyMap.Right):
			for ci := m.curCol + 1; ci < len(m.columns); ci++ {
				if len(m.columns[ci].issues) > 0 {
					m.curCol = ci
					m.clampRow()
					m.scrollRow = 0
					m.ensureRowVisible()
					m.ensureColVisible()
					break
				}
			}
		case key.Matches(msg, common.BoardKeyMap.Up):
			if m.curRow > 0 {
				m.curRow--
				m.ensureRowVisible()
			} else {
				// Wrap to last issue in previous non-empty column
				for ci := m.curCol - 1; ci >= 0; ci-- {
					if len(m.columns[ci].issues) > 0 {
						m.curCol = ci
						m.curRow = len(m.columns[ci].issues) - 1
						m.scrollRow = 0
						m.ensureRowVisible()
						m.ensureColVisible()
						break
					}
				}
			}
		case key.Matches(msg, common.BoardKeyMap.Down):
			col, ok := m.currentColumn()
			if !ok {
				break
			}
			if m.curRow < len(col.issues)-1 {
				m.curRow++
				m.ensureRowVisible()
			} else {
				// Wrap to first issue in next non-empty column
				for ci := m.curCol + 1; ci < len(m.columns); ci++ {
					if len(m.columns[ci].issues) > 0 {
						m.curCol = ci
						m.curRow = 0
						m.scrollRow = 0
						m.ensureRowVisible()
						m.ensureColVisible()
						break
					}
				}
			}
		case key.Matches(msg, common.BoardKeyMap.Open):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg { return common.OpenDetailMsg{ID: issue.ID} }
			}
		case key.Matches(msg, common.GlobalKeyMap.Sessions):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg { return common.SessionsMsg{IssueID: issue.ID} }
			}
		case key.Matches(msg, common.BoardKeyMap.EditIssue):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg { return common.LaunchEditMsg{ID: issue.ID} }
			}
		case key.Matches(msg, common.BoardKeyMap.CycleStatus):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg {
					return common.ShowPickerMsg{IssueID: issue.ID, Field: "status"}
				}
			}
		case key.Matches(msg, common.BoardKeyMap.CyclePriority):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg {
					return common.ShowPickerMsg{IssueID: issue.ID, Field: "priority"}
				}
			}
		case key.Matches(msg, common.BoardKeyMap.Labels):
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg {
					return common.ShowLabelPickerMsg{IssueID: issue.ID}
				}
			}
		case key.Matches(msg, common.BoardKeyMap.CycleSort):
			return m, func() tea.Msg { return common.CycleSortMsg{} }
		case key.Matches(msg, common.BoardKeyMap.ReverseSort):
			return m, func() tea.Msg { return common.ReverseSortMsg{} }
		case key.Matches(msg, common.BoardKeyMap.Filter):
			return m, func() tea.Msg { return common.ShowFilterMenuMsg{} }
		case key.Matches(msg, common.BoardKeyMap.ToggleEmpty):
			return m, func() tea.Msg { return common.ToggleEmptyColumnsMsg{} }
		case key.Matches(msg, common.BoardKeyMap.ToList):
			return m, func() tea.Msg { return common.SwitchScreenMsg{Screen: common.ScreenList} }
		}

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			// Scroll left through columns, skipping empty ones
			for ci := m.curCol - 1; ci >= 0; ci-- {
				if len(m.columns[ci].issues) > 0 {
					m.curCol = ci
					m.clampRow()
					m.scrollRow = 0
					m.ensureRowVisible()
					m.ensureColVisible()
					break
				}
			}
		} else if msg.Button == tea.MouseWheelDown {
			// Scroll right through columns, skipping empty ones
			for ci := m.curCol + 1; ci < len(m.columns); ci++ {
				if len(m.columns[ci].issues) > 0 {
					m.curCol = ci
					m.clampRow()
					m.scrollRow = 0
					m.ensureRowVisible()
					m.ensureColVisible()
					break
				}
			}
		}

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		switch msg.Button {
		case tea.MouseLeft:
			if colIdx, rowIdx, ok := m.cardAt(mouse.X, mouse.Y); ok {
				// Select the card; prepare for potential drag
				m.curCol = colIdx
				m.curRow = rowIdx
				m.ensureColVisible()
				m.ensureRowVisible()
				issue := m.columns[colIdx].issues[rowIdx]
				m.mouseDown = true
				m.dragging = false
				m.dragIssueID = issue.ID
				m.dragFromCol = colIdx
				m.dragOverCol = colIdx
			} else if colIdx, dir := m.moreIndicatorAt(mouse.X, mouse.Y); dir != 0 {
				// Click on a "more" indicator — scroll in that direction
				m.curCol = colIdx
				m.ensureColVisible()
				maxCards := m.maxVisibleCards()
				if dir > 0 {
					// "+N more" at bottom — scroll down
					m.scrollRow += maxCards
					col := m.columns[colIdx]
					if m.scrollRow+maxCards > len(col.issues) {
						m.scrollRow = max(0, len(col.issues)-maxCards)
					}
					m.curRow = m.scrollRow
				} else {
					// "↑ N more" at top — scroll up
					m.scrollRow -= maxCards
					if m.scrollRow < 0 {
						m.scrollRow = 0
					}
					m.curRow = m.scrollRow
				}
				m.ensureRowVisible()
			} else if colIdx, ok := m.columnAt(mouse.X); ok && colIdx != m.curCol {
				// Click in column area without hitting a card — select the column
				m.curCol = colIdx
				m.clampRow()
				m.scrollRow = 0
				m.ensureRowVisible()
				m.ensureColVisible()
			}
		case tea.MouseForward:
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg { return common.OpenDetailMsg{ID: issue.ID} }
			}
		}

	case tea.MouseMotionMsg:
		if m.mouseDown && !m.dragging {
			// First movement after click — begin drag
			m.dragging = true
		}
		if m.dragging {
			mouse := msg.Mouse()
			m.dragX = mouse.X
			m.dragY = mouse.Y
			if colIdx, ok := m.columnAt(mouse.X); ok {
				m.dragOverCol = colIdx
			}
		}

	case tea.MouseReleaseMsg:
		wasDragging := m.dragging
		wasMouseDown := m.mouseDown
		fromCol := m.dragFromCol
		overCol := m.dragOverCol
		issueID := m.dragIssueID
		m.mouseDown = false
		m.dragging = false
		m.dragOverCol = -1

		if wasDragging {
			// Real drag — drop on a different column to move the issue
			if overCol != fromCol && overCol >= 0 && overCol < len(m.columns) {
				newStatus := m.columns[overCol].status
				return m, func() tea.Msg {
					return common.MoveIssueMsg{IssueID: issueID, NewStatus: newStatus}
				}
			}
		} else if wasMouseDown {
			// Clean click (no drag movement) — open detail
			if issue, ok := m.currentIssue(); ok {
				return m, func() tea.Msg { return common.OpenDetailMsg{ID: issue.ID} }
			}
		}
	}
	return m, nil
}

func (m *Model) clampRow() {
	col, ok := m.currentColumn()
	if !ok {
		m.curRow = 0
		return
	}
	if m.curRow >= len(col.issues) {
		m.curRow = max(0, len(col.issues)-1)
	}
	if m.curRow < 0 {
		m.curRow = 0
	}
}

// ensureColVisible adjusts horizontal scroll so the current column is on screen.
func (m *Model) ensureColVisible() {
	if m.curCol < m.scrollCol {
		m.scrollCol = m.curCol
	}
	if m.curCol >= m.scrollCol+m.visCols {
		m.scrollCol = m.curCol - m.visCols + 1
	}
}

// maxVisibleCards returns how many cards fit vertically in a column.
func (m Model) maxVisibleCards() int {
	// Each card: border(2) + ID(1) + title(2) + meta(1) + date(1) = 7 lines.
	// Column header takes 2 lines (text + separator).
	// Reserve 1 extra line for a "more" indicator.
	// Reserve the column header plus one line for a "more" indicator.
	const overhead = colHeaderHeight + 1

	available := m.height - overhead
	if m.filtering || m.filter.Value() != "" {
		available-- // filter line takes 1 row
	}
	if available < cardHeight {
		return 1
	}
	return available / cardHeight
}

// ensureRowVisible adjusts scrollRow so curRow is visible in the current column.
func (m *Model) ensureRowVisible() {
	maxCards := m.maxVisibleCards()
	if m.scrollRow > m.curRow {
		m.scrollRow = m.curRow
	}
	if m.curRow >= m.scrollRow+maxCards {
		m.scrollRow = m.curRow - maxCards + 1
	}
	if m.scrollRow < 0 {
		m.scrollRow = 0
	}
}

func (m Model) View() string {
	if m.width == 0 || len(m.columns) == 0 {
		return "No issues found."
	}

	var filterLine string
	if m.filtering {
		filterLine = "  / " + m.filter.View()
	} else if m.filter.Value() != "" {
		filterLine = m.theme.StyleSubtitle.Render(fmt.Sprintf("  Filter: %s", m.filter.Value()))
	}

	visible, colWidth := m.visibleColWidth()

	renderedCols := make([]string, visible)
	for i := 0; i < visible; i++ {
		ci := m.scrollCol + i
		col := m.columns[ci]
		isActiveCol := ci == m.curCol
		colContent := m.renderColumn(col, colWidth, isActiveCol)
		if i < visible-1 {
			colContent = lipgloss.NewStyle().MarginRight(1).Render(colContent)
		}
		renderedCols[i] = colContent
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)

	// Overlay the card being dragged at the cursor, drawn in the active style so
	// it reads as the thing you are holding.
	if m.dragging {
		if issue, ok := m.findIssue(m.dragIssueID); ok {
			floating := m.renderCard(issue, colWidth, cardActive)
			board = m.overlayAt(board, floating, m.dragX, m.dragY-m.topOffset)
		}
	}

	if filterLine != "" {
		return lipgloss.JoinVertical(lipgloss.Left, filterLine, board)
	}
	return board
}

func (m Model) renderColumn(col column, width int, isActive bool) string {
	icon := common.StatusIcon(col.status)
	label := strings.ToUpper(col.status.Label())
	count := m.theme.StyleFaint.Render(fmt.Sprintf("(%d)", len(col.issues)))

	// Highlight column header when it's the drop target during a drag
	isDropTarget := m.dragging && m.dragOverCol >= 0 && m.dragOverCol < len(m.columns) &&
		m.columns[m.dragOverCol].status == col.status && m.dragOverCol != m.dragFromCol

	var headerText, separator string
	if isDropTarget {
		headerText = m.theme.StyleDropTarget.
			Foreground(m.theme.StatusColorFor(col.status)).
			Background(m.theme.StatusColorFor(col.status)).
			Width(width).
			Render(fmt.Sprintf(" %s %s ", icon, label) + count)
		separator = lipgloss.NewStyle().
			Foreground(m.theme.StatusColorFor(col.status)).
			Render(strings.Repeat("━", width))
	} else {
		headerText = m.theme.StatusHeaderStyle(col.status).Width(width).
			Render(fmt.Sprintf(" %s %s ", icon, label) + count)
		sepStyle := lipgloss.NewStyle().Foreground(m.theme.StatusColorFor(col.status))
		separator = sepStyle.Render(strings.Repeat("━", width))
	}
	header := lipgloss.JoinVertical(lipgloss.Left, headerText, separator)

	// Insert a ghost preview card when dragging over this column
	var ghostCard string
	ghostInsertIdx := -1
	if isDropTarget {
		if issue, ok := m.findIssue(m.dragIssueID); ok {
			ghostCard = m.renderCard(issue, width, cardGhost)
			// Figure out which slot the cursor is near based on Y position
			yInCol := m.dragY - m.topOffset - colHeaderHeight
			if yInCol < 0 {
				ghostInsertIdx = 0
			} else {
				ghostInsertIdx = yInCol / cardHeight
			}
		}
	}

	if len(col.issues) == 0 {
		if ghostCard != "" {
			return lipgloss.JoinVertical(lipgloss.Left, header, ghostCard)
		}
		return header
	}

	maxCards := m.maxVisibleCards()

	// Determine scroll window: active column scrolls with cursor, others show from top
	startIdx := 0
	if isActive {
		startIdx = m.scrollRow
	}
	endIdx := startIdx + maxCards
	if endIdx > len(col.issues) {
		endIdx = len(col.issues)
	}

	var cards []string

	if startIdx > 0 {
		cards = append(cards, m.theme.StyleSubtitle.Render(
			fmt.Sprintf("  ↑ %d more", startIdx)))
	}

	visibleIdx := 0
	for i := startIdx; i < endIdx; i++ {
		if ghostCard != "" && visibleIdx == ghostInsertIdx {
			cards = append(cards, ghostCard)
		}
		cards = append(cards, m.renderCard(col.issues[i], width, m.modeFor(col.issues[i], isActive && i == m.curRow)))
		visibleIdx++
	}
	// If cursor is past the last card, append ghost at the end
	if ghostCard != "" && ghostInsertIdx >= visibleIdx {
		cards = append(cards, ghostCard)
	}

	if endIdx < len(col.issues) {
		remaining := len(col.issues) - endIdx
		cards = append(cards, m.theme.StyleSubtitle.Render(
			fmt.Sprintf("  +%d more", remaining)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, cards...)
	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

// cardMode selects how a card is drawn.
type cardMode int

const (
	cardNormal  cardMode = iota
	cardActive           // under the cursor
	cardDragged          // the source card, left in place while a drag is in flight
	cardGhost            // the drop preview shown in the target column
)

// renderCard draws one issue card. Every card is exactly cardHeight lines tall,
// including empty label and date lines, so column layout and mouse hit-testing
// can rely on a fixed row height.
func (m Model) renderCard(issue data.Issue, width int, mode cardMode) string {
	faint := mode == cardDragged || mode == cardGhost

	style := m.theme.StyleCard.Width(width - 2) // -2 for border chars
	switch {
	case faint:
		style = m.theme.StyleDragCard.Width(width - 2)
	case mode == cardActive:
		style = m.theme.StyleActiveCard.Width(width - 2)
	case issue.Worktree != "":
		c := m.theme.WorktreeColorFor(issue.Worktree, m.worktreeNames)
		style = m.theme.StyleCard.Width(width - 2).BorderForeground(c)
	}

	// Inner text width = card width - 2 (border) - 2 (border in Width) - 2 (padding)
	// lipgloss v2 Width() sets the total outer width including border.
	innerW := width - 6

	// Line 1: #ID + priority icon (small, muted — like Linear's "ETA-502")
	line1 := m.theme.StyleFaint.Render(fmt.Sprintf("#%d", issue.ID))
	if mode != cardDragged {
		if issue.Priority.AtLeast(data.PriorityHigh) {
			line1 += " " + m.theme.PriorityStyle(issue.Priority).Render(
				strings.TrimSpace(common.PriorityIcon(issue.Priority)))
		}
		line1 += m.renderSourceSuffix(issue, faint)
	}

	// Lines 2-3: title, always two lines so all cards have the same height.
	titleLine1, titleLine2 := wrapTitle(issue.Title, innerW)
	title := titleLine1 + "\n" + titleLine2
	switch {
	case faint:
		title = m.theme.StyleFaint.Render(title)
	case mode == cardActive:
		title = m.theme.StyleTitle.Render(title)
	}

	// Line 4: labels (compact, muted)
	var meta []string
	used := 0
	for _, l := range issue.Labels {
		lw := ansi.StringWidth(l)
		sep := 0
		if len(meta) > 0 {
			sep = 2 // "  "
		}
		if used+sep+lw > innerW {
			break
		}
		if faint {
			meta = append(meta, m.theme.StyleFaint.Render(l))
		} else {
			meta = append(meta, m.theme.RenderLabel(l))
		}
		used += sep + lw
	}
	metaLine := strings.Join(meta, "  ")

	// Line 5: Date — show "Updated" when sorting by updated, otherwise "Created"
	var dateLine string
	if m.sortMode == data.SortByUpdated && !issue.Updated.IsZero() {
		dateLine = m.theme.StyleFaint.Render("Updated " + issue.Updated.Format("Jan 2 15:04"))
	} else if !issue.Created.IsZero() {
		dateLine = m.theme.StyleFaint.Render("Created " + issue.Created.Format("Jan 2 15:04"))
	}

	// Always include all 5 lines: ID, title (2), meta, date — for uniform card height.
	content := line1 + "\n" + title + "\n" + metaLine + "\n" + dateLine

	return style.Render(content)
}

// modeFor picks the drawing mode for a card in its column: the card being
// dragged stays visible in place, dimmed.
func (m Model) modeFor(issue data.Issue, active bool) cardMode {
	switch {
	case m.dragging && issue.ID == m.dragIssueID:
		return cardDragged
	case active:
		return cardActive
	default:
		return cardNormal
	}
}

// renderSourceSuffix marks the issues a worktree is working on ahead of the
// main checkout, which is the question a board is asked: who has this one?
//
// It deliberately says nothing about issues the main checkout owns, even when a
// worktree also has a version of them. Every worktree holds a copy of every
// issue, so marking those would put a badge on almost every card and tell the
// reader nothing. Divergence is still inspectable in the detail view.
func (m Model) renderSourceSuffix(issue data.Issue, faint bool) string {
	if issue.Worktree == "" {
		return ""
	}
	if faint {
		return " " + m.theme.StyleFaint.Render(common.WorktreeIcon())
	}
	c := m.theme.WorktreeColorFor(issue.Worktree, m.worktreeNames)
	return " " + lipgloss.NewStyle().Foreground(c).Render(common.WorktreeIcon())
}

// wrapTitle splits a title across the two lines a card gives it, breaking on a
// word boundary where there is one. Widths are measured in terminal cells, so
// wide glyphs (CJK, emoji) do not push the text past the card border.
func wrapTitle(title string, innerW int) (first, second string) {
	if innerW <= 0 {
		return "", ""
	}
	if ansi.StringWidth(title) <= innerW {
		return title, ""
	}
	head := ansi.Truncate(title, innerW, "")
	rest := title[len(head):]
	if i := strings.LastIndexByte(head, ' '); i > 0 {
		head, rest = title[:i], title[i:]
	}
	return head, truncate(strings.TrimPrefix(rest, " "), innerW)
}

// visibleColWidth computes the number of visible columns (after narrowing for
// minimum width) and the content width of each column. Inter-column gaps are one
// character each, except after the last column.
func (m Model) visibleColWidth() (visible, colWidth int) {
	visible = m.visCols
	if visible > len(m.columns)-m.scrollCol {
		visible = len(m.columns) - m.scrollCol
	}
	if visible <= 0 || m.width <= 0 {
		return 0, 0
	}
	totalGaps := visible - 1
	for visible > 1 && (m.width-totalGaps)/visible < minColWidth {
		visible--
		totalGaps = visible - 1
	}
	totalGaps = visible - 1
	colWidth = (m.width - totalGaps) / visible
	if colWidth < minColWidth {
		colWidth = minColWidth
	}
	return visible, colWidth
}

// columnAt maps a screen x coordinate to a visible column index.
func (m Model) columnAt(x int) (colIdx int, ok bool) {
	if len(m.columns) == 0 || m.width == 0 {
		return 0, false
	}
	visible, colWidth := m.visibleColWidth()
	renderWidth := colWidth + 1 // column width + inter-column gap
	ci := x/renderWidth + m.scrollCol
	if ci < m.scrollCol || ci >= m.scrollCol+visible || ci >= len(m.columns) {
		return 0, false
	}
	return ci, true
}

// cardAt maps a screen (x, y) coordinate to a column and row index.
// Returns ok=false if the click didn't land on a card.
func (m Model) cardAt(x, y int) (colIdx, rowIdx int, ok bool) {
	if len(m.columns) == 0 || m.width == 0 {
		return 0, 0, false
	}
	visible, colWidth := m.visibleColWidth()
	renderWidth := colWidth + 1 // column width + inter-column gap

	ci := x/renderWidth + m.scrollCol
	if ci < m.scrollCol || ci >= m.scrollCol+visible || ci >= len(m.columns) {
		return 0, 0, false
	}
	col := m.columns[ci]
	if len(col.issues) == 0 {
		return 0, 0, false
	}

	// Skip lines above content (app header + filter bar) + the column header.
	totalSkip := m.topOffset + colHeaderHeight
	if y < totalSkip {
		return 0, 0, false
	}

	// Scroll offset for this column (active column may be scrolled).
	scrollOff := 0
	if ci == m.curCol {
		scrollOff = m.scrollRow
	}

	yOffset := y - totalSkip
	// "↑ N more" indicator occupies the first line when scrolled.
	if scrollOff > 0 {
		if yOffset == 0 {
			return 0, 0, false
		}
		yOffset--
	}

	ri := yOffset/cardHeight + scrollOff
	// Clamp to visible window — reject clicks on "more" indicators.
	maxCards := m.maxVisibleCards()
	endIdx := scrollOff + maxCards
	if endIdx > len(col.issues) {
		endIdx = len(col.issues)
	}
	if ri < scrollOff || ri >= endIdx {
		return 0, 0, false
	}
	return ci, ri, true
}

// moreIndicatorAt checks if screen (x, y) lands on a "+N more" (bottom)
// or "↑ N more" (top) indicator. Returns the column index and direction:
// +1 for bottom (scroll down), -1 for top (scroll up), 0 for neither.
func (m Model) moreIndicatorAt(x, y int) (colIdx, dir int) {
	if len(m.columns) == 0 || m.width == 0 {
		return 0, 0
	}
	visible, colWidth := m.visibleColWidth()
	renderWidth := colWidth + 1

	ci := x/renderWidth + m.scrollCol
	if ci < m.scrollCol || ci >= m.scrollCol+visible || ci >= len(m.columns) {
		return 0, 0
	}
	col := m.columns[ci]
	if len(col.issues) == 0 {
		return 0, 0
	}

	scrollOff := 0
	if ci == m.curCol {
		scrollOff = m.scrollRow
	}

	totalSkip := m.topOffset + colHeaderHeight
	if y < totalSkip {
		return 0, 0
	}

	yOffset := y - totalSkip

	// "↑ N more" at the top when scrolled.
	if scrollOff > 0 && yOffset == 0 {
		return ci, -1
	}

	// "+N more" at the bottom: check if click is past the last visible card.
	maxCards := m.maxVisibleCards()
	endIdx := scrollOff + maxCards
	if endIdx < len(col.issues) {
		// Adjust yOffset for the top indicator line.
		adj := yOffset
		if scrollOff > 0 {
			adj--
		}
		visibleCards := endIdx - scrollOff
		if adj >= visibleCards*cardHeight {
			return ci, 1
		}
	}

	return 0, 0
}

func (m Model) filteredIssues() []data.Issue {
	query := m.filter.Value()
	if strings.TrimSpace(query) == "" {
		return m.allIssues
	}
	var out []data.Issue
	for _, iss := range m.allIssues {
		if data.MatchesQuery(iss, query) {
			out = append(out, iss)
		}
	}
	return out
}

func groupByStatus(issues []data.Issue, statusFilter []data.Status, hideEmpty bool) []column {
	byStatus := make(map[data.Status][]data.Issue)
	for _, iss := range issues {
		byStatus[iss.Status] = append(byStatus[iss.Status], iss)
	}

	// When a status filter is active, only show those columns
	statuses := data.AllStatuses
	if len(statusFilter) > 0 {
		statuses = statusFilter
	}

	var cols []column
	for _, s := range statuses {
		if hideEmpty && len(byStatus[s]) == 0 {
			continue
		}
		cols = append(cols, column{
			status: s,
			issues: byStatus[s],
		})
	}
	return cols
}

// truncate shortens s to maxWidth terminal cells, marking the cut with "...".
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return ansi.Truncate(s, maxWidth, "")
	}
	return ansi.Truncate(s, maxWidth, "...")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// findIssue looks up an issue by ID across all columns.
func (m Model) findIssue(id int) (data.Issue, bool) {
	for _, col := range m.columns {
		for _, iss := range col.issues {
			if iss.ID == id {
				return iss, true
			}
		}
	}
	return data.Issue{}, false
}

// overlayAt composites a small fg box on top of bg at position (x, y).
func (m Model) overlayAt(bg, fg string, x, y int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	// Offset so the card appears below and to the right of the cursor
	startX := x + 1
	startY := y + 1

	// Measure fg width
	fgWidth := 0
	for _, line := range fgLines {
		if w := lipgloss.Width(line); w > fgWidth {
			fgWidth = w
		}
	}

	// Clamp to stay within bounds
	if startX+fgWidth > m.width {
		startX = m.width - fgWidth
	}
	if startX < 0 {
		startX = 0
	}
	if startY+len(fgLines) > len(bgLines) {
		startY = len(bgLines) - len(fgLines)
	}
	if startY < 0 {
		startY = 0
	}

	for i, fgLine := range fgLines {
		row := startY + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bgLine := bgLines[row]

		// Left portion
		left := ansi.Truncate(bgLine, startX, "")
		leftW := lipgloss.Width(left)
		if leftW < startX {
			left += strings.Repeat(" ", startX-leftW)
		}

		// Right portion
		rightStart := startX + fgWidth
		right := ansi.TruncateLeft(bgLine, rightStart, "")

		bgLines[row] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
