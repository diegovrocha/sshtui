package menu

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/diegovrocha/sshtui/internal/generate"
	"github.com/diegovrocha/sshtui/internal/history"
	"github.com/diegovrocha/sshtui/internal/inspect"
	"github.com/diegovrocha/sshtui/internal/ui"
	"github.com/diegovrocha/sshtui/internal/update"
)

type menuItem struct {
	label       string
	desc        string
	action      string
	isSeparator bool
}

type screen int

const (
	screenMenu screen = iota
	screenSub
)

type Model struct {
	items    []menuItem
	cursor   int
	screen   screen
	sub      tea.Model
	width    int
	height   int
	quitting bool

	// Fuzzy filter state (main menu only)
	filterMode bool
	filterText string

	// Contextual help overlay
	showHelp bool
}

func New() Model {
	items := []menuItem{
		{label: "── INSPECT ──────────────────────────────────────", isSeparator: true},
		{label: "SSH key", desc: "public/private key details", action: "inspect_key"},
		{label: "SSH cert", desc: "principals, validity, signing CA", action: "inspect_cert"},
		{label: "── GENERATE ─────────────────────────────────────", isSeparator: true},
		{label: "SSH key", desc: "new Ed25519/RSA/ECDSA keypair", action: "gen_key"},
		{label: "─────────────────────────────────────────────────", isSeparator: true},
		{label: "History", desc: "view recent operations log", action: "history"},
		{label: "Update", desc: "download and install the latest version", action: "update"},
		{label: "Quit", action: "quit"},
	}

	m := Model{items: items, cursor: 1}
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.screen == screenSub {
			return m.updateSub(msg)
		}
		return m.updateMenu(msg)
	}

	if m.screen == screenSub && m.sub != nil {
		var cmd tea.Cmd
		m.sub, cmd = m.sub.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Help overlay toggle (only when not typing in filter mode)
	if !m.filterMode {
		if key == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			if key == "esc" {
				m.showHelp = false
				return m, nil
			}
			// Swallow all other keys while help is open
			return m, nil
		}
	}

	if m.filterMode {
		switch key {
		case "esc":
			m.filterMode = false
			m.filterText = ""
			m.resetCursor()
			return m, nil
		case "enter":
			visible := m.visibleIndices()
			if len(visible) == 0 {
				return m, nil
			}
			action := m.items[m.cursor].action
			if action == "" || m.items[m.cursor].isSeparator {
				return m, nil
			}
			return m.handleAction(action)
		case "up":
			m.moveCursorFiltered(-1)
			return m, nil
		case "down":
			m.moveCursorFiltered(1)
			return m, nil
		case "backspace":
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
			}
			m.snapCursorToFirstMatch()
			return m, nil
		case "/":
			// Toggle off only when filter is empty.
			if m.filterText == "" {
				m.filterMode = false
				m.resetCursor()
			}
			return m, nil
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case " ", "space":
			m.filterText += " "
			m.snapCursorToFirstMatch()
			return m, nil
		}
		// Accept printable single-rune keys as filter input.
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
				m.filterText += string(r)
				m.snapCursorToFirstMatch()
				return m, nil
			}
		}
		return m, nil
	}

	switch key {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "/":
		m.filterMode = true
		m.filterText = ""
		m.snapCursorToFirstMatch()
		return m, nil
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		action := m.items[m.cursor].action
		return m.handleAction(action)
	}
	return m, nil
}

