package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/diegovrocha/sshtui/internal/history"
	"github.com/diegovrocha/sshtui/internal/ui"
)

type keyStep int

const (
	stepAlgo keyStep = iota
	stepBits
	stepOut
	stepConfirmOverwrite
	stepComment
	stepPassphrase
	stepPassphrase2
	stepGenerating
	stepDone
)

// Algorithm choices shown on stepAlgo. Order matters — it defines algoValues.
var algoLabels = []string{
	"Ed25519    (recommended — fast, small, modern)",
	"RSA        (traditional, broad compatibility)",
	"ECDSA      (modern NIST curves)",
	"DSA        (legacy, discouraged)",
}

var algoValues = []string{"ed25519", "rsa", "ecdsa", "dsa"}

var rsaBits = []string{"2048", "3072", "4096"}
var ecdsaBits = []string{"256", "384", "521"}

// KeyModel is the Bubble Tea model for ssh-keygen-driven key generation.
type KeyModel struct {
	step       keyStep
	input      textinput.Model
	spin       spinner.Model
	optCur     int
	algo       string
	bits       string // empty for ed25519/dsa
	out        string
	comment    string
	passphrase string
	pass2      string
	result     string
	success    bool
	showHelp   bool
	overwrite  bool // true once user confirmed y on stepConfirmOverwrite
	passErr    string
}

// NewKey returns a Bubble Tea model that walks the user through ssh-keygen.
func NewKey() tea.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &KeyModel{
		step:   stepAlgo,
		optCur: 0, // default ed25519
		spin:   sp,
	}
}

func (m *KeyModel) Init() tea.Cmd { return nil }

type keyGenResult struct {
	success bool
	message string
}

func isKeyChoiceStep(s keyStep) bool {
	return s == stepAlgo || s == stepBits || s == stepConfirmOverwrite
}

func isKeyInputStep(s keyStep) bool {
	return s == stepOut || s == stepComment || s == stepPassphrase || s == stepPassphrase2
}

func (m *KeyModel) choiceMax() int {
	switch m.step {
	case stepAlgo:
		return len(algoLabels) - 1
	case stepBits:
		if m.algo == "rsa" {
			return len(rsaBits) - 1
		}
		if m.algo == "ecdsa" {
			return len(ecdsaBits) - 1
		}
		return 0
	case stepConfirmOverwrite:
		return 1
	}
	return 0
}

func (m *KeyModel) newInput(placeholder, value string, password bool) tea.Cmd {
	m.input = textinput.New()
	m.input.Placeholder = placeholder
	if value != "" {
		m.input.SetValue(value)
	}
	if password {
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
	}
	m.input.Focus()
	return m.input.Focus()
}

func (m *KeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isKeyChoiceStep(m.step) || m.step == stepDone || m.step == stepGenerating {
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
		switch msg.String() {
		case "esc":
			return m, nil
		case "up", "k":
			if isKeyChoiceStep(m.step) && m.optCur > 0 {
				m.optCur--
			}
			return m, nil
		case "down", "j":
			if isKeyChoiceStep(m.step) && m.optCur < m.choiceMax() {
				m.optCur++
			}
			return m, nil
		case "enter":
			return m.advance()
		}

	case keyGenResult:
		m.success = msg.success
		m.result = msg.message
		m.step = stepDone
		if msg.success {
			history.Log("generate_ssh_key",
				history.KV("algo", m.algo),
				history.KV("bits", m.bits),
				history.KV("out", m.out),
				history.KV("comment", m.comment),
				history.KV("encrypted", fmt.Sprintf("%t", m.passphrase != "")))
		}
		return m, nil

	case spinner.TickMsg:
		if m.step == stepGenerating {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	}

	if isKeyInputStep(m.step) {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *KeyModel) advance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepAlgo:
		m.algo = algoValues[m.optCur]
		if m.algo == "rsa" || m.algo == "ecdsa" {
			m.step = stepBits
			m.optCur = 0
			return m, nil
		}
		// ed25519 / dsa — skip bits step
		m.bits = ""
		m.step = stepOut
		return m, m.newInput(defaultOutPath(m.algo), defaultOutPath(m.algo), false)

	case stepBits:
		if m.algo == "rsa" {
			b := rsaBits[m.optCur]
			if !validateAlgoBits(m.algo, b) {
				return m, nil
			}
			m.bits = b
		} else if m.algo == "ecdsa" {
			m.bits = ecdsaBits[m.optCur]
		}
		m.step = stepOut
		return m, m.newInput(defaultOutPath(m.algo), defaultOutPath(m.algo), false)

	case stepOut:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			v = defaultOutPath(m.algo)
		}
		m.out = expandTilde(v)
		if _, err := os.Stat(m.out); err == nil {
			// file exists — confirm
			m.step = stepConfirmOverwrite
			m.optCur = 1 // default No
			return m, nil
		}
		m.step = stepComment
		return m, m.newInput(defaultComment(), defaultComment(), false)

	case stepConfirmOverwrite:
		if m.optCur == 0 {
			// Yes — remove existing files so ssh-keygen doesn't prompt.
			_ = os.Remove(m.out)
			_ = os.Remove(m.out + ".pub")
			m.overwrite = true
			m.step = stepComment
			return m, m.newInput(defaultComment(), defaultComment(), false)
		}
		// No — go back to path input
		m.step = stepOut
		return m, m.newInput(defaultOutPath(m.algo), m.out, false)

	case stepComment:
		c := strings.TrimSpace(m.input.Value())
		if c == "" {
			c = defaultComment()
		}
		m.comment = c
		m.step = stepPassphrase
		return m, m.newInput("(empty for no passphrase)", "", true)

	case stepPassphrase:
		m.passphrase = m.input.Value()
		m.step = stepPassphrase2
		return m, m.newInput("confirm passphrase", "", true)

	case stepPassphrase2:
		m.pass2 = m.input.Value()
		if m.pass2 != m.passphrase {
			m.passErr = "Passphrases do not match — try again."
			m.passphrase = ""
			m.pass2 = ""
			m.step = stepPassphrase
			return m, m.newInput("(empty for no passphrase)", "", true)
		}
		m.passErr = ""
		m.step = stepGenerating
		return m, tea.Batch(m.spin.Tick, m.doGenerate())
	}
	return m, nil
}

