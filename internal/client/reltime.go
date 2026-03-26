package client

import (
	"fmt"
	"time"
)

// HumanTimeRel returns a short phrase like "just now", "5 minutes ago", "2 days ago".
func HumanTimeRel(t time.Time) string {
	return HumanTimeRelAt(t, time.Now())
}

// HumanTimeRelAt is like HumanTimeRel but uses now as the reference instant.
func HumanTimeRelAt(t, now time.Time) string {
	t = t.UTC()
	now = now.UTC()
	if !t.Before(now) {
		return t.Format("Jan 2, 2006 15:04 UTC")
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		n := int(d / time.Minute)
		return agoPlural(n, "minute", "minutes")
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		return agoPlural(n, "hour", "hours")
	default:
		days := int(d / (24 * time.Hour))
		if days < 30 {
			return agoPlural(days, "day", "days")
		}
		if days < 365 {
			months := days / 30
			if months < 1 {
				months = 1
			}
			return agoPlural(months, "month", "months")
		}
		years := days / 365
		if years < 1 {
			years = 1
		}
		return agoPlural(years, "year", "years")
	}
}

func agoPlural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", one)
	}
	return fmt.Sprintf("%d %s ago", n, many)
}

// ShortBackupID returns an 8-character prefix for display when id is longer.
func ShortBackupID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
