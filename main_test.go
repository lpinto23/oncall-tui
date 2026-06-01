package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name       string
		incidentID string
		summary    string
		wantErr    bool
	}{
		{name: "valid", incidentID: "PD-123", summary: "service degraded", wantErr: false},
		{name: "missing incident id", incidentID: "", summary: "service degraded", wantErr: true},
		{name: "missing summary", incidentID: "PD-123", summary: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredFields(tt.incidentID, tt.summary)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRequiredFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateChronology(t *testing.T) {
	now := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	later := now.Add(30 * time.Minute)
	earlier := now.Add(-30 * time.Minute)

	tests := []struct {
		name    string
		start   *time.Time
		end     *time.Time
		wantErr bool
	}{
		{name: "nil end allowed", start: &now, end: nil, wantErr: false},
		{name: "nil start allowed", start: nil, end: &later, wantErr: false},
		{name: "equal allowed", start: &now, end: &now, wantErr: false},
		{name: "after allowed", start: &now, end: &later, wantErr: false},
		{name: "end before start rejected", start: &now, end: &earlier, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChronology(tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateChronology() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetHeaderEndTime(t *testing.T) {
	end := time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC)
	endLine := "**End:** " + end.Format(incidentTimeLayout)

	content := strings.Join([]string{
		"# Incident PD-123",
		"",
		"**Start:** 2026-06-02 10:00:00 UTC",
		"**End:** Still open",
		"**Tags:** api,p1",
		"",
		"---",
		"",
		"Body line with literal **End:** Still open should stay untouched.",
	}, "\n")

	updated, err := setHeaderEndTime(content, end)
	if err != nil {
		t.Fatalf("setHeaderEndTime() unexpected error: %v", err)
	}

	if !strings.Contains(updated, endLine) {
		t.Fatalf("updated header is missing expected end line %q", endLine)
	}
	if strings.Count(updated, "**End:** Still open") != 1 {
		t.Fatalf("expected body literal to remain unchanged, got: %q", updated)
	}
}

func TestSetHeaderEndTimeMissingField(t *testing.T) {
	content := strings.Join([]string{
		"# Incident PD-123",
		"**Start:** 2026-06-02 10:00:00 UTC",
		"---",
	}, "\n")

	_, err := setHeaderEndTime(content, time.Now())
	if err == nil {
		t.Fatal("setHeaderEndTime() expected error when End header is missing")
	}
}

func TestUpsertResolutionSection(t *testing.T) {
	base := "# Incident PD-123\n\n**End:** Still open\n\n---\n\nBody"
	t1 := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	t2 := t1.Add(15 * time.Minute)

	first := upsertResolutionSection(base, "Applied hotfix", t1)
	if strings.Count(first, "## Resolution") != 1 {
		t.Fatalf("expected one resolution header after first upsert, got: %q", first)
	}
	if !strings.Contains(first, "Applied hotfix") {
		t.Fatalf("first upsert missing resolution text")
	}

	second := upsertResolutionSection(first, "Restarted worker", t2)
	if strings.Count(second, "## Resolution") != 1 {
		t.Fatalf("expected one resolution header after second upsert, got: %q", second)
	}
	if strings.Count(second, "**Resolved:**") != 2 {
		t.Fatalf("expected two resolution entries after second upsert, got: %q", second)
	}
	if !strings.Contains(second, "Applied hotfix") || !strings.Contains(second, "Restarted worker") {
		t.Fatalf("second upsert missing expected resolution entries")
	}
}

func TestWriteFileAtomically(t *testing.T) {
	t.Run("replaces content and preserves permissions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "incident.md")

		if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
			t.Fatalf("WriteFile() setup failed: %v", err)
		}

		if err := writeFileAtomically(path, []byte("new-content")); err != nil {
			t.Fatalf("writeFileAtomically() unexpected error: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() failed: %v", err)
		}
		if string(got) != "new-content" {
			t.Fatalf("content mismatch: got %q", string(got))
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() failed: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("permissions mismatch: got %o want %o", info.Mode().Perm(), os.FileMode(0600))
		}
	})

	t.Run("returns error when parent directory does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "incident.md")
		err := writeFileAtomically(path, []byte("content"))
		if err == nil {
			t.Fatal("writeFileAtomically() expected error for missing parent directory")
		}
	})
}
