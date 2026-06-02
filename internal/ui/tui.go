package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lpinto23/oncall-tui/internal/config"
)

type step int

const (
	stepSetup          step = iota // first run: output directory + default mode
	stepNotFound                   // --resolution/--close: no file found, offer to create
	stepResolutionOnly             // --resolution: file found, just fill resolution
	stepCloseOnly                  // --close: file found, fill end time + optional resolution
	stepIncidentID
	stepSummary
	stepStartTime
	stepEndTime
	stepAffectedServices
	stepTags
	stepResolution
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

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD93D"))

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

// Options holds optional pre-fill data passed in from --resolution/--close mode.
type Options struct {
	ResolutionMode bool
	CloseMode      bool
	RawMode        bool
	ModeLocked     bool
	IncidentID     string
	ExistingFile   string
}

// Model holds two kinds of inputs:
//   - shortInputs: single-line textinput for ID, start, end, affected services, tags
//   - longInputs:  textarea for summary and resolution
type Model struct {
	step      step
	setupStep int // 0=output dir, 1=default mode
	closeStep int // 0=end time, 1=resolution (used only in stepCloseOnly)

	setupInput     textinput.Model
	setupModeInput textinput.Model

	// single-line fields
	idInput               textinput.Model
	startInput            textinput.Model
	endInput              textinput.Model
	affectedServicesInput textinput.Model
	tagsInput             textinput.Model

	// multiline fields
	summaryInput    textarea.Model
	resolutionInput textarea.Model

	spinner  spinner.Model
	result   SubmitResult
	cfg      config.Config
	opts     Options
	onSubmit func(outputDir, existingFile string, answers [7]string, rawMode bool) (string, error)
	width    int
}

func newTextarea(placeholder string, height int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.SetHeight(height)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	// style: remove default border so we wrap it in our own fieldStyle
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	return ta
}

func newTextinput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 512
	return ti
}