// doGenerate runs ssh-keygen with the configured args.
func (m *KeyModel) doGenerate() tea.Cmd {
	return func() tea.Msg {
		// Ensure output directory exists.
		if dir := filepath.Dir(m.out); dir != "" {
			_ = os.MkdirAll(dir, 0700)
		}
		args := buildArgs(m)
		// args[0] is the program name in our builder; split for exec.
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return keyGenResult{false, "ssh-keygen failed: " + err.Error() + "\n" + strings.TrimSpace(string(out))}
		}

		bitsInfo := ""
		if m.bits != "" {
			bitsInfo = fmt.Sprintf(" | bits/curve: %s", m.bits)
		}
		msg := fmt.Sprintf("Private key: %s\nPublic key:  %s.pub\nAlgo: %s%s\nComment: %s",
			m.out, m.out, m.algo, bitsInfo, m.comment)
		return keyGenResult{true, msg}
	}
}

// buildArgs constructs the ssh-keygen command line for the given model.
// Returns a slice where element 0 is the program name ("ssh-keygen") and the
// rest are its arguments. Tests assert the exact shape of this slice.
func buildArgs(m *KeyModel) []string {
	args := []string{"ssh-keygen", "-t", m.algo}
	if m.bits != "" {
		args = append(args, "-b", m.bits)
	}
	args = append(args, "-f", m.out, "-C", m.comment, "-N", m.passphrase)
	return args
}

// validateAlgoBits returns true if the (algo, bits) pair is acceptable.
// For ed25519 and dsa the bits argument is ignored (always true).
// For rsa we require >= 2048.
// For ecdsa we accept 256/384/521.
func validateAlgoBits(algo, bits string) bool {
	switch algo {
	case "ed25519", "dsa":
		return true
	case "rsa":
		for _, b := range rsaBits {
			if b == bits {
				return true
			}
		}
		// Also allow any numeric >= 2048 that isn't in the preset list.
		n := 0
		for _, r := range bits {
			if r < '0' || r > '9' {
				return false
			}
			n = n*10 + int(r-'0')
		}
		return n >= 2048
	case "ecdsa":
		for _, b := range ecdsaBits {
			if b == bits {
				return true
			}
		}
		return false
	}
	return false
}

// expandTilde replaces a leading "~/" or bare "~" with the user's home directory.
// Paths that don't start with "~" are returned unchanged.
func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// defaultOutPath returns the default key output path for an algorithm, already tilde-expanded.
func defaultOutPath(algo string) string {
	return expandTilde("~/.ssh/id_" + algo)
}

// defaultComment returns "$USER@$HOSTNAME" using environment and os hostname.
func defaultComment() string {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	host, _ := os.Hostname()
	if user == "" && host == "" {
		return ""
	}
	if user == "" {
		return host
	}
	if host == "" {
		return user
	}
	return user + "@" + host
}

