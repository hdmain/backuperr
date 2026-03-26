//go:build !linux

package client

import "fmt"

// ErrCronScheduleUnsupported is returned when cron scheduling is not available on this OS.
var ErrCronScheduleUnsupported = fmt.Errorf("cron scheduling is only supported on Linux")

func ApplyBackupCron(cfgPath string, scheduleExpr string, fullBackup bool) error {
	_, _, _ = cfgPath, scheduleExpr, fullBackup
	return ErrCronScheduleUnsupported
}

func RemoveBackupCron() error {
	return ErrCronScheduleUnsupported
}

func CurrentBackupCronLine() string {
	return ""
}