func New(cfg config.Config, isFirstRun bool, opts Options, onSubmit func(outputDir, existingFile string, answers [7]string, rawMode bool) (string, error)) Model {
	setup := newTextinput(cfg.OutputDir)
	setupModeInput := newTextinput(config.ModeEnriched)
	setupModeInput.SetValue(config.NormalizeMode(cfg.DefaultMode))

	idInput := newTextinput("e.g. PD-12345")
	now := time.Now().Format("2006-01-02 15:04")
	startInput := newTextinput(now)
	endInput := newTextinput(now)
	affectedServicesInput := newTextinput("comma-separated, e.g. checkout-api,postgres,redis (leave blank to skip)")
	tagsInput := newTextinput("comma-separated, e.g. database,outage,p1 (leave blank to skip)")

	summaryInput := newTextarea("Brief description of what happened", 4)
	resolutionInput := newTextarea("How was the problem resolved? (leave blank to skip)", 4)

	if opts.IncidentID != "" {
		idInput.SetValue(opts.IncidentID)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ECDC4"))

	var firstStep step
	var cmd tea.Cmd
	switch {
	case isFirstRun:
		firstStep = stepSetup
		setup.Focus()
	case opts.ResolutionMode && opts.ExistingFile != "":
		firstStep = stepResolutionOnly
		cmd = resolutionInput.Focus()
	case (opts.ResolutionMode || opts.CloseMode) && opts.ExistingFile == "":
		firstStep = stepNotFound
	case opts.CloseMode && opts.ExistingFile != "":
		firstStep = stepCloseOnly
		endInput.Focus()
	default:
		firstStep = stepIncidentID
		idInput.Focus()
	}
	_ = cmd

	return Model{
		step:                  firstStep,
		setupStep:             0,
		setupInput:            setup,
		setupModeInput:        setupModeInput,
		idInput:               idInput,
		startInput:            startInput,
		endInput:              endInput,
		affectedServicesInput: affectedServicesInput,
		tagsInput:             tagsInput,
		summaryInput:          summaryInput,
		resolutionInput:       resolutionInput,
		spinner:               sp,
		cfg:                   cfg,
		opts:                  opts,
		onSubmit:              onSubmit,
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
		w := msg.Width - 6 // account for border + padding
		if w < 20 {
			w = 20
		}
		m.summaryInput.SetWidth(w)
		m.resolutionInput.SetWidth(w)

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
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// esc inside a textarea blurs it; at top level quit
			if m.step == stepSummary || m.step == stepResolution || m.step == stepResolutionOnly {
				// let textarea handle it
			} else {
				return m, tea.Quit
			}

		case "ctrl+d":
			// advance from multiline fields
			return m.handleAdvance()

		case "enter":
			// single-line fields advance; textarea fields insert newline (handled below)
			switch m.step {
			case stepSetup, stepNotFound, stepIncidentID, stepStartTime, stepEndTime, stepAffectedServices, stepTags, stepConfirm, stepDone, stepError:
				return m.handleEnter()
			case stepCloseOnly:
				if m.closeStep == 0 {
					return m.handleEnter() // end time is single-line, advance
				}
				// closeStep==1 is resolution textarea, fall through to insert newline
			}
			// for stepSummary, stepResolution, stepResolutionOnly, stepCloseOnly(resolution): fall through to textarea update

		case "y", "Y":
			if m.step == stepNotFound {
				m.opts.ExistingFile = ""
				m.step = stepIncidentID
				m.idInput.Focus()
				return m, nil
			}

		case "n", "N":
			if m.step == stepNotFound {
				return m, tea.Quit
			}

		case "up", "shift+tab":
			if m.step > stepIncidentID && m.step < stepConfirm {
				return m.movePrev()
			}

		case "down", "tab":
			if m.step >= stepIncidentID && m.step < stepConfirm {
				return m.moveNext()
			}

		case "right":
			switch m.step {
			case stepCloseOnly:
				if m.endInput.Value() == "" {
					m.endInput.SetValue(m.endInput.Placeholder)
				}
			case stepSetup:
				if m.setupStep == 0 {
					if m.setupInput.Value() == "" {
						m.setupInput.SetValue(m.setupInput.Placeholder)
					}
				} else {
					if m.setupModeInput.Value() == "" {
						m.setupModeInput.SetValue(m.setupModeInput.Placeholder)
					}
				}
			case stepIncidentID:
				if m.idInput.Value() == "" {
					m.idInput.SetValue(m.idInput.Placeholder)
				}
			case stepStartTime:
				if m.startInput.Value() == "" {
					m.startInput.SetValue(m.startInput.Placeholder)
				}
			case stepEndTime:
				if m.endInput.Value() == "" {
					m.endInput.SetValue(m.endInput.Placeholder)
				}
			case stepTags:
				if m.tagsInput.Value() == "" {
					m.tagsInput.SetValue(m.tagsInput.Placeholder)
				}
			case stepAffectedServices:
				if m.affectedServicesInput.Value() == "" {
					m.affectedServicesInput.SetValue(m.affectedServicesInput.Placeholder)
				}
			}
		}
	}

	return m.updateActiveInput(msg)
}

func (m Model) updateActiveInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case stepSetup:
		if m.setupStep == 0 {
			m.setupInput, cmd = m.setupInput.Update(msg)
		} else {
			m.setupModeInput, cmd = m.setupModeInput.Update(msg)
		}
	case stepIncidentID:
		m.idInput, cmd = m.idInput.Update(msg)
	case stepSummary:
		m.summaryInput, cmd = m.summaryInput.Update(msg)
	case stepStartTime:
		m.startInput, cmd = m.startInput.Update(msg)
	case stepEndTime:
		m.endInput, cmd = m.endInput.Update(msg)
	case stepAffectedServices:
		m.affectedServicesInput, cmd = m.affectedServicesInput.Update(msg)
	case stepTags:
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	case stepResolution, stepResolutionOnly:
		m.resolutionInput, cmd = m.resolutionInput.Update(msg)
	case stepCloseOnly:
		if m.closeStep == 0 {
			m.endInput, cmd = m.endInput.Update(msg)
		} else {
			m.resolutionInput, cmd = m.resolutionInput.Update(msg)
		}
	}
	return m, cmd
}

