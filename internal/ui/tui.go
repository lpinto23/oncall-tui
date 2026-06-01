package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lpinto23/oncall-tui/internal/config"
)

type step int

const (
	stepSetup step = iota
	stepIncidentID
	stepSummary
	stepStartTime
	stepEndTime
	stepTags
	stepConfirm
	stepProcessing
	stepDone
	stepError
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ECDC4"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#95E06C"))

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B"))

	fieldStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4ECDC4")).
			Padding(0, 1).
			MarginBottom(1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	summaryKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#C3A6FF"))
)

type SubmitResult struct {
	FilePath string
	Err      error
}

type Model struct {
	step       step
	setupInput textinput.Model
	inputs     [5]textinput.Model
	spinner    spinner.Model
	result     SubmitResult
	cfg        config.Config
	onSubmit   func(outputDir string, answers [5]string) (string, error)
	width      int
}

const (
	iIncidentID = 0
	iSummary    = 1
	iStartTime  = 2
	iEndTime    = 3
	iTags       = 4
)

func New(cfg config.Config, isFirstRun bool, onSubmit func(outputDir string, answers [5]string) (string, error)) Model {
	// setup input
	setup := textinput.New()
	setup.CharLimit = 512
	setup.Placeholder = cfg.OutputDir
	if isFirstRun {
		setup.Focus()
	}

	// incident inputs
	inputs := [5]textinput.Model{}
	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 512
		inputs[i] = t
	}

	inputs[iIncidentID].Placeholder = "e.g. PD-12345"
	inputs[iSummary].Placeholder = "Brief description of what happened"

	now := time.Now().Format("2006-01-02 15:04")
	inputs[iStartTime].Placeholder = now
	inputs[iEndTime].Placeholder = now
	inputs[iTags].Placeholder = "comma-separated, e.g. database,outage,p1 (leave blank to skip)"

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ECDC4"))

	firstStep := stepSetup
	if !isFirstRun {
		firstStep = stepIncidentID
		inputs[iIncidentID].Focus()
	}

	return Model{
		step:       firstStep,
		setupInput: setup,
		inputs:     inputs,
		spinner:    sp,
		cfg:        cfg,
		onSubmit:   onSubmit,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

type submitMsg struct {
	filePath string
	err      error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case submitMsg:
		if msg.err != nil {
			m.result = SubmitResult{Err: msg.err}
			m.step = stepError
		} else {
			m.result = SubmitResult{FilePath: msg.filePath}
			m.step = stepDone
		}
		return m, nil

	case spinner.TickMsg:
		if m.step == stepProcessing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "shift+tab":
			if m.step > stepIncidentID && m.step < stepConfirm {
				return m.movePrev()
			}

		case "down", "tab":
			if m.step >= stepIncidentID && m.step < stepConfirm {
				return m.moveNext()
			}

		case "right":
			if m.step == stepSetup {
				if m.setupInput.Value() == "" && m.setupInput.Placeholder != "" {
					m.setupInput.SetValue(m.setupInput.Placeholder)
				}
			} else if m.step <= stepTags {
				idx := int(m.step) - 1 // offset by setup step
				if m.inputs[idx].Value() == "" && m.inputs[idx].Placeholder != "" {
					m.inputs[idx].SetValue(m.inputs[idx].Placeholder)
				}
			}
		}
	}

	if m.step == stepSetup {
		var cmd tea.Cmd
		m.setupInput, cmd = m.setupInput.Update(msg)
		return m, cmd
	}

	if m.step >= stepIncidentID && m.step <= stepTags {
		idx := int(m.step) - 1 // offset by setup step
		var cmd tea.Cmd
		m.inputs[idx], cmd = m.inputs[idx].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSetup:
		dir := strings.TrimSpace(m.setupInput.Value())
		if dir == "" {
			dir = m.setupInput.Placeholder
		}
		m.cfg.OutputDir = dir
		if err := config.Save(m.cfg); err == nil {
			// saved successfully
		}
		m.step = stepIncidentID
		m.inputs[iIncidentID].Focus()

	case stepIncidentID:
		if strings.TrimSpace(m.inputs[iIncidentID].Value()) == "" {
			return m, nil
		}
		m.inputs[iIncidentID].Blur()
		m.inputs[iSummary].Focus()
		m.step = stepSummary

	case stepSummary:
		if strings.TrimSpace(m.inputs[iSummary].Value()) == "" {
			return m, nil
		}
		m.inputs[iSummary].Blur()
		m.inputs[iStartTime].Focus()
		m.step = stepStartTime

	case stepStartTime:
		m.inputs[iStartTime].Blur()
		m.inputs[iEndTime].Focus()
		m.step = stepEndTime

	case stepEndTime:
		m.inputs[iEndTime].Blur()
		m.inputs[iTags].Focus()
		m.step = stepTags

	case stepTags:
		m.inputs[iTags].Blur()
		m.step = stepConfirm

	case stepConfirm:
		m.step = stepProcessing
		answers := [5]string{
			m.inputs[iIncidentID].Value(),
			m.inputs[iSummary].Value(),
			m.inputs[iStartTime].Value(),
			m.inputs[iEndTime].Value(),
			m.inputs[iTags].Value(),
		}
		outputDir := m.cfg.OutputDir
		onSubmit := m.onSubmit
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				path, err := onSubmit(outputDir, answers)
				return submitMsg{filePath: path, err: err}
			},
		)

	case stepDone, stepError:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) movePrev() (tea.Model, tea.Cmd) {
	if m.step <= stepIncidentID {
		return m, nil
	}
	cur := int(m.step) - 1 // current inputs index
	m.inputs[cur].Blur()
	m.step--
	m.inputs[cur-1].Focus()
	return m, nil
}

