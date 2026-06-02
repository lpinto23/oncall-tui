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
const aiEnrichedHeader = "## AI Enriched Report"

func main() {
	resolutionID := flag.String("resolution", "", "PagerDuty incident ID to add a resolution to")
	closeID := flag.String("close", "", "PagerDuty incident ID to close (set end time + optional resolution)")
	rawMode := flag.Bool("raw", false, "Skip Claude enrichment and generate report from raw notes only")
	enrichedMode := flag.Bool("enriched", false, "Force Claude enrichment even when default mode is raw")
	flag.Parse()

	cfg, isFirstRun, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	resolvedRawMode, err := resolveRawMode(cfg, *rawMode, *enrichedMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mode error: %v\n", err)
		os.Exit(1)
	}

	opts := ui.Options{
		RawMode:    resolvedRawMode,
		ModeLocked: *rawMode || *enrichedMode,
	}

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

	onSubmit := func(outputDir, existingFile string, answers [7]string, rawMode bool) (string, error) {
		incidentID := strings.TrimSpace(answers[0])
		summary := strings.TrimSpace(answers[1])

		// Resolution-only mode: append resolution section to existing file.
		if opts.ResolutionMode && existingFile != "" {
			resolution := strings.TrimSpace(answers[6])
			return appendResolution(existingFile, resolution)
		}

		// Close mode: patch end time in header + append resolution section.
		if opts.CloseMode && existingFile != "" {
			endTime, err := ui.ParseEndTime(answers[3])
			if err != nil {
				return "", fmt.Errorf("end time: %w", err)
			}
			resolution := strings.TrimSpace(answers[6])
			return closeIncident(existingFile, endTime, resolution)
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
			PagerDutyID:      incidentID,
			Summary:          summary,
			StartTime:        startTime,
			EndTime:          endTime,
			AffectedServices: parseAffectedServices(answers[4]),
			Tags:             parseTags(answers[5]),
			Resolution:       strings.TrimSpace(answers[6]),
		}

		content, err := buildIncidentContent(inc, rawMode)
		if err != nil {
			return "", err
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
	return parseCommaSeparated(raw)
}

func parseAffectedServices(raw string) []string {
	return parseCommaSeparated(raw)
}

func parseCommaSeparated(raw string) []string {
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
	if len(inc.AffectedServices) > 0 {
		sb.WriteString(fmt.Sprintf("**Affected Services:** %s\n", strings.Join(inc.AffectedServices, ", ")))
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

func closeIncident(path string, endTime *time.Time, resolution string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	resolution = strings.TrimSpace(resolution)
	resolvedAt := time.Now()
	finalEnd := resolvedAt
	if endTime != nil {
		finalEnd = *endTime
	} else if resolution == "" {
		finalEnd = time.Now()
	}

	updated, err := setHeaderEndTime(string(data), finalEnd)
	if err != nil {
		return "", err
	}

	// Append resolution section if provided.
	if resolution != "" {
		updated = upsertResolutionSection(updated, resolution, resolvedAt)
	}

	if err := writeFileAtomically(path, []byte(updated)); err != nil {
		return "", fmt.Errorf("updating file: %w", err)
	}
	return path, nil
}

func sanitize(s string) string { return util.Sanitize(s) }

func resolveRawMode(cfg config.Config, rawFlag, enrichedFlag bool) (bool, error) {
	if rawFlag && enrichedFlag {
		return false, errors.New("--raw and --enriched cannot be used together")
	}

	rawMode := config.NormalizeMode(cfg.DefaultMode) == config.ModeRaw
	if rawFlag {
		rawMode = true
	}
	if enrichedFlag {
		rawMode = false
	}

	return rawMode, nil
}

func buildIncidentContent(inc model.Incident, rawMode bool) (string, error) {
	if rawMode {
		return buildRawIncidentContent(inc), nil
	}

	content, err := ai.EnrichIncident(inc)
	if err != nil {
		return "", fmt.Errorf("claude enrichment: %w", err)
	}
	return buildEnrichedIncidentContent(inc, content), nil
}

func buildEnrichedIncidentContent(inc model.Incident, content string) string {
	rawSection := buildOriginalRawInputSection(inc)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return strings.TrimSpace(rawSection + "\n\n" + aiEnrichedHeader)
	}

	if strings.HasPrefix(trimmed, aiEnrichedHeader) {
		return strings.TrimSpace(rawSection + "\n\n" + trimmed)
	}

	return strings.TrimSpace(rawSection + "\n\n" + aiEnrichedHeader + "\n\n" + trimmed)
}

func buildRawIncidentContent(inc model.Incident) string {
	startTime, endTime, duration := incidentTimeSummary(inc)
	affectedServices := "N/A"
	if len(inc.AffectedServices) > 0 {
		affectedServices = strings.Join(inc.AffectedServices, ", ")
	}
	tags := "N/A"
	if len(inc.Tags) > 0 {
		tags = strings.Join(inc.Tags, ", ")
	}

	summaryLine := strings.TrimSpace(inc.Summary)
	if summaryLine == "" {
		summaryLine = "No summary provided."
	}

	var sb strings.Builder
	sb.WriteString("## Description\n\n")
	sb.WriteString(summaryLine + "\n\n")

	sb.WriteString("## Incident Details\n\n")
	sb.WriteString("| Field       | Value |\n")
	sb.WriteString("| ----------- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Incident ID | %s |\n", escapeTableCell(inc.PagerDutyID)))
	sb.WriteString(fmt.Sprintf("| Start Time  | %s |\n", escapeTableCell(startTime)))
	sb.WriteString(fmt.Sprintf("| End Time    | %s |\n", escapeTableCell(endTime)))
	sb.WriteString(fmt.Sprintf("| Affected Services | %s |\n", escapeTableCell(affectedServices)))
	sb.WriteString(fmt.Sprintf("| Duration    | %s |\n", escapeTableCell(duration)))
	sb.WriteString(fmt.Sprintf("| Tags        | %s |\n\n", escapeTableCell(tags)))

	sb.WriteString("## Impact\n\n")
	sb.WriteString("No additional impact analysis generated (`--raw` mode).\n\n")

	sb.WriteString("## Timeline\n\n")
	sb.WriteString("| Time | Event |\n")
	sb.WriteString("| ---- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| %s | Incident begins — %s |\n", escapeTableCell(startTime), escapeTableCell(singleLine(summaryLine))))
	if inc.EndTime != nil {
		sb.WriteString(fmt.Sprintf("| %s | Incident resolved |\n", escapeTableCell(endTime)))
	} else {
		sb.WriteString("| TBD | Incident resolved |\n")
	}

	if strings.TrimSpace(inc.Resolution) != "" {
		sb.WriteString("\n## Resolution\n\n")
		sb.WriteString(strings.TrimSpace(inc.Resolution) + "\n")
	}

	return sb.String()
}

func buildOriginalRawInputSection(inc model.Incident) string {
	summary := strings.TrimSpace(inc.Summary)
	if summary == "" {
		summary = "N/A"
	}

	affectedServices := "N/A"
	if len(inc.AffectedServices) > 0 {
		affectedServices = strings.Join(inc.AffectedServices, ", ")
	}

	tags := "N/A"
	if len(inc.Tags) > 0 {
		tags = strings.Join(inc.Tags, ", ")
	}

	resolution := strings.TrimSpace(inc.Resolution)
	if resolution == "" {
		resolution = "N/A"
	}

	start := "N/A"
	if inc.StartTime != nil {
		start = inc.StartTime.Format(incidentTimeLayout)
	}

	end := "Still open"
	if inc.EndTime != nil {
		end = inc.EndTime.Format(incidentTimeLayout)
	}

	var sb strings.Builder
	sb.WriteString("## Original Raw Input\n\n")
	sb.WriteString("```text\n")
	sb.WriteString("Summary: " + summary + "\n")
	sb.WriteString("Start Time: " + start + "\n")
	sb.WriteString("End Time: " + end + "\n")
	sb.WriteString("Affected Services: " + affectedServices + "\n")
	sb.WriteString("Tags: " + tags + "\n")
	sb.WriteString("Resolution: " + resolution + "\n")
	sb.WriteString("```")
	return sb.String()
}

func incidentTimeSummary(inc model.Incident) (startTime, endTime, duration string) {
	startTime = "N/A"
	endTime = "Still open"
	duration = "Ongoing"

	if inc.StartTime != nil {
		startTime = inc.StartTime.Format(incidentTimeLayout)
	}
	if inc.EndTime != nil {
		endTime = inc.EndTime.Format(incidentTimeLayout)
	}
	if inc.StartTime != nil && inc.EndTime != nil {
		d := inc.EndTime.Sub(*inc.StartTime)
		switch {
		case d < 0:
			duration = "Invalid"
		case d < time.Minute:
			duration = "<1 minute"
		default:
			h := int(d.Hours())
			m := int(d.Minutes()) % 60
			if h > 0 {
				duration = fmt.Sprintf("~%dh %dm", h, m)
			} else {
				duration = fmt.Sprintf("~%d minutes", int(d.Minutes()))
			}
		}
	}

	return startTime, endTime, duration
}

func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

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