// handleAdvance moves from multiline textarea steps to the next step.
func (m Model) handleAdvance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSummary:
		m.summaryInput.Blur()
		m.startInput.Focus()
		m.step = stepStartTime
	case stepResolution:
		m.resolutionInput.Blur()
		m.step = stepConfirm
	case stepResolutionOnly:
		m.resolutionInput.Blur()
		m.step = stepConfirm
	case stepCloseOnly:
		if m.closeStep == 1 {
			m.resolutionInput.Blur()
			m.step = stepConfirm
		}
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

		if m.setupStep == 0 {
			m.cfg.OutputDir = dir
			m.setupInput.Blur()
			m.setupStep = 1
			return m, m.setupModeInput.Focus()
		}

		mode, err := parseSetupMode(m.setupModeInput.Value(), m.setupModeInput.Placeholder)
		if err != nil {
			m.result = SubmitResult{Err: err}
			m.step = stepError
			return m, nil
		}

		m.cfg.OutputDir = dir
		m.cfg.DefaultMode = mode
		if !m.opts.ModeLocked {
			m.opts.RawMode = mode == config.ModeRaw
		}
		if err := config.Save(m.cfg); err != nil {
			m.result = SubmitResult{Err: fmt.Errorf("saving config: %w", err)}
			m.step = stepError
			return m, nil
		}
		m.step = stepIncidentID
		m.idInput.Focus()

	case stepNotFound:
		m.opts.ExistingFile = ""
		m.step = stepIncidentID
		m.idInput.Focus()

	case stepCloseOnly:
		if m.closeStep == 0 {
			// end time confirmed — move to resolution textarea
			m.endInput.Blur()
			m.closeStep = 1
			return m, m.resolutionInput.Focus()
		}
		// resolution confirmed via enter (blank line) — go to confirm
		m.resolutionInput.Blur()
		m.step = stepConfirm

	case stepIncidentID:
		if strings.TrimSpace(m.idInput.Value()) == "" {
			return m, nil
		}
		m.idInput.Blur()
		return m, m.summaryInput.Focus()

	case stepStartTime:
		m.startInput.Blur()
		m.endInput.Focus()
		m.step = stepEndTime

	case stepEndTime:
		m.endInput.Blur()
		m.affectedServicesInput.Focus()
		m.step = stepAffectedServices

	case stepAffectedServices:
		m.affectedServicesInput.Blur()
		m.tagsInput.Focus()
		m.step = stepTags

	case stepTags:
		m.tagsInput.Blur()
		m.step = stepResolution
		return m, m.resolutionInput.Focus()

	case stepConfirm:
		m.step = stepProcessing
		answers := [7]string{
			m.idInput.Value(),
			m.summaryInput.Value(),
			m.startInput.Value(),
			m.endInput.Value(),
			m.affectedServicesInput.Value(),
			m.tagsInput.Value(),
			m.resolutionInput.Value(),
		}
		outputDir := m.cfg.OutputDir
		existingFile := m.opts.ExistingFile
		onSubmit := m.onSubmit
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				path, err := onSubmit(outputDir, existingFile, answers, m.opts.RawMode)
				return submitMsg{filePath: path, err: err}
			},
		)

	case stepDone, stepError:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) movePrev() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSummary:
		m.summaryInput.Blur()
		m.idInput.Focus()
		m.step = stepIncidentID
	case stepStartTime:
		m.startInput.Blur()
		return m, m.summaryInput.Focus()
	case stepEndTime:
		m.endInput.Blur()
		m.startInput.Focus()
		m.step = stepStartTime
	case stepAffectedServices:
		m.affectedServicesInput.Blur()
		m.endInput.Focus()
		m.step = stepEndTime
	case stepTags:
		m.tagsInput.Blur()
		m.affectedServicesInput.Focus()
		m.step = stepAffectedServices
	case stepResolution:
		m.resolutionInput.Blur()
		m.tagsInput.Focus()
		m.step = stepTags
	}
	return m, nil
}

