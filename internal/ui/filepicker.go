package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type fileEntry struct {
	name  string
	isDir bool
	path  string
}

type FilePicker struct {
	Prompt   string
	cwd      string
	exts     []string
	entries  []fileEntry
	filtered []fileEntry
	cursor   int
	filter   textinput.Model
	Selected string
	Done     bool
}

func newPicker(prompt string, exts []string) FilePicker {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Focus()

	cwd, _ := os.Getwd()

	fp := FilePicker{
		Prompt: prompt,
		cwd:    cwd,
		exts:   exts,
		filter: ti,
	}
	fp.loadDir()
	return fp
}

// NewFilePicker is a generic constructor. The patterns arg is a list of
// filename glob patterns (matched with filepath.Match against the filename).
func NewFilePicker(prompt string, patterns ...string) FilePicker {
	return newPicker(prompt, patterns)
}

// NewSSHKeyPicker matches any SSH key-related filename (public or private).
func NewSSHKeyPicker(prompt string) FilePicker {
	return newPicker(prompt, []string{
		"id_*",
		"*.pub",
		"*_rsa",
		"*_ed25519",
		"*_ecdsa",
		"*_dsa",
		"ssh_host_*",
	})
}

// NewSSHCertPicker matches SSH certificate filenames.
func NewSSHCertPicker(prompt string) FilePicker {
	return newPicker(prompt, []string{
		"*-cert.pub",
		"*_cert.pub",
	})
}

// NewPrivateKeyPicker matches SSH private key filenames (excludes *.pub).
func NewPrivateKeyPicker(prompt string) FilePicker {
	return newPicker(prompt, []string{
		"id_*",
		"*_rsa",
		"*_ed25519",
		"*_ecdsa",
		"*_dsa",
	})
}

