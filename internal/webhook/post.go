package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"backuperr/pkg/types"
)

const postTimeout = 15 * time.Second

func postRaw(url string, body []byte) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: postTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errHTTPStatus{code: resp.StatusCode, status: resp.Status}
	}
	return nil
}

// PostInfo sends payload to url. Discord incoming webhooks get a rich embed; other URLs receive raw JSON.
func PostInfo(url string, p types.WebhookInfoPayload) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	if IsDiscordWebhookURL(url) {
		body, err := marshalDiscordWebhook(p)
		if err != nil {
			return err
		}
		return postRaw(url, body)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return postRaw(url, body)
}

// PostJSON sends v as application/json. Empty url is a no-op.
func PostJSON(url string, v any) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return postRaw(url, body)
}

type errHTTPStatus struct {
	code   int
	status string
}

func (e errHTTPStatus) Error() string {
	return e.status
}
