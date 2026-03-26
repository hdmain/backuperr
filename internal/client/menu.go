package client

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// RootAction is the choice from the main menu (no CLI subcommand).
type RootAction int

const (
	ActionQuit RootAction = iota
	ActionBackupIncremental
	ActionBackupFull
	ActionRestore
	ActionList
	ActionSchedule
)

type rootMenuModel struct {
	labels       []string
	actions      []RootAction
	storageLines string
	cursor       int
	chosen       RootAction
	done         bool
}

func (m rootMenuModel) Init() tea.Cmd { return nil }

func (m rootMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.chosen = ActionQuit
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.labels)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = m.actions[m.cursor]
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m rootMenuModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("backuperr client") + "\n")
	if m.storageLines != "" {
		b.WriteString(m.storageLines)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	for i, label := range m.labels {
		line := "  " + label
		if i == m.cursor {
			b.WriteString(selStyle.Render("▶ "+line) + "\n")
		} else {
			b.WriteString(lineStyle.Render(line) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k ↓/j · enter · q quit"))
	return b.String()
}

// HostStorageMenuLine fetches the host's data-dir disk usage for the TUI.
func HostStorageMenuLine(api *API) string {
	if api == nil {
		return hintStyle.Render("Host storage: no API client")
	}
	info, err := api.GetHostStorage()
	if err != nil {
		return hintStyle.Render(fmt.Sprintf("Host storage: unavailable (%v)", err))
	}
	if !info.Supported {
		return hintStyle.Render("Host storage: not reported by server (upgrade host or use Linux)")
	}
	return lineStyle.Render(fmt.Sprintf(
		"Host storage: %.2f GiB free / %.2f GiB total  (server %s)",
		float64(info.BytesFree)/(1<<30),
		float64(info.BytesTotal)/(1<<30),
		info.DataDir,
	))
}

// RunRootMenu shows the default TUI when ./client is run with no subcommand.
func RunRootMenu(api *API) (RootAction, error) {
	m := rootMenuModel{
		storageLines: HostStorageMenuLine(api),
		labels: []string{
			"Backup (incremental)",
			"Backup (full)",
			"Restore from host",
			"List backups",
			"Schedule backups (cron)",
			"Quit",
		},
		actions: []RootAction{
			ActionBackupIncremental,
			ActionBackupFull,
			ActionRestore,
			ActionList,
			ActionSchedule,
			ActionQuit,
		},
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return ActionQuit, err
	}
	fm, ok := final.(rootMenuModel)
	if !ok || !fm.done {
		return ActionQuit, nil
	}
	return fm.chosen, nil
}
