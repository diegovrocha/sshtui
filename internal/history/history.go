package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Log writes an entry to ~/.sshtui/history.log.
// Format: 2026-04-13 18:45:22 | operation | key=value key="value with spaces"
// Silent on any failure — must not interrupt the TUI.
func Log(operation string, details ...string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".sshtui")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	path := filepath.Join(dir, "history.log")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("%s | %s", ts, operation)
	if len(details) > 0 {
		line += " | " + strings.Join(details, " ")
	}
	line += "\n"
	_, _ = f.WriteString(line)
}

// KV formats a key=value pair. If the value contains spaces or is empty,
// the value is quoted.
func KV(key, value string) string {
	if value == "" || strings.ContainsAny(value, " \t\"") {
		value = strings.ReplaceAll(value, `"`, `\"`)
		return fmt.Sprintf(`%s="%s"`, key, value)
	}
	return fmt.Sprintf("%s=%s", key, value)
}

// Path returns the path to the history log file.
func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".sshtui", "history.log")
}

// Read returns the history log lines (all lines, oldest first).
func Read() ([]string, error) {
	p := Path()
	if p == "" {
		return nil, fmt.Errorf("could not resolve home directory")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var out []string
	for _, l := range raw {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out, nil
}
