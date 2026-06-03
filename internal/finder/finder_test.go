package finder

import (
	"testing"
)

func TestMatchesIncidentFileName(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		incident  string
		wantMatch bool
	}{
		{
			name:      "legacy format prefix match",
			fileName:  "INCIDENT_PD-367110_2026-06-03-19-01-06.md",
			incident:  "PD-367110",
			wantMatch: true,
		},
		{
			name:      "new format suffix match",
			fileName:  "2026-06-03-19-01-06_PD-367110.md",
			incident:  "PD-367110",
			wantMatch: true,
		},
		{
			name:      "non matching id",
			fileName:  "2026-06-03-19-01-06_PD-999999.md",
			incident:  "PD-367110",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesIncidentFileName(tt.fileName, tt.incident)
			if got != tt.wantMatch {
				t.Fatalf("matchesIncidentFileName(%q, %q) = %v, want %v", tt.fileName, tt.incident, got, tt.wantMatch)
			}
		})
	}
}
