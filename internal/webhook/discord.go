package webhook

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"backuperr/pkg/types"
)

const (
	discordColorOK    = 0x57F287 // green
	discordColorError = 0xED4245 // red
	discordColorInfo  = 0x5865F2 // blurple
)

// IsDiscordWebhookURL reports whether url is a Discord incoming webhook endpoint.
func IsDiscordWebhookURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	host := strings.ToLower(strings.Split(u.Host, ":")[0])
	switch host {
	case "discord.com", "canary.discord.com", "ptb.discord.com", "discordapp.com":
	default:
		return false
	}
	return strings.HasPrefix(u.Path, "/api/webhooks/")
}

type discordWebhookBody struct {
	Username string         `json:"username,omitempty"`
	Embeds   []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Fields      []discordField `json:"fields,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

func formatBytesIEC(n uint64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(n)/(1<<30))
	}
	if n >= 1<<20 {
		return fmt.Sprintf("%.2f MiB", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.2f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func discordTruncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func marshalDiscordWebhook(p types.WebhookInfoPayload) ([]byte, error) {
	color := discordColorInfo
	switch strings.ToLower(p.Status) {
	case "ok", "success":
		color = discordColorOK
	case "error", "err", "failed":
		color = discordColorError
	}

	title := "backuperr"
	switch p.Source {
	case "client":
		title = "backuperr · client"
	case "host":
		title = "backuperr · host"
	default:
		if p.Source != "" {
			title = "backuperr · " + p.Source
		}
	}

	var fields []discordField
	add := func(name, value string, inline bool) {
		fields = append(fields, discordField{
			Name:   discordTruncate(name, 256),
			Value:  discordTruncate(value, 1024),
			Inline: inline,
		})
	}

	add("Status", p.Status, true)
	if p.Event != "" {
		add("Event", p.Event, true)
	}
	add("Time (UTC)", p.Time, false)
	if p.Message != "" {
		add("Message", p.Message, false)
	}
	if p.VolumePath != "" || p.VolumeOK {
		vol := "n/a"
		if p.VolumeOK {
			vol = fmt.Sprintf("%s free / %s total", formatBytesIEC(p.BytesFree), formatBytesIEC(p.BytesTotal))
		} else if p.VolumePath != "" {
			vol = "volume stats unavailable"
		}
		add("Local volume", discordTruncate(fmt.Sprintf("%s\n%s", p.VolumePath, vol), 1024), false)
	}
	if p.HostStorageOK {
		add("Host storage", fmt.Sprintf("%s\n%s free / %s total",
			p.HostDataDir,
			formatBytesIEC(p.HostBytesFree),
			formatBytesIEC(p.HostBytesTotal)), false)
	}
	if p.ClientIP != "" {
		add("Client IP", p.ClientIP, true)
	}

	ts := p.Time
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		ts = time.Now().UTC().Format(time.RFC3339)
	}

	body := discordWebhookBody{
		Username: "backuperr",
		Embeds: []discordEmbed{{
			Title:     title,
			Color:     color,
			Fields:    fields,
			Timestamp: ts,
		}},
	}
	return json.Marshal(body)
}
