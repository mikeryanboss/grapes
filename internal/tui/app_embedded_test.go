package tui

import (
	"os"
	"path/filepath"
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

// With vineyard installed, standalone Grapes offers the sessions key and hands
// the issue to vineyard, the program that runs agent sessions.
func TestStandalone_SessionsRunVineyard(t *testing.T) {
	repo := t.TempDir()
	issuesDir := filepath.Join(repo, ".grapes")
	vineyard := filepath.Join(t.TempDir(), "vineyard")
	script := "#!/bin/sh\necho \"$PWD $*\" > " + filepath.Join(repo, "ran") +
		"\necho starting >&2\necho 'Error: another vineyard is already running' >&2\nexit 1\n"
	if err := os.WriteFile(vineyard, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ws := data.Workspace{Issues: testutil.SampleIssues()}
	m := NewModel(ws, nil, issuesDir, config.Defaults(), "test").WithVineyard(vineyard)
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = resized.(Model)

	if !strings.Contains(view(t, m), "sessions") {
		t.Error("standalone status bar should advertise the sessions key when vineyard is installed")
	}
	if _, cmd := m.Update(common.SessionsMsg{IssueID: 1}); cmd == nil {
		t.Fatal("SessionsMsg returned no command")
	}

	c, finished := m.vineyardCommand(1)
	if c.Stdin != os.Stdin || c.Stdout != os.Stdout {
		t.Error("vineyard must get grapes' standard streams; tmux refuses /dev/tty")
	}
	msg := finished(c.Run())
	ran, err := os.ReadFile(filepath.Join(repo, "ran"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(ran)), repo+" --issue 1"; got != want {
		t.Errorf("vineyard ran as %q, want %q", got, want)
	}
	updated, _ := m.Update(msg)
	if !strings.Contains(view(t, updated.(Model)), "Vineyard: another vineyard is already running") {
		t.Errorf("status bar should report vineyard's last error line:\n%s", view(t, updated.(Model)))
	}
}
