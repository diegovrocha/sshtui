package inspect

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/diegovrocha/sshtui/internal/history"
	"github.com/diegovrocha/sshtui/internal/ui"
)

type certStep int

const (
	stepCertFile certStep = iota
	stepCertPassphrase
	stepCertResult
)

// certInfo holds parsed data from `ssh-keygen -L -f <file>`.
type certInfo struct {
	File            string
	Type            string // e.g. "ssh-ed25519-cert-v01@openssh.com user certificate"
	CertKind        string // "user" or "host"
	PublicKeyAlg    string // e.g. "ED25519-CERT"
	PublicKeyFP     string // e.g. "SHA256:abc..."
	SigningCA       string // raw "Signing CA" line value
	SigningCAAlg    string // algorithm extracted from "(using ssh-ed25519)"
	SigningCAFP     string // CA fingerprint, e.g. "SHA256:ca-fp..."
	KeyID           string
	Serial          string
	ValidFrom       string
	ValidUntil      string
	ValidUntilTime  time.Time
	ValidParsed     bool
	Principals      []string
	CriticalOptions []string
	Extensions      []string
}

type certModel struct {
	step     certStep
	picker   ui.FilePicker
	file     string
	info     certInfo
	err      string
	raw      string
	logged   bool
	showHelp bool

	copyMsg     string
	copyExpires time.Time

	saving     bool
	saveIn     textinput.Model
	saveResult string
	saveOk     bool
}

// NewCert returns a Bubble Tea model that inspects SSH certificates starting
// at the file picker.
func NewCert() tea.Model {
	return &certModel{
		step:   stepCertFile,
		picker: ui.NewSSHCertPicker("Select SSH certificate"),
	}
}

// NewCertWithFile returns a Bubble Tea model pre-loaded with the given cert
// file, skipping the file-picker step.
func NewCertWithFile(path string) tea.Model {
	return &certModel{
		step: stepCertResult,
		file: path,
	}
}

type certParsedMsg struct {
	info certInfo
	raw  string
	err  string
}

type certCopyClearMsg struct{}

type certSaveResultMsg struct {
	ok      bool
	message string
}

func (m *certModel) Init() tea.Cmd {
	if m.step == stepCertResult && m.file != "" {
		return m.doInspect()
	}
	return textinput.Blink
}

func (m *certModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case certParsedMsg:
		m.info = msg.info
		m.raw = msg.raw
		m.err = msg.err
		m.step = stepCertResult
		if msg.err == "" && !m.logged {
			history.Log("inspect_ssh_cert",
				history.KV("file", m.file),
				history.KV("key_id", m.info.KeyID),
				history.KV("principals", strings.Join(m.info.Principals, ",")),
				history.KV("valid_until", m.info.ValidUntil))
			m.logged = true
		}
		return m, nil

	case certCopyClearMsg:
		if !time.Now().Before(m.copyExpires) {
			m.copyMsg = ""
		}
		return m, nil

	case certSaveResultMsg:
		m.saving = false
		m.saveOk = msg.ok
		m.saveResult = msg.message
		return m, nil

	case tea.KeyMsg:
		// Help overlay handling
		if m.step == stepCertResult && !m.saving {
			if msg.String() == "?" {
				m.showHelp = !m.showHelp
				return m, nil
			}
			if m.showHelp {
				if msg.String() == "esc" {
					m.showHelp = false
				}
				return m, nil
			}
		}

		if msg.String() == "esc" {
			if m.saving {
				m.saving = false
				return m, nil
			}
			if m.step == stepCertResult {
				m.step = stepCertFile
				m.info = certInfo{}
				m.err = ""
				m.raw = ""
				m.logged = false
				m.saveResult = ""
				m.copyMsg = ""
				m.picker = ui.NewSSHCertPicker("Select SSH certificate")
				return m, nil
			}
			return m, nil
		}

		if m.step == stepCertResult {
			if m.saving {
				if msg.String() == "enter" {
					name := strings.TrimSpace(m.saveIn.Value())
					if name == "" {
						name = defaultCertSaveName(m.info)
					}
					return m, m.doSave(name)
				}
				var cmd tea.Cmd
				m.saveIn, cmd = m.saveIn.Update(msg)
				return m, cmd
			}
			switch msg.String() {
			case "y", "Y":
				return m, m.doCopy()
			case "s", "S":
				m.saving = true
				m.saveResult = ""
				m.saveIn = textinput.New()
				m.saveIn.Placeholder = defaultCertSaveName(m.info)
				m.saveIn.SetValue(defaultCertSaveName(m.info))
				m.saveIn.Focus()
				return m, m.saveIn.Focus()
			case "n", "N":
				m.step = stepCertFile
				m.info = certInfo{}
				m.err = ""
				m.raw = ""
				m.logged = false
				m.saveResult = ""
				m.copyMsg = ""
				m.picker = ui.NewSSHCertPicker("Select SSH certificate")
				return m, nil
			}
			return m, nil
		}
	}

	switch m.step {
	case stepCertFile:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		if m.picker.Done {
			m.file = m.picker.Selected
			return m, m.doInspect()
		}
		return m, cmd
	}

	return m, nil
}

