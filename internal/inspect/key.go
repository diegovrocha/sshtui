package inspect

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/diegovrocha/sshtui/internal/history"
	"github.com/diegovrocha/sshtui/internal/ui"
)

type keyStep int

const (
	stepKeyFile keyStep = iota
	stepKeyPassphrase
	stepKeyResult
)

// KeyInfo holds the parsed information about an SSH key.
type KeyInfo struct {
	Type              string // RSA, ED25519, ECDSA, DSA
	Bits              int
	FingerprintSHA256 string
	FingerprintMD5    string
	Comment           string
	IsPublic          bool
	IsEncrypted       bool
	PublicKeyPreview  string // first 64 chars of the key data + "..."
}

// KeyModel is a Bubble Tea model that inspects SSH keys.
type KeyModel struct {
	step        keyStep
	picker      ui.FilePicker
	passIn      textinput.Model
	saveIn      textinput.Model
	saving      bool
	saveResult  string
	saveOk      bool
	copyMsg     string
	copyExpires time.Time
	file        string
	passphrase  string
	info        KeyInfo
	err         string
	height      int
	logged      bool
	showHelp    bool
	embedded    bool

	pendingInspect bool
}

// NewKey returns a new SSH key inspect model starting at the file picker.
func NewKey() tea.Model {
	return &KeyModel{
		step:   stepKeyFile,
		picker: ui.NewSSHKeyPicker("Select SSH key"),
	}
}

// NewKeyWithFile returns an inspect model pre-loaded with the given file path.
// If the file is an encrypted private key, the model jumps to the passphrase
// step; otherwise it goes directly to the result step.
func NewKeyWithFile(path string) tea.Model {
	return newKeyWithFile(path, false)
}

// NewKeyWithFileEmbedded is like NewKeyWithFile but skips the banner when
// rendered (used when embedded inside another screen).
func NewKeyWithFileEmbedded(path string) tea.Model {
	return newKeyWithFile(path, true)
}

func newKeyWithFile(path string, embedded bool) tea.Model {
	m := &KeyModel{file: path, embedded: embedded}
	isPub := detectIsPublic(path)
	if !isPub && isEncrypted(path) {
		m.step = stepKeyPassphrase
		m.passIn = textinput.New()
		m.passIn.Placeholder = "Private key passphrase"
		m.passIn.EchoMode = textinput.EchoPassword
		m.passIn.Focus()
		return m
	}
	m.step = stepKeyResult
	m.pendingInspect = true
	return m
}

func (m *KeyModel) Init() tea.Cmd {
	if m.pendingInspect {
		m.pendingInspect = false
		return m.doInspect()
	}
	return textinput.Blink
}

type keyInspectResult struct {
	info KeyInfo
	err  string
}

type keyCopyClearMsg struct{}

type keySaveResultMsg struct {
	ok      bool
	message string
}

