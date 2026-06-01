# oncall-tui

A terminal UI tool for logging on-call incidents. Fill in a short form, and the tool pipes your notes to the local Claude Code CLI to produce a well-structured markdown incident report — saved to a folder of your choice.

![Confirm screen](docs/screenshots/confirm.gif)

---

## Features

- Interactive multi-step TUI powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- First-run setup prompt to configure where incidents are saved
- Prompts for PagerDuty incident number, summary, start/end times, and tags
- Start time defaults to now — press `→` to accept and edit
- Leaving end time blank marks the incident as **still open**; press `→` to fill current time if it has resolved
- Enriches your notes via the local `claude` CLI (no API key needed)
- Saves the result as `INCIDENT_{ID}_YYYY-MM-DD-HH-MM-SS.md`
- Confirm screen shows all fields including the save location before submitting
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
make install          # builds and copies binary to /usr/local/bin
```

To upgrade after pulling new changes:

```bash
make upgrade          # git pull + rebuild + reinstall
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

### First run

On the first run you will be prompted to choose where incident files are saved. The default (`~/oncall-incidents`) is shown as a placeholder — press `→` to accept it or type a custom path, then `Enter` to confirm. The choice is saved to `~/.config/oncall-tui/config.json` and never asked again.

![Setup screen](docs/screenshots/setup.gif)

### Incident form

![Incident form](docs/screenshots/form.gif)

The tool then walks you through five fields:

| Step | Field | Required |
|------|-------|----------|
| 1 | PagerDuty incident number | Yes |
| 2 | Short summary of what happened | Yes |
| 3 | Incident start time (`YYYY-MM-DD HH:MM`) | No — defaults to now |
| 4 | Incident end time (`YYYY-MM-DD HH:MM`) | No — blank = still open |
| 5 | Tags (comma-separated) | No |

After confirming, the tool calls Claude and writes the enriched report to your configured directory.

### Time field behaviour

| Scenario | Result |
|----------|--------|
| Start time left blank | Defaults to the current time |
| Start time `→` pressed | Fills current time for editing |
| End time left blank | Logged as **Still open** — no end time written |
| End time `→` pressed | Fills current time for editing (use when incident is resolved) |
| Either time typed manually | Parsed as `YYYY-MM-DD HH:MM` |

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Advance to next step / confirm |
| `→` | Accept the placeholder value for the current field |
| `Tab` / `↓` | Move to next field |
| `Shift+Tab` / `↑` | Move to previous field |
| `Esc` / `Ctrl+C` | Quit |

---

## Output format

Files are saved as:

```
INCIDENT_{PAGERDUTY_ID}_{YYYY-MM-DD-HH-MM-SS}.md
```

Resolved incident (`INCIDENT_PD-12345_2025-06-01-14-30-00.md`):

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

Still-open incident (end time left blank):

```markdown
# Incident PD-12345

**Logged:** 2025-06-01 14:30:00 UTC

**Start:** 2025-06-01 13:00:00 UTC
**End:** Still open
**Tags:** database, p1, latency

---

Database latency spike caused elevated error rates on the checkout service.

## Description

At approximately 13:00 UTC, the primary database began exhibiting elevated query latencies...
```

---

## Configuration

The config file lives at:

```
~/.config/oncall-tui/config.json
```

It is created automatically on first run via the setup prompt. To change the output directory afterwards, edit the file directly:

```json
{
  "output_dir": "/path/to/your/incidents"
}
```

---

## How enrichment works

After you confirm the form, the tool pipes the raw incident notes as stdin to the local `claude` CLI:

```bash
echo "<prompt>" | claude -p --output-format text
```

No API key needed — it uses whatever Claude Code session is already authenticated on your machine.

---

## Development

```bash
make build    # build binary locally
make fmt      # format source
make vet      # run go vet
make clean    # remove built binary
```
