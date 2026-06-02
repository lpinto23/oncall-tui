package ui

import (
	"testing"

	"github.com/lpinto23/oncall-tui/internal/config"
)

func TestParseSetupMode(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		placeholder string
		want        string
		wantErr     bool
	}{
		{name: "explicit raw", raw: "raw", placeholder: config.ModeEnriched, want: config.ModeRaw, wantErr: false},
		{name: "explicit enriched", raw: "enriched", placeholder: config.ModeRaw, want: config.ModeEnriched, wantErr: false},
		{name: "empty uses placeholder", raw: "", placeholder: config.ModeRaw, want: config.ModeRaw, wantErr: false},
		{name: "invalid", raw: "llm", placeholder: config.ModeEnriched, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSetupMode(tt.raw, tt.placeholder)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSetupMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("parseSetupMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModeLabel(t *testing.T) {
	if got := modeLabel(true); got != "Raw (no LLM)" {
		t.Fatalf("modeLabel(true) = %q", got)
	}
	if got := modeLabel(false); got != "Enriched (Claude)" {
		t.Fatalf("modeLabel(false) = %q", got)
	}
}