func (m *certModel) doInspect() tea.Cmd {
	file := m.file
	return func() tea.Msg {
		out, err := exec.Command("ssh-keygen", "-L", "-f", file).CombinedOutput()
		if err != nil {
			return certParsedMsg{err: "ssh-keygen failed: " + strings.TrimSpace(string(out))}
		}
		info, perr := parseKeygenL(string(out))
		if perr != nil {
			return certParsedMsg{raw: string(out), err: "Could not parse certificate: " + perr.Error()}
		}
		info.File = file
		return certParsedMsg{info: info, raw: string(out)}
	}
}

func (m *certModel) doCopy() tea.Cmd {
	text := formatCertInfo(m.info, false)
	m.copyExpires = time.Now().Add(2 * time.Second)
	if err := clipboard.WriteAll(text); err != nil {
		m.copyMsg = "✖ Copy failed: " + err.Error()
	} else {
		m.copyMsg = "✔ Copied to clipboard"
	}
	return tea.Tick(2100*time.Millisecond, func(time.Time) tea.Msg {
		return certCopyClearMsg{}
	})
}

func (m *certModel) doSave(name string) tea.Cmd {
	text := formatCertInfo(m.info, false)
	return func() tea.Msg {
		if err := os.WriteFile(name, []byte(text), 0644); err != nil {
			return certSaveResultMsg{ok: false, message: "Could not save: " + err.Error()}
		}
		return certSaveResultMsg{ok: true, message: "File: " + name}
	}
}

func defaultCertSaveName(info certInfo) string {
	base := info.KeyID
	if base == "" {
		base = "ssh_certificate"
	}
	r := strings.NewReplacer(
		"*", "_", " ", "_", "/", "_", "\\", "_",
		":", "_", "?", "_", "\"", "_", "<", "_",
		">", "_", "|", "_", "@", "_at_",
	)
	return strings.Trim(r.Replace(base), "._") + ".txt"
}

func (m *certModel) View() string {
	if m.showHelp {
		return m.renderHelp()
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.Banner())
	b.WriteString("\n  " + ui.TitleStyle.Render("── Inspect SSH Certificate ──") + "\n\n")

	switch m.step {
	case stepCertFile:
		b.WriteString(m.picker.View())
		b.WriteString("\n  " + ui.DimStyle.Render("esc back  ↑/↓ navigate  enter confirm  ctrl+c quit") + "\n")

	case stepCertPassphrase:
		b.WriteString("  " + ui.DimStyle.Render("(unused)") + "\n")

	case stepCertResult:
		if m.err != "" {
			b.WriteString(ui.ResultBox(false, "Error", m.err))
			b.WriteString("\n")
			if m.raw != "" {
				b.WriteString("\n  " + ui.DimStyle.Render("ssh-keygen output:") + "\n")
				for _, line := range strings.Split(m.raw, "\n") {
					b.WriteString("    " + line + "\n")
				}
			}
		} else if m.saving {
			b.WriteString("  Save certificate details to file:\n\n")
			b.WriteString("  " + m.saveIn.View() + "\n")
		} else {
			b.WriteString(renderCertBox(m.info))
		}

		// Transient copy message
		if m.copyMsg != "" && time.Now().Before(m.copyExpires) {
			b.WriteString("\n  " + ui.SuccessStyle.Render(m.copyMsg) + "\n")
		}
		if m.saveResult != "" {
			b.WriteString("\n")
			if m.saveOk {
				b.WriteString(ui.ResultBox(true, "Saved", m.saveResult))
			} else {
				b.WriteString(ui.ResultBox(false, "Error", m.saveResult))
			}
			b.WriteString("\n")
		}

		if m.saving {
			b.WriteString("\n  " + ui.DimStyle.Render("enter save  esc cancel  ctrl+c quit") + "\n")
		} else {
			b.WriteString("\n  " + ui.DimStyle.Render("? help  y copy  s save  n inspect another  esc back  ctrl+c quit") + "\n")
		}
	}

	return b.String()
}

