package config

import "testing"

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "raw", in: "raw", want: ModeRaw},
		{name: "enriched", in: "enriched", want: ModeEnriched},
		{name: "mixed case", in: " RaW ", want: ModeRaw},
		{name: "invalid falls back", in: "unknown", want: ModeEnriched},
		{name: "empty falls back", in: "", want: ModeEnriched},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMode(tt.in)
			if got != tt.want {
				t.Fatalf("NormalizeMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsValidMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "raw", in: "raw", want: true},
		{name: "enriched", in: "enriched", want: true},
		{name: "trimmed", in: " EnRiChEd ", want: true},
		{name: "invalid", in: "raw-mode", want: false},
		{name: "empty", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidMode(tt.in)
			if got != tt.want {
				t.Fatalf("IsValidMode(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
