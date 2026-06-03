package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lpinto23/oncall-tui/internal/config"
	"github.com/lpinto23/oncall-tui/internal/model"
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

func TestParseAffectedServices(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty input", raw: "", want: nil},
		{name: "comma separated", raw: "checkout-api,postgres,redis", want: []string{"checkout-api", "postgres", "redis"}},
		{name: "trims blanks", raw: " checkout-api, , redis ", want: []string{"checkout-api", "redis"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAffectedServices(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("item %d mismatch: got %q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStripPlaceholderPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "removes lowercase eg", in: "e.g. PD-367110", want: "PD-367110"},
		{name: "removes mixed case eg", in: "E.G. PD-367110", want: "PD-367110"},
		{name: "keeps plain value", in: "PD-367110", want: "PD-367110"},
		{name: "keeps empty after eg", in: "e.g.", want: "e.g."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPlaceholderPrefix(tt.in)
			if got != tt.want {
				t.Fatalf("stripPlaceholderPrefix(%q) = %q, want %q", tt.in, got, tt.want)
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

func TestBuildIncidentContentRawMode(t *testing.T) {
	start := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)
	inc := modelIncidentForTest(
		"PD-321",
		"API latency spike",
		&start,
		&end,
		[]string{"checkout-api", "postgres"},
		[]string{"api", "p1"},
		"Rolled back deploy",
	)

	content, err := buildIncidentContent(inc, true)
	if err != nil {
		t.Fatalf("buildIncidentContent(raw) returned error: %v", err)
	}

	if strings.Contains(content, "{Two to four sentences") {
		t.Fatalf("raw content should not include LLM template placeholders: %q", content)
	}
	if !strings.Contains(content, "## Description") || !strings.Contains(content, "## Incident Details") || !strings.Contains(content, "## Timeline") {
		t.Fatalf("raw content missing required sections: %q", content)
	}
	if !strings.Contains(content, "API latency spike") {
		t.Fatalf("raw content missing user summary: %q", content)
	}
	if !strings.Contains(content, "| Affected Services | checkout-api, postgres |") {
		t.Fatalf("raw content missing affected services table row: %q", content)
	}
	if strings.HasPrefix(strings.TrimSpace(content), "API latency spike") {
		t.Fatalf("raw content should start directly at Description section")
	}
	if !strings.HasPrefix(strings.TrimSpace(content), "## Description") {
		t.Fatalf("raw content should start with Description section")
	}
}

func TestBuildEnrichedIncidentContent(t *testing.T) {
	start := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	inc := modelIncidentForTest(
		"PD-900",
		"Checkout latency spike",
		&start,
		&end,
		[]string{"checkout-api", "postgres"},
		[]string{"latency", "p1"},
		"Rolled back deployment",
	)

	t.Run("adds enrichment header once", func(t *testing.T) {
		input := "Incident summary\n\n## Description\n\ncontent"
		got := buildEnrichedIncidentContent(inc, input)
		if !strings.HasPrefix(got, "## Original Raw Input\n\n```text\n") {
			t.Fatalf("missing raw input section header: %q", got)
		}
		if !strings.Contains(got, "Summary: Checkout latency spike") {
			t.Fatalf("raw input section should preserve summary: %q", got)
		}
		if !strings.Contains(got, "\n\n"+aiEnrichedHeader+"\n\n") {
			t.Fatalf("missing enrichment header after raw section: %q", got)
		}
		if strings.Count(got, aiEnrichedHeader) != 1 {
			t.Fatalf("expected single enrichment header: %q", got)
		}
	})

	t.Run("keeps existing header", func(t *testing.T) {
		input := aiEnrichedHeader + "\n\nAlready enriched"
		got := buildEnrichedIncidentContent(inc, input)
		if strings.Count(got, aiEnrichedHeader) != 1 {
			t.Fatalf("expected single enrichment header when input already has one")
		}
		if !strings.Contains(got, "Already enriched") {
			t.Fatalf("expected enriched content to be preserved")
		}
	})
}

func TestResolveRawMode(t *testing.T) {
	tests := []struct {
		name       string
		cfgMode    string
		rawFlag    bool
		enrichFlag bool
		want       bool
		wantErr    bool
	}{
		{name: "default enriched", cfgMode: config.ModeEnriched, want: false},
		{name: "default raw", cfgMode: config.ModeRaw, want: true},
		{name: "raw flag overrides", cfgMode: config.ModeEnriched, rawFlag: true, want: true},
		{name: "enriched flag overrides raw default", cfgMode: config.ModeRaw, enrichFlag: true, want: false},
		{name: "conflicting flags", cfgMode: config.ModeEnriched, rawFlag: true, enrichFlag: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRawMode(config.Config{DefaultMode: tt.cfgMode}, tt.rawFlag, tt.enrichFlag)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveRawMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("resolveRawMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRawIncidentContentEscapesTableCells(t *testing.T) {
	start := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	inc := modelIncidentForTest(
		"PD|777",
		"Gateway | timeout",
		&start,
		nil,
		[]string{"checkout|api"},
		[]string{"edge|api"},
		"",
	)

	content := buildRawIncidentContent(inc)

	if !strings.Contains(content, "| Incident ID | PD\\|777 |") {
		t.Fatalf("incident id cell was not escaped: %q", content)
	}
	if !strings.Contains(content, "edge\\|api") {
		t.Fatalf("tags cell was not escaped: %q", content)
	}
	if !strings.Contains(content, "checkout\\|api") {
		t.Fatalf("affected services cell was not escaped: %q", content)
	}
}

func TestIncidentTimeSummary(t *testing.T) {
	start := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		end     *time.Time
		wantEnd string
		wantDur string
	}{
		{name: "open incident", end: nil, wantEnd: "Still open", wantDur: "Ongoing"},
		{name: "short incident", end: timePtr(start.Add(30 * time.Second)), wantEnd: start.Add(30 * time.Second).Format(incidentTimeLayout), wantDur: "<1 minute"},
		{name: "minutes incident", end: timePtr(start.Add(15 * time.Minute)), wantEnd: start.Add(15 * time.Minute).Format(incidentTimeLayout), wantDur: "~15 minutes"},
		{name: "hours incident", end: timePtr(start.Add(2*time.Hour + 5*time.Minute)), wantEnd: start.Add(2*time.Hour + 5*time.Minute).Format(incidentTimeLayout), wantDur: "~2h 5m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inc := modelIncidentForTest("PD-1", "summary", &start, tt.end, nil, nil, "")
			_, gotEnd, gotDur := incidentTimeSummary(inc)
			if gotEnd != tt.wantEnd {
				t.Fatalf("end mismatch: got %q want %q", gotEnd, tt.wantEnd)
			}
			if gotDur != tt.wantDur {
				t.Fatalf("duration mismatch: got %q want %q", gotDur, tt.wantDur)
			}
		})
	}
}

func TestCloseIncidentUsesResolutionTimestampAsEndWhenEndIsBlank(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident.md")
	content := strings.Join([]string{
		"# Incident PD-123",
		"",
		"**Start:** 2026-06-02 10:00:00 UTC",
		"**End:** Still open",
		"",
		"---",
		"",
		"Body",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() setup failed: %v", err)
	}

	if _, err := closeIncident(path, nil, "Applied mitigation"); err != nil {
		t.Fatalf("closeIncident() failed: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	text := string(updated)

	endLine := lineWithPrefix(text, "**End:** ")
	if endLine == "" {
		t.Fatalf("missing End header in updated content: %q", text)
	}
	resolvedLine := lineWithPrefix(text, "**Resolved:** ")
	if resolvedLine == "" {
		t.Fatalf("missing Resolved line in updated content: %q", text)
	}

	endValue := strings.TrimPrefix(endLine, "**End:** ")
	resolvedValue := strings.TrimPrefix(resolvedLine, "**Resolved:** ")
	if endValue != resolvedValue {
		t.Fatalf("expected end time to match resolution timestamp, got end=%q resolved=%q", endValue, resolvedValue)
	}
}

func TestCloseIncidentBlankEndAndNoResolutionSetsEndOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incident.md")
	content := strings.Join([]string{
		"# Incident PD-123",
		"",
		"**Start:** 2026-06-02 10:00:00 UTC",
		"**End:** Still open",
		"",
		"---",
		"",
		"Body",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() setup failed: %v", err)
	}

	if _, err := closeIncident(path, nil, ""); err != nil {
		t.Fatalf("closeIncident() failed: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	text := string(updated)

	if strings.Contains(text, "**End:** Still open") {
		t.Fatalf("expected end time to be set, still open found: %q", text)
	}
	if strings.Contains(text, "## Resolution") {
		t.Fatalf("did not expect resolution section when resolution is empty: %q", text)
	}
}

func lineWithPrefix(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func modelIncidentForTest(id, summary string, start, end *time.Time, affectedServices, tags []string, resolution string) model.Incident {
	return model.Incident{
		PagerDutyID:      id,
		Summary:          summary,
		StartTime:        start,
		EndTime:          end,
		AffectedServices: affectedServices,
		Tags:             tags,
		Resolution:       resolution,
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