func (m Model) updateSub(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.sub = nil
		return m, nil
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	if m.sub != nil {
		var cmd tea.Cmd
		m.sub, cmd = m.sub.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	n := len(m.items)
	for i := 0; i < n; i++ {
		m.cursor = (m.cursor + delta + n) % n
		if !m.items[m.cursor].isSeparator {
			break
		}
	}
}

// moveCursorFiltered moves the cursor within currently matching items.
func (m *Model) moveCursorFiltered(delta int) {
	visible := m.visibleIndices()
	if len(visible) == 0 {
		return
	}
	// Find current position in visible list.
	pos := 0
	for i, idx := range visible {
		if idx == m.cursor {
			pos = i
			break
		}
	}
	pos = (pos + delta + len(visible)) % len(visible)
	m.cursor = visible[pos]
}

// resetCursor points cursor at the first non-separator item.
func (m *Model) resetCursor() {
	for i, it := range m.items {
		if !it.isSeparator {
			m.cursor = i
			return
		}
	}
}

// snapCursorToFirstMatch snaps the cursor to the first filter-matching item.
func (m *Model) snapCursorToFirstMatch() {
	visible := m.visibleIndices()
	if len(visible) > 0 {
		m.cursor = visible[0]
	}
}

// visibleIndices returns the indices of items that currently match the filter
// (or all selectable items when filter is empty / inactive).
func (m Model) visibleIndices() []int {
	var out []int
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	for i, it := range m.items {
		if it.isSeparator {
			continue
		}
		if !m.filterMode || q == "" {
			out = append(out, i)
			continue
		}
		hay := strings.ToLower(it.label + " " + it.desc)
		if strings.Contains(hay, q) {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) handleAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "quit":
		m.quitting = true
		return m, tea.Quit
	case "inspect_key":
		m.screen = screenSub
		m.sub = inspect.NewKey()
		return m, m.sub.Init()
	case "inspect_cert":
		m.screen = screenSub
		m.sub = inspect.NewCert()
		return m, m.sub.Init()
	case "gen_key":
		m.screen = screenSub
		m.sub = generate.NewKey()
		return m, m.sub.Init()
	case "history":
		m.screen = screenSub
		m.sub = history.NewView()
		return m, m.sub.Init()
	case "update":
		m.screen = screenSub
		m.sub = update.New()
		return m, m.sub.Init()
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "\n  " + ui.SuccessStyle.Render("Goodbye!") + "\n\n"
	}

	if m.screen == screenSub && m.sub != nil {
		return m.sub.View()
	}

	if m.showHelp {
		return m.renderHelp()
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.Banner())
	b.WriteString("  " + ui.DimStyle.Render(sshKeygenVersion()) + "\n")
	b.WriteString("\n")

	filtering := m.filterMode && strings.TrimSpace(m.filterText) != ""
	visibleSet := map[int]bool{}
	matches := 0
	if filtering {
		for _, idx := range m.visibleIndices() {
			visibleSet[idx] = true
			matches++
		}
	}

	// Build menu lines
	for i, item := range m.items {
		if item.isSeparator {
			if filtering {
				// Hide separators while filtering.
				continue
			}
			b.WriteString(fmt.Sprintf("  %s\n", ui.SeparatorStyle.Render(item.label)))
			continue
		}
		if filtering && !visibleSet[i] {
			continue
		}

		cursor := "  "
		labelStyle := ui.InactiveStyle
		if m.cursor == i {
			cursor = ui.ActiveStyle.Render("➤ ")
			labelStyle = ui.ActiveStyle
		}

		label := labelStyle.Render(fmt.Sprintf("%-20s", item.label))
		desc := ui.DescStyle.Render(item.desc)
		b.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, label, desc))
	}

	if m.filterMode {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s %s_  %s\n",
			ui.ActiveStyle.Render("Filter:"),
			m.filterText,
			ui.DimStyle.Render("(esc to clear)")))
		b.WriteString(fmt.Sprintf("  %s\n", ui.DimStyle.Render(fmt.Sprintf("Matches: %d", matches))))
		b.WriteString("\n  " + ui.DimStyle.Render("↑/↓ navigate  enter select  backspace delete  esc clear  ctrl+c quit") + "\n")
	} else {
		b.WriteString("\n  " + ui.DimStyle.Render("? help  ↑/↓ navigate  enter select  / filter  q / ctrl+c quit") + "\n")
	}

	return b.String()
}

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.Banner())
	b.WriteString("  " + ui.TitleStyle.Render("── Main Menu ──") + "\n")
	sections := []ui.HelpSection{
		{
			Title: "Navigation",
			Entries: []ui.HelpEntry{
				{Key: "↑/↓ or j/k", Desc: "Navigate menu items"},
				{Key: "enter", Desc: "Select highlighted item"},
				{Key: "q", Desc: "Quit sshtui"},
			},
		},
		{
			Title: "Search",
			Entries: []ui.HelpEntry{
				{Key: "/", Desc: "Fuzzy filter menu items"},
				{Key: "esc", Desc: "Clear filter"},
			},
		},
		ui.CommonHelp(),
	}
	b.WriteString(ui.RenderHelp("Main Menu — Help", sections))
	return b.String()
}

// sshKeygenVersion returns the first line of `ssh -V` output (which goes to
// stderr) or falls back to `ssh-keygen` help output. Returns an empty string
// if neither is available.
func sshKeygenVersion() string {
	// `ssh -V` writes to stderr.
	if out, err := exec.Command("ssh", "-V").CombinedOutput(); err == nil {
		if line := firstLine(string(out)); line != "" {
			return line
		}
	}
	// Fallback: `ssh-keygen` with no args prints usage to stderr.
	if out, err := exec.Command("ssh-keygen", "-?").CombinedOutput(); err == nil {
		if line := firstLine(string(out)); line != "" {
			return line
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
