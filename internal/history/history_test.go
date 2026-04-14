package history

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLogFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	Log("inspect", "file=x", "cn=y")

	data, err := os.ReadFile(filepath.Join(dir, ".sshtui", "history.log"))
	if err != nil {
		t.Fatalf("could not read log: %v", err)
	}
	line := strings.TrimRight(string(data), "\n")
	// YYYY-MM-DD HH:MM:SS | inspect | file=x cn=y
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \| inspect \| file=x cn=y$`)
	if !re.MatchString(line) {
		t.Errorf("unexpected log format: %q", line)
	}
}

func TestLogAppend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	Log("op1", "a=1")
	Log("op2", "b=2")

	data, err := os.ReadFile(filepath.Join(dir, ".sshtui", "history.log"))
	if err != nil {
		t.Fatalf("could not read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}

func TestLogDirCreation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	Log("test")

	logPath := filepath.Join(dir, ".sshtui", "history.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("expected log file at %s, got err: %v", logPath, err)
	}
}

func TestKVHelper(t *testing.T) {
	got := KV("file", "path with spaces")
	want := `file="path with spaces"`
	if got != want {
		t.Errorf("KV with spaces: got %q, want %q", got, want)
	}

	got = KV("key", "simple")
	if got != "key=simple" {
		t.Errorf("KV simple: got %q, want key=simple", got)
	}

	got = KV("key", "")
	if got != `key=""` {
		t.Errorf("KV empty: got %q, want key=\"\"", got)
	}

	got = KV("key", `has"quote`)
	if got != `key="has\"quote"` {
		t.Errorf("KV quoted: got %q, want %q", got, `key="has\"quote"`)
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := os.MkdirAll(filepath.Join(dir, ".sshtui"), 0755); err != nil {
		t.Fatal(err)
	}
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filepath.Join(dir, ".sshtui", "history.log"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	want := []string{"line1", "line2", "line3", "line4", "line5"}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %v", len(want), len(lines), lines)
	}
	for i, l := range want {
		if lines[i] != l {
			t.Errorf("line %d: got %q, want %q", i, lines[i], l)
		}
	}
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	lines, err := Read()
	if err != nil {
		t.Errorf("Read on missing file should not error, got: %v", err)
	}
	if lines != nil {
		t.Errorf("Read on missing file should return nil, got %v", lines)
	}
}