// matchFilename returns true if name matches any of the given glob patterns.
// Files ending in .pub are excluded when none of the patterns explicitly
// target .pub files (so NewPrivateKeyPicker will not match public keys even
// though "id_*" would otherwise match "id_rsa.pub").
func matchFilename(name string, patterns []string) bool {
	// Determine if any pattern explicitly targets .pub files.
	allowPub := false
	for _, p := range patterns {
		if strings.HasSuffix(p, ".pub") {
			allowPub = true
			break
		}
	}
	if !allowPub && strings.HasSuffix(name, ".pub") {
		return false
	}
	for _, p := range patterns {
		ok, err := filepath.Match(p, name)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func (fp *FilePicker) loadDir() {
	fp.entries = nil
	fp.cursor = 0
	fp.filter.SetValue("")

	dirEntries, err := os.ReadDir(fp.cwd)
	if err != nil {
		return
	}

	// Directories first
	var dirs []fileEntry
	var files []fileEntry

	for _, e := range dirEntries {
		// Skip only a few noisy entries; show .ssh, .config, etc.
		// (SSH keys live in ~/.ssh — users need to see dot-dirs.)
		switch e.Name() {
		case ".git", ".DS_Store", ".Trash", ".cache":
			continue
		}
		if e.IsDir() {
			// Show ALL directories so the user can freely navigate.
			// SSH keys tend to live in ~/.ssh only, so filtering dirs
			// by content (like certui does) would hide useful folders.
			dirs = append(dirs, fileEntry{name: e.Name() + "/", isDir: true, path: filepath.Join(fp.cwd, e.Name())})
		} else {
			if matchFilename(e.Name(), fp.exts) {
				files = append(files, fileEntry{name: e.Name(), isDir: false, path: filepath.Join(fp.cwd, e.Name())})
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	fp.entries = append(dirs, files...)
	fp.filtered = fp.entries
}

func (fp *FilePicker) dirHasFiles(dir string, depth int) bool {
	if depth <= 0 {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		switch e.Name() {
		case ".git", ".DS_Store", ".Trash", ".cache":
			continue
		}
		if e.IsDir() {
			if fp.dirHasFiles(filepath.Join(dir, e.Name()), depth-1) {
				return true
			}
		} else {
			if matchFilename(e.Name(), fp.exts) {
				return true
			}
		}
	}
	return false
}

func (fp FilePicker) Init() tea.Cmd {
	return textinput.Blink
}

func (fp FilePicker) Update(msg tea.Msg) (FilePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if fp.cursor > 0 {
				fp.cursor--
			}
			return fp, nil
		case "down":
			if fp.cursor < len(fp.filtered)-1 {
				fp.cursor++
			}
			return fp, nil
		case "left":
			// Go to parent directory
			parent := filepath.Dir(fp.cwd)
			if parent != fp.cwd {
				fp.cwd = parent
				fp.loadDir()
			}
			return fp, nil
		case "right":
			// Enter highlighted directory (right-arrow shortcut)
			if len(fp.filtered) > 0 {
				entry := fp.filtered[fp.cursor]
				if entry.isDir {
					fp.cwd = entry.path
					fp.loadDir()
				}
			}
			return fp, nil
		case "enter":
			if len(fp.filtered) == 0 {
				return fp, nil
			}
			entry := fp.filtered[fp.cursor]
			if entry.isDir {
				fp.cwd = entry.path
				fp.loadDir()
				return fp, nil
			}
			fp.Selected = entry.path
			fp.Done = true
			return fp, nil
		}
	}

	// Update the text filter
	var cmd tea.Cmd
	fp.filter, cmd = fp.filter.Update(msg)

	// Apply filter
	query := strings.ToLower(fp.filter.Value())
	if query == "" {
		fp.filtered = fp.entries
	} else {
		fp.filtered = nil
		for _, e := range fp.entries {
			if strings.Contains(strings.ToLower(e.name), query) {
				fp.filtered = append(fp.filtered, e)
			}
		}
	}

	// Adjust cursor
	if fp.cursor >= len(fp.filtered) {
		fp.cursor = len(fp.filtered) - 1
	}
	if fp.cursor < 0 {
		fp.cursor = 0
	}

	return fp, cmd
}

func (fp FilePicker) View() string {
	var b strings.Builder

	b.WriteString("  " + ActiveStyle.Render(fp.Prompt) + "\n")

	// Breadcrumb
	home, _ := os.UserHomeDir()
	display := fp.cwd
	if strings.HasPrefix(display, home) {
		display = "~" + display[len(home):]
	}
	b.WriteString("  " + DimStyle.Render("📂 "+display) + "\n\n")

	b.WriteString("  " + fp.filter.View() + "\n\n")

	if len(fp.entries) == 0 {
		b.WriteString("  " + ErrorStyle.Render("No files found in this directory") + "\n")
		b.WriteString("  " + DimStyle.Render("← go to parent directory") + "\n")
		return b.String()
	}

	if len(fp.filtered) == 0 {
		b.WriteString("  " + DimStyle.Render("No results for this filter") + "\n")
		return b.String()
	}

	// Show at most 15 items with scroll
	maxVisible := 15
	start := 0
	if fp.cursor >= maxVisible {
		start = fp.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(fp.filtered) {
		end = len(fp.filtered)
	}

	if start > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", DimStyle.Render(fmt.Sprintf("  ↑ %d more above", start))))
	}

	for i := start; i < end; i++ {
		e := fp.filtered[i]
		icon := "  "
		if e.isDir {
			icon = "📁 "
		}
		if i == fp.cursor {
			b.WriteString(fmt.Sprintf("  %s%s%s\n", ActiveStyle.Render("➤ "), icon, ActiveStyle.Render(e.name)))
		} else {
			b.WriteString(fmt.Sprintf("    %s%s\n", icon, e.name))
		}
	}

	remaining := len(fp.filtered) - end
	if remaining > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", DimStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining))))
	}

	// Count files and dirs
	nDirs := 0
	nFiles := 0
	for _, e := range fp.filtered {
		if e.isDir {
			nDirs++
		} else {
			nFiles++
		}
	}
	b.WriteString(fmt.Sprintf("\n  %s\n", DimStyle.Render(fmt.Sprintf("%d files, %d folders", nFiles, nDirs))))
	b.WriteString("  " + DimStyle.Render("←/→ parent/enter folder  enter open/select") + "\n")

	return b.String()
}
