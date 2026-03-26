package client

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type schedulePreset struct {
	label      string
	expr       string
	fullBackup bool
}

var schedulePresets = []schedulePreset{
	{"Daily at 00:00 (incremental)", "0 0 * * *", false},
	{"Daily at 03:00 (incremental)", "0 3 * * *", false},
	{"Every 12 hours (incremental)", "0 */12 * * *", false},
	{"Every hour (incremental)", "0 * * * *", false},
	{"Weekly Sunday 00:00 (full)", "0 0 * * 0", true},
}

type scheduleWizardModel struct {
	cfgPath string
	step    int // 0 = preset list, 1 = custom cron input
	cursor  int
	custom  string
	errMsg  string
	okMsg   string
}

func (m scheduleWizardModel) Init() tea.Cmd { return nil }

func (m scheduleWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		if m.step == 1 {
			return m.updateCustomInput(msg)
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := len(schedulePresets) + 2 // custom, remove, back
			if m.cursor < max {
				m.cursor++
			}
		case "enter":
			m.errMsg = ""
			m.okMsg = ""
			n := len(schedulePresets)
			switch {
			case m.cursor < n:
				p := schedulePresets[m.cursor]
				if err := ApplyBackupCron(m.cfgPath, p.expr, p.fullBackup); err != nil {
					m.errMsg = err.Error()
				} else {
					m.okMsg = fmt.Sprintf("Installed: %s", p.label)
				}
			case m.cursor == n:
				m.step = 1
				m.custom = ""
			case m.cursor == n+1:
				if err := RemoveBackupCron(); err != nil {
					m.errMsg = err.Error()
				} else {
					m.okMsg = "Removed backuperr cron block (if it existed)."
				}
			default:
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m scheduleWizardModel) updateCustomInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = 0
			m.errMsg = ""
			return m, nil
		case "enter":
			m.errMsg = ""
			m.okMsg = ""
			expr := strings.TrimSpace(m.custom)
			if expr == "" {
				m.errMsg = "Enter 5 cron fields, e.g. 0 0 * * *"
				return m, nil
			}
			if err := ApplyBackupCron(m.cfgPath, expr, false); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.okMsg = "Installed custom schedule (incremental backup)."
			m.step = 0
			m.custom = ""
			return m, nil
		case "backspace":
			if len(m.custom) > 0 {
				m.custom = m.custom[:len(m.custom)-1]
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.custom += string(msg.Runes)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m scheduleWizardModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Schedule backups (cron)") + "\n\n")
	cur := CurrentBackupCronLine()
	if cur != "" {
		b.WriteString(lineStyle.Render("Current: "+cur) + "\n\n")
	} else {
		b.WriteString(hintStyle.Render("No backuperr cron entry yet.") + "\n\n")
	}

	if m.step == 1 {
		b.WriteString(lineStyle.Render("Custom schedule (5 fields, incremental backup)") + "\n")
		b.WriteString(selStyle.Render(m.custom+"_") + "\n\n")
		b.WriteString(hintStyle.Render("enter save · esc back · q quit"))
		return b.String()
	}

	if m.okMsg != "" {
		b.WriteString(selStyle.Render(m.okMsg) + "\n\n")
	}
	if m.errMsg != "" {
		b.WriteString(hintStyle.Render("Error: "+m.errMsg) + "\n\n")
	}

	for i, p := range schedulePresets {
		line := "  " + p.label
		if i == m.cursor {
			b.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			b.WriteString(lineStyle.Render(line) + "\n")
		}
	}
	customIdx := len(schedulePresets)
	removeIdx := customIdx + 1
	backIdx := removeIdx + 1

	line := "  Custom schedule..."
	if m.cursor == customIdx {
		b.WriteString(selStyle.Render("▶ "+line) + "\n")
	} else {
		b.WriteString(lineStyle.Render(line) + "\n")
	}
	line = "  Remove scheduled backups"
	if m.cursor == removeIdx {
		b.WriteString(selStyle.Render("▶ "+line) + "\n")
	} else {
		b.WriteString(lineStyle.Render(line) + "\n")
	}
	line = "  Back to main menu"
	if m.cursor == backIdx {
		b.WriteString(selStyle.Render("▶ "+line) + "\n")
	} else {
		b.WriteString(lineStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k ↓/j · enter · q quit"))
	return b.String()
}

// RunScheduleWizard runs the cron setup TUI (Linux: installs user crontab).
func RunScheduleWizard(cfgPath string) error {
	m := scheduleWizardModel{cfgPath: cfgPath}
	_, err := tea.NewProgram(m).Run()
	return err
}
