package client

import (
	"fmt"
	"strings"

	"backuperr/pkg/types"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	lineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
)

// PickBackupModel is a minimal TUI to pick a backup from a list.
type PickBackupModel struct {
	Backups []types.BackupMeta
	Cursor  int
	Chosen  string
	Quitting bool
}

func (m PickBackupModel) Init() tea.Cmd { return nil }

func (m PickBackupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Backups)-1 {
				m.Cursor++
			}
		case "enter":
			if len(m.Backups) > 0 {
				m.Chosen = m.Backups[m.Cursor].ID
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m PickBackupModel) View() string {
	if len(m.Backups) == 0 {
		return hintStyle.Render("No backups on the host for this client IP.") + "\n\nPress q to quit."
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select backup to restore") + "\n\n")
	for i, bk := range m.Backups {
		at := bk.CreatedAt.UTC()
		rel := HumanTimeRel(at)
		abs := at.Format("Jan 2, 15:04 UTC")
		line := fmt.Sprintf("  %s  %s  %s  (%s)  %d files  %s",
			ShortBackupID(bk.ID), bk.Type, rel, abs, bk.FileCount, formatSize(bk.Bytes))
		if i == m.Cursor {
			b.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			b.WriteString(lineStyle.Render("  "+line) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k ↓/j move · enter restore · q quit"))
	return b.String()
}

func formatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
}

// RunPickBackup starts the TUI and returns chosen backup id, or empty if cancelled.
func RunPickBackup(backups []types.BackupMeta) (string, error) {
	m := PickBackupModel{Backups: backups, Cursor: 0}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	fm, ok := final.(PickBackupModel)
	if !ok {
		return "", nil
	}
	if fm.Quitting && fm.Chosen == "" {
		return "", nil
	}
	return fm.Chosen, nil
}
