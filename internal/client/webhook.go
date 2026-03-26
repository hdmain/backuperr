package client

import (
	"log"
	"strings"
	"time"

	"backuperr/internal/webhook"
	"backuperr/pkg/types"
)

func buildClientWebhookPayload(cfg *Config, api *API, status, message, event string) types.WebhookInfoPayload {
	p := types.WebhookInfoPayload{
		Source:     "client",
		Status:     status,
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Event:      event,
		VolumePath: cfg.BackupRoot,
	}
	if cfg.BackupRoot != "" {
		total, free, ok := nodeVolumeSpace(cfg.BackupRoot)
		p.BytesTotal, p.BytesFree, p.VolumeOK = total, free, ok
	}
	if api != nil {
		hs, err := api.GetHostStorage()
		if err == nil && hs.Supported {
			p.HostBytesFree = hs.BytesFree
			p.HostBytesTotal = hs.BytesTotal
			p.HostDataDir = hs.DataDir
			p.HostStorageOK = true
		}
	}
	return p
}

// SendClientWebhook POSTs an info payload to cfg.webhook_url (if set). Runs asynchronously.
func SendClientWebhook(cfg *Config, api *API, status, message, event string) {
	url := strings.TrimSpace(cfg.WebhookURL)
	if url == "" {
		return
	}
	p := buildClientWebhookPayload(cfg, api, status, message, event)
	go func() {
		if err := webhook.PostInfo(url, p); err != nil {
			log.Printf("webhook: %v", err)
		}
	}()
}
