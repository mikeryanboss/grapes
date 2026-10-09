package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Mibokess/grapes/internal/config"
	"github.com/Mibokess/grapes/internal/data"
	"github.com/Mibokess/grapes/internal/tui/common"
	"github.com/Mibokess/grapes/internal/tui/testutil"
)

// Standalone Grapes has no host to answer the sessions key, so it neither
// advertises the key nor stops quitting the program.
func TestStandalone_QuitsAndHidesSessions(t *testing.T) {
	ws := data.Workspace{Issues: testutil.SampleIssues()}
	m := NewModel(ws, nil, t.TempDir(), config.Defaults(), "test")
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = resized.(Model)

	if strings.Contains(view(t, m), "sessions") {
		t.Error("standalone status bar advertises the sessions key")
	}
	updated, cmd := m.Update(common.SessionsMsg{IssueID: 1})
	if cmd != nil || updated.(Model).screen != m.screen {
		t.Error("standalone Grapes should ignore SessionsMsg")
	}
	_, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	if cmd == nil {
		t.Fatal("q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit standalone Grapes")
	}
}
