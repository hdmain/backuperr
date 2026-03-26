//go:build linux

package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	cronBlockBegin = "# BEGIN_BACKUPERR_CRON"
	cronBlockEnd   = "# END_BACKUPERR_CRON"
)

// shellQuoteSingle wraps s in single quotes for sh -c / crontab command lines.
func shellQuoteSingle(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

func readUserCrontabLines() ([]string, error) {
	out, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("crontab -l: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

func writeUserCrontabLines(lines []string) error {
	body := strings.Join(lines, "\n")
	if body != "" {
		body += "\n"
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab -: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func stripBackuperrCronBlock(lines []string) []string {
	var out []string
	skip := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == cronBlockBegin {
			skip = true
			continue
		}
		if t == cronBlockEnd {
			skip = false
			continue
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

func clientExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Clean(p))
}

func validateCronSchedule(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("cron schedule must have 5 fields (got %d): %q", len(fields), expr)
	}
	return nil
}

func buildCronCommand(clientAbs, cfgAbs string, fullBackup bool) string {
	cmd := fmt.Sprintf("%s -config %s backup", shellQuoteSingle(clientAbs), shellQuoteSingle(cfgAbs))
	if fullBackup {
		cmd += " --full"
	}
	return cmd
}

// ApplyBackupCron installs or replaces the backuperr block in the user's crontab.
func ApplyBackupCron(cfgPath string, scheduleExpr string, fullBackup bool) error {
	if err := validateCronSchedule(scheduleExpr); err != nil {
		return err
	}
	clientAbs, err := clientExecutablePath()
	if err != nil {
		return fmt.Errorf("client executable: %w", err)
	}
	cfgAbs, err := filepath.Abs(cfgPath)
	if err != nil {
		return fmt.Errorf("config path: %w", err)
	}

	lines, err := readUserCrontabLines()
	if err != nil {
		return err
	}
	lines = stripBackuperrCronBlock(lines)

	job := fmt.Sprintf("%s %s", scheduleExpr, buildCronCommand(clientAbs, cfgAbs, fullBackup))
	block := []string{
		cronBlockBegin,
		"# backuperr managed (edit via client Schedule menu)",
		job,
		cronBlockEnd,
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, block...)

	return writeUserCrontabLines(lines)
}

// RemoveBackupCron removes the backuperr block from the user's crontab.
func RemoveBackupCron() error {
	lines, err := readUserCrontabLines()
	if err != nil {
		return err
	}
	lines = stripBackuperrCronBlock(lines)
	return writeUserCrontabLines(lines)
}

// CurrentBackupCronLine returns the first non-comment job line inside the block, if any.
func CurrentBackupCronLine() string {
	lines, err := readUserCrontabLines()
	if err != nil || len(lines) == 0 {
		return ""
	}
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == cronBlockBegin {
			in = true
			continue
		}
		if t == cronBlockEnd {
			break
		}
		if in && t != "" && !strings.HasPrefix(t, "#") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
