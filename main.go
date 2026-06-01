package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lpinto23/oncall-tui/internal/ai"
	"github.com/lpinto23/oncall-tui/internal/config"
	"github.com/lpinto23/oncall-tui/internal/model"
	"github.com/lpinto23/oncall-tui/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	onSubmit := func(answers [5]string) (string, error) {
		startTime, err := ui.ParseTime(answers[2])
		if err != nil {
			return "", fmt.Errorf("start time: %w", err)
		}
		endTime, err := ui.ParseTime(answers[3])
		if err != nil {
			return "", fmt.Errorf("end time: %w", err)
		}

		tags := parseTags(answers[4])

		inc := model.Incident{
			PagerDutyID: strings.TrimSpace(answers[0]),
			Summary:     strings.TrimSpace(answers[1]),
			StartTime:   startTime,
			EndTime:     endTime,
			Tags:        tags,
		}

		content, err := ai.EnrichIncident(inc)
		if err != nil {
			return "", fmt.Errorf("claude enrichment: %w", err)
		}

		return writeIncident(cfg.OutputDir, inc, content)
	}

	tui := ui.New(onSubmit)
	p := tea.NewProgram(tui, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func writeIncident(outputDir string, inc model.Incident, enrichedContent string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	now := time.Now()
	filename := fmt.Sprintf("INCIDENT_%s_%s.md",
		sanitize(inc.PagerDutyID),
		now.Format("2006-01-02-15-04-05"),
	)
	path := filepath.Join(outputDir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Incident %s\n\n", inc.PagerDutyID))
	sb.WriteString(fmt.Sprintf("**Logged:** %s\n\n", now.Format("2006-01-02 15:04:05 MST")))

	if inc.StartTime != nil {
		sb.WriteString(fmt.Sprintf("**Start:** %s\n", inc.StartTime.Format("2006-01-02 15:04:05 MST")))
	}
	if inc.EndTime != nil {
		sb.WriteString(fmt.Sprintf("**End:** %s\n", inc.EndTime.Format("2006-01-02 15:04:05 MST")))
	}
	if len(inc.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(inc.Tags, ", ")))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString(enrichedContent)
	sb.WriteString("\n")

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return path, nil
}

func sanitize(s string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "_",
		":", "-",
	)
	return replacer.Replace(s)
}
