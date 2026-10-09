package embedded_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Mibokess/grapes/embedded"
)

// repo is a git repository with a .grapes directory, built per test.
type repo struct {
	t    *testing.T
	root string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	r := &repo{t: t, root: t.TempDir()}
	r.git(r.root, "init", "-q", "-b", "main")
	r.git(r.root, "config", "user.email", "test@example.com")
	r.git(r.root, "config", "user.name", "Test")
	return r
}

func (r *repo) git(dir string, args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *repo) writeIssue(checkout string, id int, title string) {
	r.t.Helper()
	dir := filepath.Join(checkout, ".grapes", fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.t.Fatal(err)
	}
	meta := fmt.Sprintf("title = %q\nstatus = 'todo'\npriority = 'medium'\n"+
		"created = 2026-01-01T00:00:00Z\nupdated = 2026-01-01T00:00:00Z\n", title)
	if err := os.WriteFile(filepath.Join(dir, "meta.toml"), []byte(meta), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *repo) commit() {
	r.t.Helper()
	r.git(r.root, "add", "-A")
	r.git(r.root, "commit", "-q", "-m", "issues")
}

func (r *repo) load() embedded.Model {
	r.t.Helper()
	m, err := embedded.New(filepath.Join(r.root, ".grapes"))
	if err != nil {
		r.t.Fatal(err)
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func press(m embedded.Model, k string) (embedded.Model, []tea.Msg) {
	m, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: []rune(k)[0], Text: k}))
	return m, run(cmd)
}

// run executes cmd and the commands of any batch it returns.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, run(c)...)
	}
	return msgs
}

func find[T any](msgs []tea.Msg) (T, bool) {
	for _, msg := range msgs {
		if found, ok := msg.(T); ok {
			return found, true
		}
	}
	var zero T
	return zero, false
}

// Quitting an embedded Grapes must hand control back to the host, never end
// the host program.
func TestQuitSendsCloseInsteadOfQuitting(t *testing.T) {
	r := newRepo(t)
	r.writeIssue(r.root, 1, "first")
	r.commit()
	m := r.load()

	_, msgs := press(m, "q")

	if _, ok := find[tea.QuitMsg](msgs); ok {
		t.Fatal("quit ended the host program")
	}
	if _, ok := find[embedded.CloseMsg](msgs); !ok {
		t.Fatalf("quit sent %#v, want CloseMsg", msgs)
	}
}

func TestSessionsKeyNamesTheSelectedIssue(t *testing.T) {
	r := newRepo(t)
	r.writeIssue(r.root, 7, "embed grapes")
	r.commit()
	m := r.load()

	if view := m.View(); !strings.Contains(view, "sessions") || !strings.Contains(view, "back") {
		t.Errorf("status bar should offer sessions and back:\n%s", view)
	}

	_, msgs := press(m, "a")
	if got, ok := find[embedded.SessionsMsg](msgs); !ok || got.IssueID != 7 {
		t.Errorf("a on the board sent %#v, want SessionsMsg for #7", msgs)
	}

	m, _ = m.OpenIssue(7)
	if view := m.View(); !strings.Contains(view, "embed grapes") {
		t.Errorf("OpenIssue(7) should show the issue:\n%s", view)
	}
	_, msgs = press(m, "a")
	if got, ok := find[embedded.SessionsMsg](msgs); !ok || got.IssueID != 7 {
		t.Errorf("a on the detail screen sent %#v, want SessionsMsg for #7", msgs)
	}
}

func TestIssue(t *testing.T) {
	r := newRepo(t)
	r.writeIssue(r.root, 3, "third")
	r.commit()
	m := r.load()

	got, ok := m.Issue(3)
	want := embedded.Issue{ID: 3, Title: "third", Status: "todo"}
	if !ok || got != want {
		t.Errorf("Issue(3) = %+v, %v; want %+v", got, ok, want)
	}
	if _, ok := m.Issue(99); ok {
		t.Error("Issue(99) found an issue that does not exist")
	}
}

// A host maps its worktrees to issues with TouchedIssues: only the issues the
// worktree's branch changed count, uncommitted changes included.
func TestTouchedIssues(t *testing.T) {
	r := newRepo(t)
	r.writeIssue(r.root, 1, "one")
	r.writeIssue(r.root, 2, "two")
	r.writeIssue(r.root, 3, "three")
	r.commit()
	worktree := filepath.Join(t.TempDir(), "agent")
	r.git(r.root, "worktree", "add", "-q", "-b", "agent", worktree)
	r.writeIssue(worktree, 3, "three, being worked on")
	r.writeIssue(worktree, 2, "two, being worked on")

	m := r.load()

	if got := m.TouchedIssues(worktree); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Errorf("TouchedIssues(worktree) = %v, want [2 3]", got)
	}
	if got := m.TouchedIssues(worktree + "/"); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Errorf("a trailing slash changed the answer: %v", got)
	}
	if got := m.TouchedIssues(r.root); got != nil {
		t.Errorf("TouchedIssues(main checkout) = %v, want none", got)
	}
}
