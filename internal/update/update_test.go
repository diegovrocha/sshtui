package update

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNormalizeVer(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{"1.0 == 1.0.0", "1.0", "1.0.0", true},
		{"1.1.0 != 1.0.0", "1.1.0", "1.0.0", false},
		{"2.0 != 1.0", "2.0", "1.0", false},
		{"empty normalized to 0", "", "0", true},
		{"same exact", "1.2.3", "1.2.3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVer(tt.a) == normalizeVer(tt.b)
			if got != tt.equal {
				t.Errorf("normalizeVer(%q)=%q vs normalizeVer(%q)=%q: equal=%v, want %v",
					tt.a, normalizeVer(tt.a), tt.b, normalizeVer(tt.b), got, tt.equal)
			}
		})
	}

	if normalizeVer("") != "0" {
		t.Errorf("empty should normalize to 0, got %q", normalizeVer(""))
	}
}

func TestUpdateInfoMsgParsing(t *testing.T) {
	m := &Model{step: stepConfirm, current: "1.0.0"}

	// Error path
	next, _ := m.Update(updateInfoMsg{err: "network fail"})
	mm := next.(*Model)
	if mm.step != stepDone {
		t.Errorf("err path should move to stepDone, got %v", mm.step)
	}
	if mm.success {
		t.Error("err path should set success=false")
	}
	if mm.result != "network fail" {
		t.Errorf("err path result: got %q want %q", mm.result, "network fail")
	}

	// Success path
	m2 := &Model{step: stepConfirm, current: "1.0.0"}
	next2, _ := m2.Update(updateInfoMsg{latest: "1.2.0", body: "## Changes"})
	mm2 := next2.(*Model)
	if mm2.latest != "1.2.0" {
		t.Errorf("latest: got %q want 1.2.0", mm2.latest)
	}
	if mm2.body != "## Changes" {
		t.Errorf("body: got %q want '## Changes'", mm2.body)
	}
	if mm2.step != stepConfirm {
		t.Errorf("success path should remain in stepConfirm, got %v", mm2.step)
	}
}

func TestUpdateDownloadResult(t *testing.T) {
	m := &Model{step: stepDownloading, current: "1.0.0", latest: "1.2.0"}
	next, _ := m.Update(downloadResultMsg{success: true, message: "ok"})
	mm := next.(*Model)
	if mm.step != stepDone {
		t.Errorf("step should be stepDone, got %v", mm.step)
	}
	if !mm.success {
		t.Error("success should be true")
	}
}

// Ensure New() returns a concrete Model with current version set.
func TestNewUpdateModel(t *testing.T) {
	m := New()
	var _ tea.Model = m
	mm, ok := m.(*Model)
	if !ok {
		t.Fatal("New should return *Model")
	}
	if mm.step != stepConfirm {
		t.Errorf("initial step should be stepConfirm, got %v", mm.step)
	}
}