func (m *KeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil
	case keyInspectResult:
		m.info = msg.info
		m.err = msg.err
		m.step = stepKeyResult
		if msg.err == "" && !m.logged {
			history.Log("inspect_ssh_key",
				history.KV("file", m.file),
				history.KV("type", msg.info.Type),
				history.KV("fingerprint", msg.info.FingerprintSHA256))
			m.logged = true
		}
		return m, nil
	case keyCopyClearMsg:
		if !time.Now().Before(m.copyExpires) {
			m.copyMsg = ""
		}
		return m, nil
	case keySaveResultMsg:
		m.saving = false
		m.saveOk = msg.ok
		m.saveResult = msg.message
		return m, nil
	case tea.KeyMsg:
		if m.step == stepKeyResult && !m.saving {
			if msg.String() == "?" {
				m.showHelp = !m.showHelp
				return m, nil
			}
			if m.showHelp {
				if msg.String() == "esc" {
					m.showHelp = false
					return m, nil
				}
				return m, nil
			}
		}
		if msg.String() == "esc" {
			if m.saving {
				m.saving = false
				return m, nil
			}
			if m.step == stepKeyResult {
				m.resetToPicker()
				return m, nil
			}
			return m, nil
		}
		if m.step == stepKeyResult {
			if m.saving {
				if msg.String() == "enter" {
					name := strings.TrimSpace(m.saveIn.Value())
					if name == "" {
						name = defaultKeySaveName(m.info)
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
				m.saveIn.Placeholder = defaultKeySaveName(m.info)
				m.saveIn.SetValue(defaultKeySaveName(m.info))
				m.saveIn.Focus()
				return m, m.saveIn.Focus()
			case "n", "N":
				m.resetToPicker()
				return m, nil
			}
			return m, nil
		}
	}

	switch m.step {
	case stepKeyFile:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		if m.picker.Done {
			m.file = m.picker.Selected
			isPub := detectIsPublic(m.file)
			if !isPub && isEncrypted(m.file) {
				m.step = stepKeyPassphrase
				m.passIn = textinput.New()
				m.passIn.Placeholder = "Private key passphrase"
				m.passIn.EchoMode = textinput.EchoPassword
				m.passIn.Focus()
				return m, m.passIn.Focus()
			}
			return m, m.doInspect()
		}
		return m, cmd

	case stepKeyPassphrase:
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
			m.passphrase = m.passIn.Value()
			return m, m.doInspect()
		}
		var cmd tea.Cmd
		m.passIn, cmd = m.passIn.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *KeyModel) resetToPicker() {
	m.step = stepKeyFile
	m.info = KeyInfo{}
	m.err = ""
	m.logged = false
	m.saveResult = ""
	m.copyMsg = ""
	m.passphrase = ""
	m.picker = ui.NewSSHKeyPicker("Select SSH key")
}

func (m *KeyModel) View() string {
	if m.showHelp {
		return m.renderHelp()
	}
	var b strings.Builder
	if !m.embedded {
		b.WriteString("\n")
		b.WriteString(ui.Banner())
		b.WriteString("\n  " + ui.TitleStyle.Render("── Inspect SSH Key ──") + "\n\n")
	}

	switch m.step {
	case stepKeyFile:
		b.WriteString(m.picker.View())

	case stepKeyPassphrase:
		b.WriteString(fmt.Sprintf("  File: %s\n\n", ui.ActiveStyle.Render(m.file)))
		b.WriteString("  🔑 Private key passphrase:\n\n")
		b.WriteString("  " + m.passIn.View() + "\n")

	case stepKeyResult:
		if m.err != "" {
			b.WriteString("  " + ui.ErrorStyle.Render("✖ Error: ") + m.err + "\n")
		} else if m.saving {
			b.WriteString("  Save key details to file:\n\n")
			b.WriteString("  " + m.saveIn.View() + "\n")
		} else {
			b.WriteString(formatKeyInfo(m.file, m.info))
		}
	}

	if m.step == stepKeyResult && m.err == "" {
		if m.copyMsg != "" && time.Now().Before(m.copyExpires) {
			b.WriteString("\n  " + ui.SuccessStyle.Render(m.copyMsg) + "\n")
		}
		if m.saveResult != "" {
			b.WriteString("\n")
			if m.saveOk {
				b.WriteString("  " + ui.SuccessStyle.Render("✔ Saved: ") + m.saveResult + "\n")
			} else {
				b.WriteString("  " + ui.ErrorStyle.Render("✖ Error: ") + m.saveResult + "\n")
			}
		}
		if m.saving {
			b.WriteString("\n  " + ui.DimStyle.Render("enter save  esc cancel  ctrl+c quit") + "\n")
		} else {
			b.WriteString("\n  " + ui.DimStyle.Render("? help  n inspect another  y copy  s save  esc back  ctrl+c quit") + "\n")
		}
	} else {
		b.WriteString("\n  " + ui.DimStyle.Render("esc back  ↑/↓ navigate  enter confirm  ctrl+c quit") + "\n")
	}
	return b.String()
}

func (m *KeyModel) renderHelp() string {
	var b strings.Builder
	if !m.embedded {
		b.WriteString("\n")
		b.WriteString(ui.Banner())
		b.WriteString("\n  " + ui.TitleStyle.Render("── Inspect SSH Key — Help ──") + "\n\n")
	}
	b.WriteString("  " + ui.TitleStyle.Render("Result view") + "\n")
	b.WriteString("    y         Copy details to clipboard\n")
	b.WriteString("    s         Save details to .txt file\n")
	b.WriteString("    n         Inspect another key\n")
	b.WriteString("\n")
	b.WriteString("  " + ui.TitleStyle.Render("Common") + "\n")
	b.WriteString("    ?         Toggle this help\n")
	b.WriteString("    esc       Back / close help\n")
	b.WriteString("    ctrl+c    Quit\n")
	b.WriteString("\n  " + ui.DimStyle.Render("press ? or esc to close help") + "\n")
	return b.String()
}

func formatKeyInfo(file string, k KeyInfo) string {
	var b strings.Builder
	title := "SSH Private Key"
	if k.IsPublic {
		title = "SSH Public Key"
	}
	b.WriteString(fmt.Sprintf("  %s\n\n", ui.TitleStyle.Render(title)))
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "File:", file))
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "Type:", k.Type))
	if k.Bits > 0 {
		b.WriteString(fmt.Sprintf("    %-20s %d\n", "Size (bits):", k.Bits))
	}
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "Fingerprint SHA-256:", k.FingerprintSHA256))
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "Fingerprint MD5:", k.FingerprintMD5))
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "Comment:", k.Comment))
	encStr := "no"
	if k.IsEncrypted {
		encStr = "yes"
	}
	b.WriteString(fmt.Sprintf("    %-20s %s\n", "Encrypted:", encStr))
	if k.PublicKeyPreview != "" {
		b.WriteString(fmt.Sprintf("    %-20s %s\n", "Public key:", k.PublicKeyPreview))
	}
	return b.String()
}