func (m *certModel) renderHelp() string {
	sections := []ui.HelpSection{
		{
			Title: "Result view",
			Entries: []ui.HelpEntry{
				{Key: "y", Desc: "Copy to clipboard"},
				{Key: "s", Desc: "Save to file"},
				{Key: "n", Desc: "Inspect another certificate"},
			},
		},
		{
			Title: "File picker",
			Entries: []ui.HelpEntry{
				{Key: "↑/↓", Desc: "Navigate entries"},
				{Key: "→ / enter", Desc: "Open folder"},
				{Key: "←", Desc: "Parent folder"},
				{Key: "type", Desc: "Filter entries"},
			},
		},
		ui.CommonHelp(),
	}
	return "\n" + ui.Banner() + "  " + ui.TitleStyle.Render("── Inspect SSH Certificate ──") + "\n" +
		ui.RenderHelp("Inspect SSH Certificate — Help", sections)
}

func renderCertBox(c certInfo) string {
	var b strings.Builder

	title := "SSH Certificate"
	if c.CertKind != "" {
		title = "SSH " + strings.Title(c.CertKind) + " Certificate"
	}
	b.WriteString(fmt.Sprintf("  %s\n\n", ui.TitleStyle.Render(title)))

	field := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(fmt.Sprintf("    %-18s %s\n", label+":", value))
	}

	field("File", c.File)
	field("Type", c.Type)
	if c.PublicKeyAlg != "" || c.PublicKeyFP != "" {
		field("Public key", strings.TrimSpace(c.PublicKeyAlg+" "+c.PublicKeyFP))
	}
	if c.SigningCA != "" {
		ca := c.SigningCA
		if c.SigningCAAlg != "" && !strings.Contains(ca, "(using") {
			ca += " (using " + c.SigningCAAlg + ")"
		}
		field("Signing CA", ca)
	}
	field("Key ID", c.KeyID)
	field("Serial", c.Serial)
	field("Valid from", c.ValidFrom)
	field("Valid until", c.ValidUntil)

	// Days remaining — color based on time to expiry.
	if c.ValidParsed {
		remaining := time.Until(c.ValidUntilTime)
		days := int(remaining.Hours() / 24)
		var line string
		switch {
		case remaining <= 0:
			line = ui.ErrorStyle.Render(fmt.Sprintf("✖ Expired (%d days ago)", -days))
		case days <= 30:
			line = ui.WarnStyle.Render(fmt.Sprintf("⚠ Expires in %d days", days))
		default:
			line = ui.SuccessStyle.Render(fmt.Sprintf("✔ %d days remaining", days))
		}
		b.WriteString(fmt.Sprintf("    %-18s %s\n", "Days remaining:", line))
	}

	renderList := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(fmt.Sprintf("    %-18s %s\n", label+":", items[0]))
		for _, it := range items[1:] {
			b.WriteString(fmt.Sprintf("    %-18s %s\n", "", it))
		}
	}

	b.WriteString("\n")
	renderList("Principals", c.Principals)
	renderList("Critical Options", c.CriticalOptions)
	renderList("Extensions", c.Extensions)

	return b.String()
}

// formatCertInfo produces a plain-text (no ANSI) rendering of the cert info,
// suitable for clipboard or file output.
func formatCertInfo(c certInfo, _ bool) string {
	var b strings.Builder
	b.WriteString("SSH Certificate\n\n")
	line := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(fmt.Sprintf("  %-18s %s\n", label+":", value))
	}
	line("File", c.File)
	line("Type", c.Type)
	if c.PublicKeyAlg != "" || c.PublicKeyFP != "" {
		line("Public key", strings.TrimSpace(c.PublicKeyAlg+" "+c.PublicKeyFP))
	}
	line("Signing CA", c.SigningCA)
	line("Key ID", c.KeyID)
	line("Serial", c.Serial)
	line("Valid from", c.ValidFrom)
	line("Valid until", c.ValidUntil)

	if c.ValidParsed {
		remaining := time.Until(c.ValidUntilTime)
		days := int(remaining.Hours() / 24)
		if remaining <= 0 {
			line("Days remaining", fmt.Sprintf("Expired (%d days ago)", -days))
		} else {
			line("Days remaining", fmt.Sprintf("%d days", days))
		}
	}

	writeList := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(fmt.Sprintf("  %s:\n", label))
		for _, it := range items {
			b.WriteString("    " + it + "\n")
		}
	}
	writeList("Principals", c.Principals)
	writeList("Critical Options", c.CriticalOptions)
	writeList("Extensions", c.Extensions)

	return b.String()
}