func (m Model) moveNext() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepIncidentID:
		if strings.TrimSpace(m.idInput.Value()) == "" {
			return m, nil
		}
		m.idInput.Blur()
		m.step = stepSummary
		return m, m.summaryInput.Focus()
	case stepSummary:
		m.summaryInput.Blur()
		m.startInput.Focus()
		m.step = stepStartTime
	case stepStartTime:
		m.startInput.Blur()
		m.endInput.Focus()
		m.step = stepEndTime
	case stepEndTime:
		m.endInput.Blur()
		m.affectedServicesInput.Focus()
		m.step = stepAffectedServices
	case stepAffectedServices:
		m.affectedServicesInput.Blur()
		m.tagsInput.Focus()
		m.step = stepTags
	case stepTags:
		m.tagsInput.Blur()
		m.step = stepResolution
		return m, m.resolutionInput.Focus()
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("  oncall-tui") + "\n\n")

	switch m.step {
	case stepSetup:
		m.renderSetup(&b)
	case stepNotFound:
		m.renderNotFound(&b)
	case stepResolutionOnly:
		m.renderResolutionOnly(&b)
	case stepCloseOnly:
		m.renderCloseOnly(&b)
	case stepIncidentID, stepSummary, stepStartTime, stepEndTime, stepAffectedServices, stepTags, stepResolution:
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
	b.WriteString(labelStyle.Render("Welcome! Let's configure oncall-tui") + "\n\n")
	b.WriteString(hintStyle.Render("Set save location and default report mode.") + "\n\n")

	if m.setupStep == 0 {
		b.WriteString(labelStyle.Render("> Output Directory") + "\n")
		b.WriteString(fieldStyle.Render(m.setupInput.View()) + "\n")
		b.WriteString(dimStyle.Render("  Default Mode: "+config.NormalizeMode(m.setupModeInput.Value())) + "\n")
		b.WriteString(hintStyle.Render("enter: next  •  →: accept placeholder  •  esc: quit") + "\n")
		return
	}

	b.WriteString(dimStyle.Render("  Output Directory: "+orDash(m.cfg.OutputDir)) + "\n\n")
	b.WriteString(labelStyle.Render("> Default Mode") + " " + hintStyle.Render("(raw or enriched)") + "\n")
	b.WriteString(fieldStyle.Render(m.setupModeInput.View()) + "\n")
	b.WriteString(hintStyle.Render("enter: save setup  •  →: accept placeholder  •  esc: quit") + "\n")
}

func (m Model) renderCloseOnly(b *strings.Builder) {
	b.WriteString(labelStyle.Render("Closing incident "+m.opts.IncidentID) + "\n")
	b.WriteString(dimStyle.Render(m.opts.ExistingFile) + "\n\n")

	if m.closeStep == 0 {
		b.WriteString(labelStyle.Render("> End Time") + " " + hintStyle.Render("(YYYY-MM-DD HH:MM, blank = now)") + "\n")
		b.WriteString(fieldStyle.Render(m.endInput.View()) + "\n")
		b.WriteString(hintStyle.Render("enter: next  •  →: accept placeholder  •  esc: quit") + "\n")
	} else {
		b.WriteString(dimStyle.Render("  End Time: "+orNow(m.endInput.Value())) + "\n\n")
		b.WriteString(labelStyle.Render("> Resolution") + " " + hintStyle.Render("(optional — how was it resolved?)") + "\n")
		b.WriteString(fieldStyle.Render(m.resolutionInput.View()) + "\n")
		b.WriteString(hintStyle.Render("enter: newline  •  ctrl+d: done  •  esc: quit") + "\n")
	}
}