func (m *KeyModel) View() string {
	if m.showHelp {
		return m.renderHelp()
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(ui.Banner())
	b.WriteString("\n  " + ui.TitleStyle.Render("── Generate SSH Key Pair ──") + "\n\n")

	m.viewSummary(&b)

	switch m.step {
	case stepAlgo:
		b.WriteString("  Key algorithm:\n\n")
		for i, label := range algoLabels {
			cursor := "  "
			style := ui.InactiveStyle
			if i == m.optCur {
				cursor = ui.ActiveStyle.Render("➤ ")
				style = ui.ActiveStyle
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, style.Render(label)))
		}

	case stepBits:
		var opts []string
		title := "Key size:"
		if m.algo == "rsa" {
			opts = rsaBits
			title = "RSA key size (bits):"
		} else if m.algo == "ecdsa" {
			opts = ecdsaBits
			title = "ECDSA curve size (bits):"
		}
		b.WriteString("  " + title + "\n\n")
		for i, o := range opts {
			cursor := "  "
			style := ui.InactiveStyle
			if i == m.optCur {
				cursor = ui.ActiveStyle.Render("➤ ")
				style = ui.ActiveStyle
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, style.Render(o)))
		}

	case stepOut:
		b.WriteString("  Output key path " + ui.DimStyle.Render("(~ expands to $HOME)") + ":\n\n")
		b.WriteString("  " + m.input.View() + "\n")

	case stepConfirmOverwrite:
		b.WriteString("  " + ui.WarnStyle.Render("File already exists: "+m.out) + "\n")
		b.WriteString("  Overwrite?\n\n")
		for i, o := range []string{"Yes", "No"} {
			cursor := "  "
			style := ui.InactiveStyle
			if i == m.optCur {
				cursor = ui.ActiveStyle.Render("➤ ")
				style = ui.ActiveStyle
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, style.Render(o)))
		}

	case stepComment:
		b.WriteString("  Comment " + ui.DimStyle.Render("(default: $USER@$HOSTNAME)") + ":\n\n")
		b.WriteString("  " + m.input.View() + "\n")

	case stepPassphrase:
		b.WriteString("  Passphrase " + ui.DimStyle.Render("(empty for no passphrase)") + ":\n\n")
		b.WriteString("  " + m.input.View() + "\n")
		if m.passErr != "" {
			b.WriteString("\n  " + ui.ErrorStyle.Render(m.passErr) + "\n")
		}

	case stepPassphrase2:
		b.WriteString("  Confirm passphrase:\n\n")
		b.WriteString("  " + m.input.View() + "\n")

	case stepGenerating:
		b.WriteString("  " + m.spin.View() + " Generating key pair...\n")

	case stepDone:
		if m.success {
			b.WriteString(ui.ResultBox(true, "SSH key pair generated!", m.result))
		} else {
			b.WriteString(ui.ResultBox(false, "Failed", m.result))
		}
	}

	b.WriteString("\n  " + ui.DimStyle.Render("? help  enter confirm  esc back  ctrl+c quit") + "\n")
	return b.String()
}

func (m *KeyModel) viewSummary(b *strings.Builder) {
	type field struct {
		label string
		value string
		step  keyStep
	}
	algoVal := m.algo
	if m.algo == "rsa" && m.bits != "" {
		algoVal = "rsa " + m.bits
	} else if m.algo == "ecdsa" && m.bits != "" {
		algoVal = "ecdsa p-" + m.bits
	}
	fields := []field{
		{"Algo", algoVal, stepAlgo},
		{"Out", m.out, stepOut},
		{"Comment", m.comment, stepComment},
	}
	hasAny := false
	for _, f := range fields {
		if f.step >= m.step {
			break
		}
		if f.value != "" {
			hasAny = true
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				ui.DimStyle.Render(fmt.Sprintf("%-8s", f.label)),
				ui.DimStyle.Render("="),
				f.value))
		}
	}
	if hasAny {
		b.WriteString("\n")
	}
}

func (m *KeyModel) renderHelp() string {
	sections := []ui.HelpSection{
		{
			Title: "Algorithm",
			Entries: []ui.HelpEntry{
				{Key: "↑/↓", Desc: "Choose algorithm (ed25519 recommended)"},
				{Key: "enter", Desc: "Confirm algorithm"},
			},
		},
		{
			Title: "Key size",
			Entries: []ui.HelpEntry{
				{Key: "↑/↓", Desc: "Choose size / curve"},
				{Key: "enter", Desc: "Confirm (skipped for ed25519/dsa)"},
			},
		},
		{
			Title: "Output path",
			Entries: []ui.HelpEntry{
				{Key: "~/", Desc: "Expanded to $HOME"},
				{Key: "enter (empty)", Desc: "Use default ~/.ssh/id_<algo>"},
			},
		},
		{
			Title: "Overwrite",
			Entries: []ui.HelpEntry{
				{Key: "↑/↓", Desc: "Yes/No"},
				{Key: "enter", Desc: "Confirm choice"},
			},
		},
		{
			Title: "Comment & passphrase",
			Entries: []ui.HelpEntry{
				{Key: "enter (empty)", Desc: "Use $USER@$HOSTNAME / no passphrase"},
				{Key: "enter", Desc: "Confirm, then re-enter to match"},
			},
		},
		ui.CommonHelp(),
	}
	return "\n" + ui.Banner() + "  " + ui.TitleStyle.Render("── Generate SSH Key Pair ──") + "\n" + ui.RenderHelp("Generate SSH Key — Help", sections)
}
