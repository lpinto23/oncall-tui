# oncall-tui

A terminal UI tool for logging on-call incidents. Fill in a short form, and the tool calls Claude AI to expand your notes into a well-structured markdown incident report — saved to a folder of your choice.

---

## Features

- Interactive multi-step TUI powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Prompts for PagerDuty incident number, summary, start/end times, and tags
- Sends your notes to Claude (Haiku by default) via the Anthropic API to produce a consistent, professional report
- Saves the result as `INCIDENT_{ID}_YYYY-MM-DD-HH-MM-SS.md`
- Configurable output directory and model via a JSON config file
- Back-navigation between fields with `Tab` / `Shift+Tab`

---

## Requirements

- Go 1.21+
- [Claude Code](https://claude.ai/code) CLI installed and authenticated (`claude` must be on your `PATH`)

---

## Installation

```bash
git clone https://github.com/lpinto23/oncall-tui.git
cd oncall-tui
make install          # copies binary to /usr/local/bin
```

To uninstall:

```bash
make uninstall
```

---

## Usage

```bash
oncall-tui
```

The tool walks you through five prompts:

| Step | Field | Required |
|------|-------|----------|
| 1 | PagerDuty incident number | Yes |
| 2 | Short summary of what happened | Yes |
| 3 | Incident start time (`YYYY-MM-DD HH:MM`) | No |
| 4 | Incident end time (`YYYY-MM-DD HH:MM`) | No |
| 5 | Tags (comma-separated) | No |

After confirming, the tool calls Claude AI and writes the report to your configured output directory.

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Advance to next step / confirm |
| `Tab` / `↓` | Move to next field |
| `Shift+Tab` / `↑` | Move to previous field |
| `Esc` / `Ctrl+C` | Quit |

---

## Output format

Files are saved as:

```
INCIDENT_{PAGERDUTY_ID}_{YYYY-MM-DD-HH-MM-SS}.md
```

Example file (`INCIDENT_PD-12345_2025-06-01-14-30-00.md`):

```markdown
# Incident PD-12345

**Logged:** 2025-06-01 14:30:00 UTC

**Start:** 2025-06-01 13:00:00 UTC
**End:** 2025-06-01 14:15:00 UTC
**Tags:** database, p1, latency

---

Database latency spike caused elevated error rates on the checkout service.

## Description

At approximately 13:00 UTC, the primary database began exhibiting elevated query latencies...
```

---

## Configuration

On first run, the config file is created automatically at:

```
~/.config/oncall-tui/config.json
```

Default contents:

```json
{
  "output_dir": "~/oncall-incidents"
}
```

| Field | Description |
|-------|-------------|
| `output_dir` | Directory where incident `.md` files are saved |

---

## Development

```bash
make build    # build binary locally
make fmt      # format source
make vet      # run go vet
make clean    # remove built binary
```

---

## How enrichment works

The tool pipes the raw incident notes as stdin to the local `claude` CLI:

```bash
echo "<prompt>" | claude -p --output-format text
```

No API key setup needed — it uses whatever Claude Code session is already authenticated on your machine.