// parseKeygenL parses the output of `ssh-keygen -L -f <file>`.
//
// Uses a simple state machine where the current "section" tracks which
// multi-line block we are inside (principals / criticalOptions / extensions).
// Single-line key: value fields are parsed in the "header" section.
func parseKeygenL(output string) (certInfo, error) {
	var info certInfo
	if strings.TrimSpace(output) == "" {
		return info, errors.New("empty output")
	}

	lines := strings.Split(output, "\n")

	const (
		sectionHeader          = "header"
		sectionPrincipals      = "principals"
		sectionCriticalOptions = "criticalOptions"
		sectionExtensions      = "extensions"
	)

	section := sectionHeader
	foundAny := false

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		// The first line of ssh-keygen -L is the filename followed by ":".
		// It is un-indented. Skip it (it has no colon-separated field we care about).
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			// It's the "<file>:" header line — skip it.
			continue
		}

		// Determine indentation depth (used to detect nested list entries).
		trimmed := strings.TrimLeft(raw, " \t")
		indentChars := len(raw) - len(trimmed)

		// Section headers: "Principals:", "Critical Options:", "Extensions:".
		// These appear with a single level of indentation and end with ":".
		// When the value is "(none)" or empty, there are no list entries to
		// follow — treat it as a header row and reset the section so the
		// next indented line is not misclassified.
		if trimmed == "Principals:" {
			section = sectionPrincipals
			foundAny = true
			continue
		}
		if trimmed == "Critical Options:" {
			section = sectionCriticalOptions
			foundAny = true
			continue
		}
		if trimmed == "Extensions:" {
			section = sectionExtensions
			foundAny = true
			continue
		}
		if strings.HasPrefix(trimmed, "Principals:") ||
			strings.HasPrefix(trimmed, "Critical Options:") ||
			strings.HasPrefix(trimmed, "Extensions:") {
			// Section header with an inline "(none)" value — no list follows.
			section = sectionHeader
			foundAny = true
			continue
		}

		// A deeper indentation inside a list section means a list entry.
		// ssh-keygen typically uses 8 spaces / tab for nested list items
		// (vs 4 for header-level fields).
		if section != sectionHeader && indentChars >= 8 {
			switch section {
			case sectionPrincipals:
				info.Principals = append(info.Principals, trimmed)
			case sectionCriticalOptions:
				info.CriticalOptions = append(info.CriticalOptions, trimmed)
			case sectionExtensions:
				info.Extensions = append(info.Extensions, trimmed)
			}
			foundAny = true
			continue
		}

		// Back to header-level fields — we parsed a new "Key: value" row.
		section = sectionHeader

		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		value := strings.TrimSpace(trimmed[idx+1:])
		foundAny = true

		switch key {
		case "Type":
			info.Type = value
			low := strings.ToLower(value)
			switch {
			case strings.Contains(low, "host certificate"):
				info.CertKind = "host"
			case strings.Contains(low, "user certificate"):
				info.CertKind = "user"
			}
		case "Public key":
			// e.g. "ED25519-CERT SHA256:abc..."
			parts := strings.SplitN(value, " ", 2)
			info.PublicKeyAlg = parts[0]
			if len(parts) > 1 {
				info.PublicKeyFP = strings.TrimSpace(parts[1])
			}
		case "Signing CA":
			info.SigningCA = value
			// Extract fingerprint + (using <alg>) if present.
			// e.g. "ED25519 SHA256:ca-fp... (using ssh-ed25519)"
			if i := strings.Index(value, "(using "); i >= 0 {
				end := strings.Index(value[i:], ")")
				if end > 0 {
					info.SigningCAAlg = strings.TrimSpace(value[i+len("(using ") : i+end])
				}
			}
			for _, tok := range strings.Fields(value) {
				if strings.HasPrefix(tok, "SHA256:") || strings.HasPrefix(tok, "MD5:") {
					info.SigningCAFP = tok
					break
				}
			}
		case "Key ID":
			info.KeyID = strings.Trim(value, `"`)
		case "Serial":
			info.Serial = value
		case "Valid":
			// "from YYYY-MM-DDTHH:MM:SS to YYYY-MM-DDTHH:MM:SS"
			// or "forever"
			info.ValidFrom, info.ValidUntil = parseValidRange(value)
			if info.ValidUntil != "" && info.ValidUntil != "forever" {
				if t, err := parseKeygenTime(info.ValidUntil); err == nil {
					info.ValidUntilTime = t
					info.ValidParsed = true
				}
			}
		}
	}

	if !foundAny {
		return info, errors.New("no recognizable fields found")
	}
	if info.Type == "" && info.KeyID == "" && info.Serial == "" {
		return info, errors.New("missing required certificate fields")
	}
	return info, nil
}

// parseValidRange parses strings of the form
// "from 2026-01-01T00:00:00 to 2027-01-01T00:00:00" or "forever".
func parseValidRange(s string) (from, to string) {
	s = strings.TrimSpace(s)
	if s == "forever" {
		return "", "forever"
	}
	// "from <A> to <B>"
	if strings.HasPrefix(s, "from ") {
		rest := strings.TrimPrefix(s, "from ")
		if i := strings.Index(rest, " to "); i >= 0 {
			return strings.TrimSpace(rest[:i]), strings.TrimSpace(rest[i+len(" to "):])
		}
		return strings.TrimSpace(rest), ""
	}
	return "", s
}

func parseKeygenTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %s", s)
}
