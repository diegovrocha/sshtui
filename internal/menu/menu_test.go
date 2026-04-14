package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewMenu(t *testing.T) {
	m := New()

	if len(m.items) == 0 {
		t.Error("Menu should have items")
	}

	// First selectable item should not be a separator
	if m.items[m.cursor].isSeparator {
		t.Error("Initial cursor should not be on a separator")
	}
}

func TestMoveCursorSkipsSeparators(t *testing.T) {
	m := New()

	initial := m.cursor
	if m.items[initial].isSeparator {
		t.Error("Initial position should not be a separator")
	}

	// Move down
	m.moveCursor(1)
	if m.items[m.cursor].isSeparator {
		t.Error("moveCursor(1) should not stop on a separator")
	}

	// Move up
	m.moveCursor(-1)
	if m.items[m.cursor].isSeparator {
		t.Error("moveCursor(-1) should not stop on a separator")
	}
}

func TestMoveCursorWraps(t *testing.T) {
	m := New()

	for i := 0; i < len(m.items)*2; i++ {
		m.moveCursor(-1)
		if m.items[m.cursor].isSeparator {
			t.Errorf("Cursor stopped on separator at index %d", m.cursor)
		}
	}

	for i := 0; i < len(m.items)*2; i++ {
		m.moveCursor(1)
		if m.items[m.cursor].isSeparator {
			t.Errorf("Cursor stopped on separator at index %d", m.cursor)
		}
	}
}

func TestMenuHasAllActions(t *testing.T) {
	m := New()

	expectedActions := []string{
		"inspect_key", "inspect_cert", "gen_key", "history", "update", "quit",
	}

	actions := make(map[string]bool)
	for _, item := range m.items {
		if item.action != "" {
			actions[item.action] = true
		}
	}

	for _, expected := range expectedActions {
		if !actions[expected] {
			t.Errorf("Menu should have action '%s'", expected)
		}
	}
}

func TestMenuView(t *testing.T) {
	m := New()
	v := m.View()

	if !strings.Contains(v, "INSPECT") {
		t.Error("View should contain INSPECT section")
	}
	if !strings.Contains(v, "GENERATE") {
		t.Error("View should contain GENERATE section")
	}
	if !strings.Contains(v, "➤") {
		t.Error("View should contain cursor ➤")
	}
	if !strings.Contains(v, "Quit") {
		t.Error("View should contain Quit option")
	}
}

func sendKey(m Model, key string) Model {
	var km tea.KeyMsg
	if len(key) == 1 {
		km = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	} else {
		switch key {
		case "esc":
			km = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			km = tea.KeyMsg{Type: tea.KeyBackspace}
		case "enter":
			km = tea.KeyMsg{Type: tea.KeyEnter}
		case "up":
			km = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			km = tea.KeyMsg{Type: tea.KeyDown}
		default:
			km = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
	}
	next, _ := m.Update(km)
	return next.(Model)
}

func TestFuzzyFilterActivation(t *testing.T) {
	m := New()
	if m.filterMode {
		t.Fatal("filterMode should start false")
	}
	m = sendKey(m, "/")
	if !m.filterMode {
		t.Error("after '/', filterMode should be true")
	}
}

func TestFuzzyFilterText(t *testing.T) {
	m := New()
	m = sendKey(m, "/")
	m = sendKey(m, "i")
	m = sendKey(m, "n")
	m = sendKey(m, "s")
	if m.filterText != "ins" {
		t.Errorf("filterText: got %q want %q", m.filterText, "ins")
	}
}

func TestFuzzyFilterBackspace(t *testing.T) {
	m := New()
	m = sendKey(m, "/")
	m = sendKey(m, "a")
	m = sendKey(m, "b")
	m = sendKey(m, "c")
	if m.filterText != "abc" {
		t.Fatalf("setup: got filterText=%q", m.filterText)
	}
	m = sendKey(m, "backspace")
	if m.filterText != "ab" {
		t.Errorf("after backspace: got %q want ab", m.filterText)
	}
}

func TestFuzzyFilterEsc(t *testing.T) {
	m := New()
	m = sendKey(m, "/")
	m = sendKey(m, "x")
	if !m.filterMode || m.filterText == "" {
		t.Fatal("setup: filter should be active with text")
	}
	m = sendKey(m, "esc")
	if m.filterMode {
		t.Error("after esc, filterMode should be false")
	}
	if m.filterText != "" {
		t.Errorf("after esc, filterText should be empty, got %q", m.filterText)
	}
}
