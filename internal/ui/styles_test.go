package ui

import (
	"os"
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	b := Banner()
	// Banner uses lipgloss which may or may not include ANSI codes
	// depending on terminal detection. Check the raw ASCII text.
	if !strings.Contains(b, "___") {
		t.Error("Banner should contain ASCII art")
	}
	if !strings.Contains(b, Version) {
		t.Errorf("Banner should contain version '%s'", Version)
	}
	if !strings.Contains(b, "SSH") {
		t.Error("Banner should contain subtitle")
	}
}

func TestResultBox(t *testing.T) {
	ok := ResultBox(true, "Success", "file.pem")
	if !strings.Contains(ok, "Success") {
		t.Error("ResultBox success should contain title")
	}

	fail := ResultBox(false, "Error", "message")
	if !strings.Contains(fail, "Error") {
		t.Error("ResultBox error should contain title")
	}
}

func TestDetectThemeDark(t *testing.T) {
	t.Setenv("COLORFGBG", "15;0")
	if got := detectTheme(); got != "dark" {
		t.Errorf("detectTheme with 15;0 = %q, want dark", got)
	}
}

func TestDetectThemeLight(t *testing.T) {
	t.Setenv("COLORFGBG", "0;15")
	if got := detectTheme(); got != "light" {
		t.Errorf("detectTheme with 0;15 = %q, want light", got)
	}
}

func TestDetectThemeDefault(t *testing.T) {
	t.Setenv("COLORFGBG", "")
	// Unset entirely
	oldVal, had := os.LookupEnv("COLORFGBG")
	os.Unsetenv("COLORFGBG")
	defer func() {
		if had {
			os.Setenv("COLORFGBG", oldVal)
		}
	}()
	if got := detectTheme(); got != "dark" {
		t.Errorf("detectTheme with COLORFGBG unset = %q, want dark", got)
	}
}

func TestForceTheme(t *testing.T) {
	orig := ActiveTheme()
	defer ForceTheme(orig)

	ForceTheme("light")
	if ActiveTheme() != "light" {
		t.Errorf("after ForceTheme(light), ActiveTheme=%q, want light", ActiveTheme())
	}

	ForceTheme("dark")
	if ActiveTheme() != "dark" {
		t.Errorf("after ForceTheme(dark), ActiveTheme=%q, want dark", ActiveTheme())
	}

	// Invalid value is a no-op
	ForceTheme("invalid")
	if ActiveTheme() != "dark" {
		t.Errorf("ForceTheme(invalid) should be no-op; got %q", ActiveTheme())
	}
}

func TestThemeEnvOverride(t *testing.T) {
	orig := ActiveTheme()
	defer ForceTheme(orig)

	applyTheme("dark")
	darkCyan := string(ColorCyan)
	applyTheme("light")
	lightCyan := string(ColorCyan)
	if darkCyan == lightCyan {
		t.Errorf("dark and light ColorCyan should differ, both = %q", darkCyan)
	}
	if darkCyan != "14" {
		t.Errorf("dark ColorCyan should be 14, got %q", darkCyan)
	}
	if lightCyan != "6" {
		t.Errorf("light ColorCyan should be 6, got %q", lightCyan)
	}
}
