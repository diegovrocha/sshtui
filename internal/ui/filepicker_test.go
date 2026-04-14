package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilePickerFilter(t *testing.T) {
	fp := FilePicker{
		entries: []fileEntry{
			{name: "id_rsa", isDir: false},
			{name: "id_rsa.pub", isDir: false},
			{name: "id_ed25519", isDir: false},
			{name: "server_ecdsa", isDir: false},
			{name: "host-cert.pub", isDir: false},
		},
		filtered: []fileEntry{
			{name: "id_rsa"},
			{name: "id_rsa.pub"},
			{name: "id_ed25519"},
			{name: "server_ecdsa"},
			{name: "host-cert.pub"},
		},
	}

	// Filter by "rsa"
	query := "rsa"
	fp.filtered = nil
	for _, e := range fp.entries {
		if strings.Contains(strings.ToLower(e.name), query) {
			fp.filtered = append(fp.filtered, e)
		}
	}

	if len(fp.filtered) != 2 {
		t.Errorf("Filter 'rsa' should return 2 files, returned %d", len(fp.filtered))
	}

	// Filter by "pub"
	query = "pub"
	fp.filtered = nil
	for _, e := range fp.entries {
		if strings.Contains(strings.ToLower(e.name), query) {
			fp.filtered = append(fp.filtered, e)
		}
	}

	if len(fp.filtered) != 2 {
		t.Errorf("Filter 'pub' should return 2 files, returned %d", len(fp.filtered))
	}

	// Empty filter returns all
	fp.filtered = fp.entries
	if len(fp.filtered) != 5 {
		t.Errorf("No filter should return 5 files, returned %d", len(fp.filtered))
	}
}

func TestFilePickerCursorBounds(t *testing.T) {
	fp := FilePicker{
		entries:  []fileEntry{{name: "id_rsa"}, {name: "id_ed25519"}, {name: "id_ecdsa"}},
		filtered: []fileEntry{{name: "id_rsa"}, {name: "id_ed25519"}, {name: "id_ecdsa"}},
		cursor:   0,
	}

	if fp.cursor < 0 {
		t.Error("Cursor should not be negative")
	}

	fp.cursor = len(fp.filtered) - 1
	if fp.cursor >= len(fp.filtered) {
		t.Error("Cursor should not exceed list size")
	}
}

func TestFilePickerView(t *testing.T) {
	fp := FilePicker{
		Prompt:   "Select key",
		cwd:      "/tmp",
		entries:  []fileEntry{{name: "id_rsa", isDir: false}},
		filtered: []fileEntry{{name: "id_rsa", isDir: false}},
		cursor:   0,
	}
	fp.filter.Placeholder = "type to filter..."

	v := fp.View()
	if !strings.Contains(v, "Select key") {
		t.Error("View should contain the prompt")
	}
	if !strings.Contains(v, "id_rsa") {
		t.Error("View should contain the file")
	}
}

func TestFilePickerEmpty(t *testing.T) {
	fp := FilePicker{
		Prompt:   "Select",
		cwd:      "/tmp",
		entries:  []fileEntry{},
		filtered: []fileEntry{},
	}
	fp.filter.Placeholder = "type to filter..."

	v := fp.View()
	if !strings.Contains(v, "No files found") {
		t.Error("Empty view should show no files found message")
	}
}

func TestFilePickerDirNavigation(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "keys")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "id_rsa"), []byte("key"), 0644)

	fp := newPicker("Select", []string{"id_*"})
	fp.cwd = dir
	fp.loadDir()

	// Should show the subdir
	found := false
	for _, e := range fp.entries {
		if e.isDir && e.name == "keys/" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should list subdirectory 'keys/'")
	}

	// Navigate into subdir
	for i, e := range fp.entries {
		if e.isDir && e.name == "keys/" {
			fp.cursor = i
			break
		}
	}
	fp.cwd = fp.entries[fp.cursor].path
	fp.loadDir()

	// Should show the file
	foundFile := false
	for _, e := range fp.entries {
		if !e.isDir && e.name == "id_rsa" {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Error("Should list 'id_rsa' inside subdir")
	}
}

func TestFilePickerShowsAllDirs(t *testing.T) {
	// Unlike certui, sshtui shows all directories so the user can freely
	// navigate — SSH keys tend to live only in ~/.ssh.
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "empty"), 0755)
	os.Mkdir(filepath.Join(dir, "haskey"), 0755)
	os.WriteFile(filepath.Join(dir, "haskey", "id_rsa"), []byte("key"), 0644)

	fp := newPicker("Select", []string{"id_*"})
	fp.cwd = dir
	fp.loadDir()

	var foundEmpty, foundHasKey bool
	for _, e := range fp.entries {
		if e.isDir && e.name == "empty/" {
			foundEmpty = true
		}
		if e.isDir && e.name == "haskey/" {
			foundHasKey = true
		}
	}
	if !foundEmpty {
		t.Error("Should list empty directory to allow free navigation")
	}
	if !foundHasKey {
		t.Error("Should list directory containing matching files")
	}
}

func TestMatchFilenameSSHKey(t *testing.T) {
	patterns := []string{"id_*", "*.pub", "*_rsa", "*_ed25519", "*_ecdsa", "*_dsa", "ssh_host_*"}
	cases := map[string]bool{
		"id_rsa":              true,
		"id_rsa.pub":          true,
		"id_ed25519":          true,
		"server_rsa":          true,
		"ssh_host_ecdsa_key":  true,
		"random.txt":          false,
		"notes.md":            false,
		"host-cert.pub":       true,
	}
	for name, want := range cases {
		if got := matchFilename(name, patterns); got != want {
			t.Errorf("matchFilename(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMatchFilenamePrivateKeyExcludesPub(t *testing.T) {
	patterns := []string{"id_*", "*_rsa", "*_ed25519", "*_ecdsa", "*_dsa"}
	cases := map[string]bool{
		"id_rsa":       true,
		"id_rsa.pub":   false,
		"id_ed25519":   true,
		"server_rsa":   true,
		"foo.pub":      false,
	}
	for name, want := range cases {
		if got := matchFilename(name, patterns); got != want {
			t.Errorf("matchFilename(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMatchFilenameSSHCert(t *testing.T) {
	patterns := []string{"*-cert.pub", "*_cert.pub"}
	cases := map[string]bool{
		"host-cert.pub":  true,
		"user_cert.pub":  true,
		"id_rsa.pub":     false,
		"id_rsa":         false,
	}
	for name, want := range cases {
		if got := matchFilename(name, patterns); got != want {
			t.Errorf("matchFilename(%q) = %v, want %v", name, got, want)
		}
	}
}
