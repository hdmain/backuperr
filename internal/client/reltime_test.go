package client

import (
	"testing"
	"time"
)

func TestHumanTimeRelAt(t *testing.T) {
	now := time.Date(2026, 3, 25, 20, 0, 0, 0, time.UTC)
	tests := []struct {
		delta  time.Duration
		expect string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5 minutes ago"},
		{1 * time.Minute, "1 minute ago"},
		{3 * time.Hour, "3 hours ago"},
		{1 * time.Hour, "1 hour ago"},
		{48 * time.Hour, "2 days ago"},
		{24 * time.Hour, "1 day ago"},
	}
	for _, tt := range tests {
		then := now.Add(-tt.delta)
		if got := HumanTimeRelAt(then, now); got != tt.expect {
			t.Errorf("delta=%v got %q want %q", tt.delta, got, tt.expect)
		}
	}
}