func (m *KeyModel) plainFormattedText() string {
	var b strings.Builder
	b.WriteString(formatKeyInfo(m.file, m.info))
	return stripKeyANSI(b.String())
}

var keyAnsiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripKeyANSI(s string) string {
	return keyAnsiRE.ReplaceAllString(s, "")
}

func defaultKeySaveName(k KeyInfo) string {
	name := "ssh_key"
	if k.Comment != "" {
		name = sanitizeKeyFilename(k.Comment)
	}
	if name == "" {
		name = "ssh_key"
	}
	return name + ".txt"
}

func sanitizeKeyFilename(s string) string {
	r := strings.NewReplacer(
		"*", "_", " ", "_", "/", "_", "\\", "_",
		":", "_", "?", "_", "\"", "_", "<", "_",
		">", "_", "|", "_", "@", "_at_",
	)
	return strings.Trim(r.Replace(s), "._")
}

func (m *KeyModel) doCopy() tea.Cmd {
	text := m.plainFormattedText()
	m.copyExpires = time.Now().Add(2 * time.Second)
	if err := clipboard.WriteAll(text); err != nil {
		m.copyMsg = "✖ Copy failed: " + err.Error()
	} else {
		m.copyMsg = "✔ Copied to clipboard"
	}
	return tea.Tick(2100*time.Millisecond, func(t time.Time) tea.Msg {
		return keyCopyClearMsg{}
	})
}

func (m *KeyModel) doSave(name string) tea.Cmd {
	text := m.plainFormattedText()
	return func() tea.Msg {
		if err := os.WriteFile(name, []byte(text), 0644); err != nil {
			return keySaveResultMsg{ok: false, message: "Could not save: " + err.Error()}
		}
		return keySaveResultMsg{ok: true, message: "File: " + name}
	}
}

func (m *KeyModel) doInspect() tea.Cmd {
	file := m.file
	passphrase := m.passphrase
	return func() tea.Msg {
		info, err := inspectKey(file, passphrase)
		if err != nil {
			return keyInspectResult{err: err.Error()}
		}
		return keyInspectResult{info: info}
	}
}

// inspectKey runs ssh-keygen against the given file and returns the parsed
// KeyInfo. For private keys that are encrypted, passphrase must be provided
// in order to derive the public key.
func inspectKey(file, passphrase string) (KeyInfo, error) {
	info := KeyInfo{}
	info.IsPublic = detectIsPublic(file)
	if !info.IsPublic {
		info.IsEncrypted = isEncrypted(file)
	}

	// SHA-256 fingerprint (works on both public and private key files).
	sha256Out, err := runKeygen(file, passphrase, "-l", "-E", "sha256", "-f", file)
	if err != nil {
		return info, fmt.Errorf("ssh-keygen -l sha256 failed: %s", err.Error())
	}
	parsed, ok := parseKeygenLine(sha256Out)
	if !ok {
		return info, fmt.Errorf("could not parse ssh-keygen output: %s", strings.TrimSpace(sha256Out))
	}
	info.Bits = parsed.Bits
	info.FingerprintSHA256 = parsed.Fingerprint
	info.Comment = parsed.Comment
	info.Type = parsed.Type

	// MD5 fingerprint (legacy).
	if md5Out, err := runKeygen(file, passphrase, "-l", "-E", "md5", "-f", file); err == nil {
		if p, ok := parseKeygenLine(md5Out); ok {
			info.FingerprintMD5 = p.Fingerprint
		}
	}

	// Derive public key preview for private keys (and read directly for public).
	if info.IsPublic {
		data, err := os.ReadFile(file)
		if err == nil {
			info.PublicKeyPreview = firstLinePreview(string(data), 64)
		}
	} else {
		pubOut, err := runKeygenRaw("ssh-keygen", "-y", "-P", passphrase, "-f", file)
		if err == nil {
			info.PublicKeyPreview = firstLinePreview(pubOut, 64)
		}
	}

	return info, nil
}