func (m Model) renderResolutionOnly(b *strings.Builder) {
	b.WriteString(labelStyle.Render("Adding resolution to "+m.opts.IncidentID) + "\n")
	b.WriteString(dimStyle.Render(m.opts.ExistingFile) + "\n\n")
	b.WriteString(labelStyle.Render("> Resolution") + " " + hintStyle.Render("(how was it resolved?)") + "\n")
	b.WriteString(fieldStyle.Render(m.resolutionInput.View()) + "\n")
	b.WriteString(hintStyle.Render("enter: newline  •  ctrl+d: done  •  esc: quit") + "\n")
}

func (m Model) renderNotFound(b *strings.Builder) {
	b.WriteString(warningStyle.Render("No incident file found for "+m.opts.IncidentID) + "\n\n")
	b.WriteString("Would you like to create a new entry with this ID pre-filled?\n\n")
	b.WriteString(labelStyle.Render("[Y] Yes, create new") + "   " + dimStyle.Render("[N] No, quit") + "\n\n")
	b.WriteString(hintStyle.Render("y/enter: create  •  n/esc: quit") + "\n")
}

type fieldDef struct {
	label string
	hint  string
}

func (m Model) renderInputStep(b *strings.Builder) {
	fields := []fieldDef{
		{"PagerDuty Incident #", "Required"},
		{"Summary", "Required — enter: newline, ctrl+d: next field"},
		{"Start Time", "Optional — YYYY-MM-DD HH:MM, blank = now"},
		{"End Time", "Optional — YYYY-MM-DD HH:MM, blank = still open"},
		{"Affected Services", "Optional — comma-separated"},
		{"Tags", "Optional — comma-separated"},
		{"Resolution", "Optional — enter: newline, ctrl+d: next field"},
	}

	steps := []step{stepIncidentID, stepSummary, stepStartTime, stepEndTime, stepAffectedServices, stepTags, stepResolution}

	for i, s := range steps {
		f := fields[i]
		active := m.step == s
		label := "  " + f.label
		if active {
			label = "> " + f.label
		}

		if active {
			b.WriteString(labelStyle.Render(label) + " " + hintStyle.Render("("+f.hint+")") + "\n")
			b.WriteString(fieldStyle.Render(m.viewForStep(s)) + "\n")
		} else {
			val := m.valueForStep(s)
			if val == "" {
				val = dimStyle.Render("—")
			} else {
				// collapse multiline to single line for inactive display
				val = strings.ReplaceAll(val, "\n", " ↵ ")
			}
			b.WriteString(dimStyle.Render(label+": "+val) + "\n")
		}
	}

	hint := "enter: next  •  →: accept placeholder  •  tab/shift+tab: navigate  •  esc: quit"
	if m.step == stepSummary || m.step == stepResolution {
		hint = "enter: newline  •  ctrl+d: done  •  tab/shift+tab: navigate  •  esc: quit"
	}
	b.WriteString("\n" + hintStyle.Render(hint) + "\n")
}

func (m Model) viewForStep(s step) string {
	switch s {
	case stepIncidentID:
		return m.idInput.View()
	case stepSummary:
		return m.summaryInput.View()
	case stepStartTime:
		return m.startInput.View()
	case stepEndTime:
		return m.endInput.View()
	case stepAffectedServices:
		return m.affectedServicesInput.View()
	case stepTags:
		return m.tagsInput.View()
	case stepResolution:
		return m.resolutionInput.View()
	}
	return ""
}

func (m Model) valueForStep(s step) string {
	switch s {
	case stepIncidentID:
		return m.idInput.Value()
	case stepSummary:
		return m.summaryInput.Value()
	case stepStartTime:
		return m.startInput.Value()
	case stepEndTime:
		return m.endInput.Value()
	case stepAffectedServices:
		return m.affectedServicesInput.Value()
	case stepTags:
		return m.tagsInput.Value()
	case stepResolution:
		return m.resolutionInput.Value()
	}
	return ""
}

