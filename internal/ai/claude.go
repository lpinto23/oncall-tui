package ai

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lpinto23/oncall-tui/internal/model"
)

func EnrichIncident(incident model.Incident) (string, error) {
	prompt := buildPrompt(incident)

	cmd := exec.Command("claude", "-p", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("claude: %s", msg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func buildPrompt(inc model.Incident) string {
	var sb strings.Builder

	sb.WriteString("You are an on-call engineer writing an incident report. ")
	sb.WriteString("Given the raw incident notes below, produce a well-structured markdown incident report.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Keep the original summary verbatim as the intro paragraph.\n")
	sb.WriteString("- Add a ## Description section with a clear, professional explanation of what happened.\n")
	sb.WriteString("- Do NOT invent facts. Only expand on what is provided.\n")
	sb.WriteString("- Use plain, technical language.\n\n")

	sb.WriteString("--- RAW NOTES ---\n")
	sb.WriteString(fmt.Sprintf("PagerDuty Incident: %s\n", inc.PagerDutyID))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", inc.Summary))

	if inc.StartTime != nil {
		sb.WriteString(fmt.Sprintf("Start Time: %s\n", inc.StartTime.Format("2006-01-02 15:04:05 MST")))
	}
	if inc.EndTime != nil {
		sb.WriteString(fmt.Sprintf("End Time: %s\n", inc.EndTime.Format("2006-01-02 15:04:05 MST")))
	}
	if len(inc.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(inc.Tags, ", ")))
	}

	sb.WriteString("--- END RAW NOTES ---\n\n")
	sb.WriteString("Produce only the markdown body (no outer code fences). Start directly with the intro paragraph.")

	return sb.String()
}
