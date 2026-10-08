package detail_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Mibokess/grapes/internal/tui/common"
	"github.com/Mibokess/grapes/internal/tui/detail"
	"github.com/Mibokess/grapes/internal/tui/testutil"
)

func newDetailModel() detail.Model {
	issues := testutil.SampleIssues()
	return detail.New(issues[0], issues, 100, 40, common.NewTheme(true)).SetTopOffset(1)
}

// newShortDetailModel creates a detail view with a small viewport height
// so that scrolling can actually change the view.
func newShortDetailModel() detail.Model {
	issues := testutil.SampleIssues()
	return detail.New(issues[0], issues, 100, 5, common.NewTheme(true)).SetTopOffset(1)
}

func keyMsg(k string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: -2, Text: k})
}

func extractMsg(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// --- Keyboard interactions ---

func TestDetail_KeyEsc_GoesBack(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 27}))
	if _, ok := extractMsg(cmd).(common.GoBackMsg); !ok {
		t.Error("esc should send GoBackMsg")
	}
}

func TestDetail_KeyBackspace_GoesBack(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if _, ok := extractMsg(cmd).(common.GoBackMsg); !ok {
		t.Error("backspace should send GoBackMsg")
	}
}

func TestDetail_KeyShiftB_SwitchesToBoard(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("B"))
	sw, ok := extractMsg(cmd).(common.SwitchScreenMsg)
	if !ok {
		t.Fatal("B should send SwitchScreenMsg")
	}
	if sw.Screen != common.ScreenBoard {
		t.Error("expected switch to board screen")
	}
}

func TestDetail_KeyL_SwitchesToList(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("l"))
	sw, ok := extractMsg(cmd).(common.SwitchScreenMsg)
	if !ok {
		t.Fatal("l should send SwitchScreenMsg")
	}
	if sw.Screen != common.ScreenList {
		t.Error("expected switch to list screen")
	}
}

func TestDetail_KeyS_ShowsStatusPicker(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("s"))
	picker, ok := extractMsg(cmd).(common.ShowPickerMsg)
	if !ok {
		t.Fatal("s should send ShowPickerMsg")
	}
	if picker.Field != "status" {
		t.Errorf("expected field 'status', got %q", picker.Field)
	}
	if picker.IssueID != 1 {
		t.Errorf("expected issue ID 1, got %d", picker.IssueID)
	}
}

func TestDetail_KeyP_ShowsPriorityPicker(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("p"))
	picker, ok := extractMsg(cmd).(common.ShowPickerMsg)
	if !ok {
		t.Fatal("p should send ShowPickerMsg")
	}
	if picker.Field != "priority" {
		t.Errorf("expected field 'priority', got %q", picker.Field)
	}
}

func TestDetail_KeyE_LaunchesEdit(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("e"))
	if _, ok := extractMsg(cmd).(common.LaunchEditMsg); !ok {
		t.Error("e should send LaunchEditMsg")
	}
}

func TestDetail_KeyC_LaunchesComment(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(keyMsg("c"))
	if _, ok := extractMsg(cmd).(common.LaunchEditorMsg); !ok {
		t.Error("c should send LaunchEditorMsg")
	}
}

// --- Mouse interactions ---

func TestDetail_BackwardButton_GoesBack(t *testing.T) {
	m := newDetailModel()
	_, cmd := m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseBackward})
	if _, ok := extractMsg(cmd).(common.GoBackMsg); !ok {
		t.Error("backward button should send GoBackMsg")
	}
}

func TestDetail_MouseWheelDown_Scrolls(t *testing.T) {
	// Use a short viewport so scrolling actually changes visible content
	m := newShortDetailModel()
	view1 := testutil.StripANSI(m.View())
	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	view2 := testutil.StripANSI(m.View())

	if view1 == view2 {
		t.Error("mouse wheel down should scroll the detail view")
	}
}

func TestDetail_MouseWheelUp_AtTop_NoChange(t *testing.T) {
	m := newDetailModel()
	view1 := testutil.StripANSI(m.View())
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	view2 := testutil.StripANSI(m.View())

	if view1 != view2 {
		t.Error("mouse wheel up at top should not change view")
	}
}

func TestDetail_ClickOutsideViewport_NoOp(t *testing.T) {
	m := newDetailModel()
	// Click in the app header area (y=0, which is within topOffset)
	_, cmd := m.Update(tea.MouseClickMsg{X: 5, Y: 0, Button: tea.MouseLeft})
	msg := extractMsg(cmd)
	if msg != nil {
		if _, ok := msg.(common.ShowPickerMsg); ok {
			t.Error("clicking outside viewport should not open a picker")
		}
		if _, ok := msg.(common.OpenDetailMsg); ok {
			t.Error("clicking outside viewport should not navigate")
		}
	}
}

// --- Issue ID ---

func TestDetail_IssueID(t *testing.T) {
	m := newDetailModel()
	if m.IssueID() != 1 {
		t.Errorf("expected issue ID 1, got %d", m.IssueID())
	}
}

// --- Simple issue (no content/comments) ---

func TestDetail_SimpleIssue_KeysWork(t *testing.T) {
	issues := testutil.SampleIssues()
	m := detail.New(issues[2], issues, 100, 30, common.NewTheme(true)).SetTopOffset(1)

	_, cmd := m.Update(keyMsg("s"))
	picker, ok := extractMsg(cmd).(common.ShowPickerMsg)
	if !ok {
		t.Fatal("s should send ShowPickerMsg")
	}
	if picker.IssueID != 3 {
		t.Errorf("expected issue ID 3, got %d", picker.IssueID)
	}
}