func (m Model) renderConfirm(b *strings.Builder) {
	if m.opts.CloseMode && m.opts.ExistingFile != "" {
		b.WriteString(labelStyle.Render("Confirm close") + "\n\n")
		fields := []struct{ k, v string }{
			{"Incident #", m.opts.IncidentID},
			{"End Time", orNow(m.endInput.Value())},
			{"Resolution", orDash(m.resolutionInput.Value())},
			{"File", m.opts.ExistingFile},
		}
		for _, f := range fields {
			b.WriteString(summaryKeyStyle.Render(fmt.Sprintf("%-14s", f.k)) + " " + f.v + "\n")
		}
	} else if m.opts.ResolutionMode && m.opts.ExistingFile != "" {
		b.WriteString(labelStyle.Render("Confirm resolution") + "\n\n")
		fields := []struct{ k, v string }{
			{"Incident #", m.opts.IncidentID},
			{"Resolution", orDash(m.resolutionInput.Value())},
			{"File", m.opts.ExistingFile},
		}
		for _, f := range fields {
			b.WriteString(summaryKeyStyle.Render(fmt.Sprintf("%-14s", f.k)) + " " + f.v + "\n")
		}
	} else {
		b.WriteString(labelStyle.Render("Review your incident") + "\n\n")
		fields := []struct{ k, v string }{
			{"Incident #", m.idInput.Value()},
			{"Mode", modeLabel(m.opts.RawMode)},
			{"Summary", strings.ReplaceAll(m.summaryInput.Value(), "\n", " ↵ ")},
			{"Start Time", orDash(m.startInput.Value())},
			{"End Time", orStillOpen(m.endInput.Value())},
			{"Affected Services", orDash(m.affectedServicesInput.Value())},
			{"Tags", orDash(m.tagsInput.Value())},
			{"Resolution", orDash(strings.ReplaceAll(m.resolutionInput.Value(), "\n", " ↵ "))},
			{"Save to", m.cfg.OutputDir},
		}
		for _, f := range fields {
			b.WriteString(summaryKeyStyle.Render(fmt.Sprintf("%-14s", f.k)) + " " + f.v + "\n")
		}
	}

	action := confirmActionLabel(m.opts)
	b.WriteString("\n" + hintStyle.Render("Press enter to "+action+"  •  esc to quit") + "\n")
}

func (m Model) renderProcessing(b *strings.Builder) {
	if m.opts.CloseMode || m.opts.ResolutionMode {
		b.WriteString(m.spinner.View() + " Saving incident updates...\n")
		b.WriteString(hintStyle.Render("This should be quick.") + "\n")
		return
	}

	if m.opts.RawMode {
		b.WriteString(m.spinner.View() + " Saving incident report from raw notes...\n")
		b.WriteString(hintStyle.Render("This should be quick.") + "\n")
		return
	}

	b.WriteString(m.spinner.View() + " Saving and enriching with Claude AI...\n")
	b.WriteString(hintStyle.Render("This may take a few seconds.") + "\n")
}

func confirmActionLabel(opts Options) string {
	if opts.CloseMode || opts.ResolutionMode {
		return "save changes"
	}
	if opts.RawMode {
		return "save raw incident report"
	}
	return "save & enrich with Claude"
}

func modeLabel(rawMode bool) string {
	if rawMode {
		return "Raw (no LLM)"
	}
	return "Enriched (Claude)"
}

func parseSetupMode(raw, placeholder string) (string, error) {
	mode := strings.TrimSpace(raw)
	if mode == "" {
		mode = strings.TrimSpace(placeholder)
	}
	if !config.IsValidMode(mode) {
		return "", fmt.Errorf("invalid default mode %q (expected raw or enriched)", mode)
	}
	return config.NormalizeMode(mode), nil
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

func orNow(s string) string {
	if strings.TrimSpace(s) == "" {
		return "now (" + time.Now().Format("2006-01-02 15:04") + ")"
	}
	return s
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
