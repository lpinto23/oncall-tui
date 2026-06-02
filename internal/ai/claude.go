package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lpinto23/oncall-tui/internal/model"
)

const claudeTimeout = 5 * time.Minute

func EnrichIncident(incident model.Incident) (string, error) {
	prompt := buildPrompt(incident)

	ctx, cancel := context.WithTimeout(context.Background(), claudeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("claude: timed out after %s", claudeTimeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = fmt.Sprintf("exit code %d", exitErr.ExitCode())
			}
			return "", fmt.Errorf("claude: %s", msg)
		}
		return "", fmt.Errorf("claude: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func buildPrompt(inc model.Incident) string {
	var sb strings.Builder

	startTime := "N/A"
	endTime := "Still open"
	duration := "Ongoing"

	if inc.StartTime != nil {
		startTime = inc.StartTime.Format("2006-01-02 15:04:05 MST")
	}
	if inc.EndTime != nil {
		endTime = inc.EndTime.Format("2006-01-02 15:04:05 MST")
		if inc.StartTime != nil && inc.EndTime.After(*inc.StartTime) {
			d := inc.EndTime.Sub(*inc.StartTime)
			h := int(d.Hours())
			m := int(d.Minutes()) % 60
			if h > 0 {
				duration = fmt.Sprintf("~%dh %dm", h, m)
			} else {
				duration = fmt.Sprintf("~%d minutes", int(d.Minutes()))
			}
		}
	}

	tags := "N/A"
	if len(inc.Tags) > 0 {
		tags = strings.Join(inc.Tags, ", ")
	}

	affectedServices := "N/A"
	if len(inc.AffectedServices) > 0 {
		affectedServices = strings.Join(inc.AffectedServices, ", ")
	}

	sb.WriteString("You are an on-call engineer writing a structured incident report.\n")
	sb.WriteString("Fill in the template below using ONLY the information from the raw notes. Do NOT invent facts.\n")
	sb.WriteString("Use plain, technical language. Expand abbreviations where obvious but do not guess.\n\n")

	sb.WriteString("--- RAW NOTES ---\n")
	sb.WriteString(fmt.Sprintf("Incident ID: %s\n", inc.PagerDutyID))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", inc.Summary))
	sb.WriteString(fmt.Sprintf("Start Time: %s\n", startTime))
	sb.WriteString(fmt.Sprintf("End Time: %s\n", endTime))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", duration))
	sb.WriteString(fmt.Sprintf("Affected Services: %s\n", affectedServices))
	sb.WriteString(fmt.Sprintf("Tags: %s\n", tags))
	if inc.Resolution != "" {
		sb.WriteString(fmt.Sprintf("Resolution: %s\n", inc.Resolution))
	}
	sb.WriteString("--- END RAW NOTES ---\n\n")

	sb.WriteString("Produce ONLY the markdown body below. No code fences. Follow this exact structure:\n\n")

	// Intro paragraph
	sb.WriteString("{One paragraph intro — the original summary verbatim, followed by one sentence of context if inferable.}\n\n")

	// Description
	sb.WriteString("## Description\n\n")
	sb.WriteString("{Two to four sentences describing what happened, which systems were affected, and the user/business impact. Derive from summary, affected services, and tags only.}\n\n")

	// Incident Details table
	sb.WriteString("## Incident Details\n\n")
	sb.WriteString("| Field       | Value |\n")
	sb.WriteString("| ----------- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Incident ID | %s |\n", inc.PagerDutyID))
	sb.WriteString(fmt.Sprintf("| Start Time  | %s |\n", startTime))
	sb.WriteString(fmt.Sprintf("| End Time    | %s |\n", endTime))
	sb.WriteString(fmt.Sprintf("| Affected Services | %s |\n", affectedServices))
	sb.WriteString(fmt.Sprintf("| Duration    | %s |\n", duration))
	sb.WriteString(fmt.Sprintf("| Tags        | %s |\n\n", tags))

	// Impact
	sb.WriteString("## Impact\n\n")
	sb.WriteString("{One to two sentences on the user-facing or business impact inferred from the summary, affected services, and tags.}\n\n")

	// Timeline
	sb.WriteString("## Timeline\n\n")
	sb.WriteString("| Time | Event |\n")
	sb.WriteString("| ---- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| %s | Incident begins — {brief description from summary} |\n", startTime))
	if inc.EndTime != nil {
		sb.WriteString(fmt.Sprintf("| %s | Incident resolved |\n", endTime))
	} else {
		sb.WriteString("| TBD | Incident resolved |\n")
	}
	sb.WriteString("\n")

	// Resolution (only if provided)
	if inc.Resolution != "" {
		sb.WriteString("## Resolution\n\n")
		sb.WriteString(fmt.Sprintf("{Rewrite the following resolution notes in clear technical prose, do not add facts}: %s\n", inc.Resolution))
	}

	return sb.String()
}
