package ui

import (
	"strings"
	"testing"

	"github.com/lpinto23/oncall-tui/internal/config"
)

func TestModeIndexFromValue(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want int
	}{
		{name: "raw", mode: config.ModeRaw, want: 1},
		{name: "enriched", mode: config.ModeEnriched, want: 0},
		{name: "invalid defaults", mode: "llm", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modeIndexFromValue(tt.mode)
			if got != tt.want {
				t.Fatalf("modeIndexFromValue() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestModeValueFromIndex(t *testing.T) {
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{name: "enriched", idx: 0, want: config.ModeEnriched},
		{name: "raw", idx: 1, want: config.ModeRaw},
		{name: "negative defaults", idx: -1, want: config.ModeEnriched},
		{name: "overflow defaults", idx: 99, want: config.ModeEnriched},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modeValueFromIndex(tt.idx)
			if got != tt.want {
				t.Fatalf("modeValueFromIndex() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderSetupModeSelector(t *testing.T) {
	got := renderSetupModeSelector(1)
	if !strings.Contains(got, "Raw") {
		t.Fatalf("selector should include Raw option: %q", got)
	}
	if !strings.Contains(got, "Enriched") {
		t.Fatalf("selector should include Enriched option: %q", got)
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

func TestCanSelectOutputMode(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want bool
	}{
		{name: "default incident flow", opts: Options{}, want: true},
		{name: "locked by flag", opts: Options{ModeLocked: true}, want: false},
		{name: "close flow", opts: Options{CloseMode: true}, want: false},
		{name: "resolution flow", opts: Options{ResolutionMode: true}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canSelectOutputMode(tt.opts)
			if got != tt.want {
				t.Fatalf("canSelectOutputMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyModeSelection(t *testing.T) {
	tests := []struct {
		name        string
		rawMode     bool
		key         string
		wantMode    bool
		wantChanged bool
	}{
		{name: "r sets raw", rawMode: false, key: "r", wantMode: true, wantChanged: true},
		{name: "R sets raw", rawMode: false, key: "R", wantMode: true, wantChanged: true},
		{name: "left sets raw", rawMode: false, key: "left", wantMode: true, wantChanged: true},
		{name: "e sets enriched", rawMode: true, key: "e", wantMode: false, wantChanged: true},
		{name: "E sets enriched", rawMode: true, key: "E", wantMode: false, wantChanged: true},
		{name: "right sets enriched", rawMode: true, key: "right", wantMode: false, wantChanged: true},
		{name: "no change raw", rawMode: true, key: "r", wantMode: true, wantChanged: false},
		{name: "no change enriched", rawMode: false, key: "e", wantMode: false, wantChanged: false},
		{name: "irrelevant key", rawMode: true, key: "x", wantMode: true, wantChanged: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotChanged := applyModeSelection(tt.rawMode, tt.key)
			if gotMode != tt.wantMode || gotChanged != tt.wantChanged {
				t.Fatalf("applyModeSelection(%v, %q) = (%v, %v), want (%v, %v)", tt.rawMode, tt.key, gotMode, gotChanged, tt.wantMode, tt.wantChanged)
			}
		})
	}
}