func (m Model) moveNext() (tea.Model, tea.Cmd) {
	if m.step >= stepTags {
		return m, nil
	}
	cur := int(m.step) - 1 // current inputs index
	m.inputs[cur].Blur()
	m.step++
	m.inputs[cur+1].Focus()
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  oncall-tui") + "\n\n")

	switch m.step {
	case stepSetup:
		m.renderSetup(&b)
	case stepIncidentID, stepSummary, stepStartTime, stepEndTime, stepTags:
		m.renderInputStep(&b)
	case stepConfirm:
		m.renderConfirm(&b)
	case stepProcessing:
		m.renderProcessing(&b)
	case stepDone:
		m.renderDone(&b)
	case stepError:
		m.renderError(&b)
	}

	return b.String()
}

func (m Model) renderSetup(b *strings.Builder) {
	b.WriteString(labelStyle.Render("Welcome! Where should incidents be saved?") + "\n\n")
	b.WriteString(hintStyle.Render("Press → to accept the default, or type a path.") + "\n")
	b.WriteString(fieldStyle.Render(m.setupInput.View()) + "\n")
	b.WriteString(hintStyle.Render("enter: confirm  •  esc: quit") + "\n")
}

func (m Model) renderInputStep(b *strings.Builder) {
	steps := []struct {
		label string
		hint  string
		idx   int
	}{
		{"PagerDuty Incident #", "Required", iIncidentID},
		{"Summary", "Required — what happened?", iSummary},
		{"Start Time", "Optional — YYYY-MM-DD HH:MM, blank = now", iStartTime},
		{"End Time", "Optional — YYYY-MM-DD HH:MM, blank = still open", iEndTime},
		{"Tags", "Optional", iTags},
	}

	for i, s := range steps {
		active := int(m.step)-1 == i
		label := s.label
		if active {
			label = "> " + label
		} else {
			label = "  " + label
		}

		if active {
			b.WriteString(labelStyle.Render(label) + " " + hintStyle.Render("("+s.hint+")") + "\n")
			b.WriteString(fieldStyle.Render(m.inputs[s.idx].View()) + "\n")
		} else {
			val := m.inputs[s.idx].Value()
			if val == "" {
				val = dimStyle.Render("—")
			}
			b.WriteString(dimStyle.Render(label+": "+val) + "\n")
		}
	}

	b.WriteString("\n" + hintStyle.Render("enter: next  •  →: accept placeholder  •  tab/shift+tab: navigate  •  esc: quit") + "\n")
}

func (m Model) renderConfirm(b *strings.Builder) {
	b.WriteString(labelStyle.Render("Review your incident") + "\n\n")

	fields := []struct{ k, v string }{
		{"Incident #", m.inputs[iIncidentID].Value()},
		{"Summary", m.inputs[iSummary].Value()},
		{"Start Time", orDash(m.inputs[iStartTime].Value())},
		{"End Time", orStillOpen(m.inputs[iEndTime].Value())},
		{"Tags", orDash(m.inputs[iTags].Value())},
		{"Save to", m.cfg.OutputDir},
	}

	for _, f := range fields {
		b.WriteString(summaryKeyStyle.Render(fmt.Sprintf("%-14s", f.k)) + " " + f.v + "\n")
	}

	b.WriteString("\n" + hintStyle.Render("Press enter to save & enrich with Claude  •  esc to quit") + "\n")
}

func (m Model) renderProcessing(b *strings.Builder) {
	b.WriteString(m.spinner.View() + " Saving and enriching with Claude AI...\n")
	b.WriteString(hintStyle.Render("This may take a few seconds.") + "\n")
}

func (m Model) renderDone(b *strings.Builder) {
	b.WriteString(successStyle.Render("Incident logged successfully!") + "\n\n")
	b.WriteString(summaryKeyStyle.Render("File: ") + m.result.FilePath + "\n\n")
	b.WriteString(hintStyle.Render("Press enter or esc to exit.") + "\n")
}

func (m Model) renderError(b *strings.Builder) {
	b.WriteString(errorStyle.Render("Something went wrong") + "\n\n")
	b.WriteString(m.result.Err.Error() + "\n\n")
	b.WriteString(hintStyle.Render("Press enter or esc to exit.") + "\n")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func orStillOpen(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Still open"
	}
	return s
}

func ParseTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		t := time.Now()
		return &t, nil
	}
	return parseTimeStr(s)
}

func ParseEndTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	return parseTimeStr(s)
}

func parseTimeStr(s string) (*time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		t, err := time.ParseInLocation(l, s, time.Local)
		if err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unrecognised time format %q (expected YYYY-MM-DD HH:MM)", s)
}
