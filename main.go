package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lpinto23/oncall-tui/internal/ai"
	"github.com/lpinto23/oncall-tui/internal/config"
	"github.com/lpinto23/oncall-tui/internal/finder"
	"github.com/lpinto23/oncall-tui/internal/model"
	"github.com/lpinto23/oncall-tui/internal/ui"
	"github.com/lpinto23/oncall-tui/internal/util"
)

const incidentTimeLayout = "2006-01-02 15:04:05 MST"

func main() {
	resolutionID := flag.String("resolution", "", "PagerDuty incident ID to add a resolution to")
	closeID := flag.String("close", "", "PagerDuty incident ID to close (set end time + optional resolution)")
	flag.Parse()

	cfg, isFirstRun, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	opts := ui.Options{}

	switch {
	case *resolutionID != "":
		opts.ResolutionMode = true
		opts.IncidentID = *resolutionID
		existingFile, err := finder.FindByID(cfg.OutputDir, *resolutionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lookup error: %v\n", err)
			os.Exit(1)
		}
		opts.ExistingFile = existingFile

	case *closeID != "":
		opts.CloseMode = true
		opts.IncidentID = *closeID
		existingFile, err := finder.FindByID(cfg.OutputDir, *closeID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lookup error: %v\n", err)
			os.Exit(1)
		}
		opts.ExistingFile = existingFile
	}

	onSubmit := func(outputDir, existingFile string, answers [6]string) (string, error) {
		incidentID := strings.TrimSpace(answers[0])
		summary := strings.TrimSpace(answers[1])

		// Resolution-only mode: append resolution section to existing file.
		if opts.ResolutionMode && existingFile != "" {
			resolution := strings.TrimSpace(answers[5])
			return appendResolution(existingFile, resolution)
		}

		// Close mode: patch end time in header + append resolution section.
		if opts.CloseMode && existingFile != "" {
			endTime, err := ui.ParseEndTime(answers[3])
			if err != nil {
				return "", fmt.Errorf("end time: %w", err)
			}
			if endTime == nil {
				t := time.Now()
				endTime = &t
			}
			resolution := strings.TrimSpace(answers[5])
			return closeIncident(existingFile, *endTime, resolution)
		}

		startTime, err := ui.ParseTime(answers[2])
		if err != nil {
			return "", fmt.Errorf("start time: %w", err)
		}
		endTime, err := ui.ParseEndTime(answers[3])
		if err != nil {
			return "", fmt.Errorf("end time: %w", err)
		}

		if err := validateRequiredFields(incidentID, summary); err != nil {
			return "", err
		}
		if err := validateChronology(startTime, endTime); err != nil {
			return "", err
		}

		inc := model.Incident{
			PagerDutyID: incidentID,
			Summary:     summary,
			StartTime:   startTime,
			EndTime:     endTime,
			Tags:        parseTags(answers[4]),
			Resolution:  strings.TrimSpace(answers[5]),
		}

		content, err := ai.EnrichIncident(inc)
		if err != nil {
			return "", fmt.Errorf("claude enrichment: %w", err)
		}

		return writeIncident(outputDir, inc, content)
	}

	tui := ui.New(cfg, isFirstRun, opts, onSubmit)
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

func incidentBody(inc model.Incident, enrichedContent string) string {
	var sb strings.Builder
	now := time.Now()

	sb.WriteString(fmt.Sprintf("# Incident %s\n\n", inc.PagerDutyID))
	sb.WriteString(fmt.Sprintf("**Logged:** %s\n\n", now.Format(incidentTimeLayout)))

	if inc.StartTime != nil {
		sb.WriteString(fmt.Sprintf("**Start:** %s\n", inc.StartTime.Format(incidentTimeLayout)))
	}
	if inc.EndTime != nil {
		sb.WriteString(fmt.Sprintf("**End:** %s\n", inc.EndTime.Format(incidentTimeLayout)))
	} else {
		sb.WriteString("**End:** Still open\n")
	}
	if len(inc.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(inc.Tags, ", ")))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString(enrichedContent)
	sb.WriteString("\n")

	return sb.String()
}

func writeIncident(outputDir string, inc model.Incident, enrichedContent string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	filename := fmt.Sprintf("INCIDENT_%s_%s.md",
		sanitize(inc.PagerDutyID),
		time.Now().Format("2006-01-02-15-04-05"),
	)
	path := filepath.Join(outputDir, filename)

	if err := os.WriteFile(path, []byte(incidentBody(inc, enrichedContent)), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return path, nil
}

func appendResolution(path, resolution string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	updated := upsertResolutionSection(string(data), resolution, time.Now())

	if err := writeFileAtomically(path, []byte(updated)); err != nil {
		return "", fmt.Errorf("writing resolution: %w", err)
	}
	return path, nil
}

func closeIncident(path string, endTime time.Time, resolution string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	updated, err := setHeaderEndTime(string(data), endTime)
	if err != nil {
		return "", err
	}

	// Append resolution section if provided.
	if resolution != "" {
		updated = upsertResolutionSection(updated, resolution, time.Now())
	}

	if err := writeFileAtomically(path, []byte(updated)); err != nil {
		return "", fmt.Errorf("updating file: %w", err)
	}
	return path, nil
}

func sanitize(s string) string { return util.Sanitize(s) }

func validateRequiredFields(incidentID, summary string) error {
	if incidentID == "" {
		return errors.New("incident ID is required")
	}
	if summary == "" {
		return errors.New("summary is required")
	}
	return nil
}

func validateChronology(startTime, endTime *time.Time) error {
	if startTime != nil && endTime != nil && endTime.Before(*startTime) {
		return errors.New("end time must be equal to or after start time")
	}
	return nil
}

func setHeaderEndTime(content string, endTime time.Time) (string, error) {
	lines := strings.Split(content, "\n")
	headerEnd := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			headerEnd = i
			break
		}
	}

	endLine := fmt.Sprintf("**End:** %s", endTime.Format(incidentTimeLayout))
	for i := 0; i < headerEnd; i++ {
		if strings.HasPrefix(lines[i], "**End:**") {
			lines[i] = endLine
			return strings.Join(lines, "\n"), nil
		}
	}

	return "", errors.New("incident header is missing the End field")
}

func upsertResolutionSection(content, resolution string, resolvedAt time.Time) string {
	entry := buildResolutionEntry(strings.TrimSpace(resolution), resolvedAt)
	trimmed := strings.TrimRight(content, "\n")

	if hasResolutionSection(trimmed) {
		return trimmed + "\n\n" + entry + "\n"
	}

	return trimmed + "\n\n## Resolution\n\n" + entry + "\n"
}

func hasResolutionSection(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "## Resolution" {
			return true
		}
	}
	return false
}

func buildResolutionEntry(resolution string, resolvedAt time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Resolved:** %s", resolvedAt.Format(incidentTimeLayout)))
	if resolution != "" {
		sb.WriteString("\n\n")
		sb.WriteString(resolution)
	}
	return sb.String()
}

func writeFileAtomically(path string, data []byte) error {
	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".oncall-tui-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}