// parsedKeygenLine is the struct form of a single `ssh-keygen -l` output line.
type parsedKeygenLine struct {
	Bits        int
	Fingerprint string
	Comment     string
	Type        string
}

// parseKeygenLine parses one line of `ssh-keygen -l` output, e.g.
//
//	256 SHA256:abcdef... user@host (ED25519)
//	2048 MD5:aa:bb:cc... no comment (RSA)
func parseKeygenLine(line string) (parsedKeygenLine, bool) {
	s := strings.TrimSpace(line)
	if s == "" {
		return parsedKeygenLine{}, false
	}
	// Extract the trailing (TYPE).
	typ := ""
	if i := strings.LastIndex(s, "("); i >= 0 {
		if j := strings.LastIndex(s, ")"); j > i {
			typ = s[i+1 : j]
			s = strings.TrimSpace(s[:i])
		}
	}
	parts := strings.SplitN(s, " ", 3)
	if len(parts) < 2 {
		return parsedKeygenLine{}, false
	}
	bits := 0
	fmt.Sscanf(parts[0], "%d", &bits)
	fp := parts[1]
	comment := ""
	if len(parts) == 3 {
		comment = strings.TrimSpace(parts[2])
	}
	return parsedKeygenLine{
		Bits:        bits,
		Fingerprint: fp,
		Comment:     comment,
		Type:        typ,
	}, true
}

// detectIsPublic returns true if the file looks like an SSH public key.
// A file is considered public if its name ends with .pub OR its first line
// starts with a known SSH public key type prefix.
func detectIsPublic(path string) bool {
	if strings.HasSuffix(strings.ToLower(path), ".pub") {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return looksLikePublicKeyData(string(data))
}

func looksLikePublicKeyData(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	// First non-empty line
	line := trimmed
	if nl := strings.Index(trimmed, "\n"); nl >= 0 {
		line = strings.TrimSpace(trimmed[:nl])
	}
	prefixes := []string{
		"ssh-rsa ",
		"ssh-dss ",
		"ssh-ed25519 ",
		"ecdsa-sha2-",
		"sk-ecdsa-sha2-",
		"sk-ssh-ed25519",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// isEncrypted reports whether the given file is an encrypted SSH private key.
// It first greps the file for "ENCRYPTED" / "Proc-Type: 4,ENCRYPTED" / the
// OpenSSH encrypted marker, and falls back to `ssh-keygen -y -P ''` which
// fails with a passphrase-related error when the file is encrypted.
func isEncrypted(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	if strings.Contains(content, "ENCRYPTED") {
		return true
	}
	// OpenSSH-formatted keys don't contain the word "ENCRYPTED" in the header
	// when they are encrypted; probe with ssh-keygen.
	out, err := runKeygenRaw("ssh-keygen", "-y", "-P", "", "-f", path)
	if err == nil {
		return false
	}
	low := strings.ToLower(out + " " + err.Error())
	if strings.Contains(low, "incorrect passphrase") ||
		strings.Contains(low, "bad passphrase") ||
		strings.Contains(low, "passphrase") {
		return true
	}
	return false
}

// firstLinePreview returns up to n runes of the first non-empty line of s,
// stripping a leading type prefix like "ssh-rsa " and returning the key blob.
// A "..." ellipsis is appended when the key data is longer than n chars.
func firstLinePreview(s string, n int) string {
	line := ""
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			line = l
			break
		}
	}
	if line == "" {
		return ""
	}
	// If it looks like "ssh-xxx BLOB comment", extract BLOB.
	fields := strings.Fields(line)
	blob := line
	if len(fields) >= 2 && looksLikePublicKeyData(line) {
		blob = fields[1]
	}
	if len(blob) > n {
		return blob[:n] + "..."
	}
	return blob
}

// runKeygen runs `ssh-keygen <args...>` and returns the combined stdout. If
// the key is encrypted, the passphrase is supplied. It prefers to pass the
// passphrase via -P where applicable; for subcommands that already take -P
// the caller should include it in args.
func runKeygen(_ string, _ string, args ...string) (string, error) {
	return runKeygenRaw("ssh-keygen", args...)
}

func runKeygenRaw(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
