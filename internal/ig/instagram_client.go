package ig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	HTTP       *http.Client
	APIToken   string // page access token (per brand)
	APIVersion string // e.g. v21.0
}

func NewClient(apiToken string) *Client {
	return &Client{
		HTTP:       &http.Client{Timeout: 10 * time.Second},
		APIToken:   apiToken,
		APIVersion: "v21.0",
	}
}

// Post Comment Reply (public)
func (c *Client) ReplyComment(ctx context.Context, commentID, message string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/replies", c.APIVersion, commentID)
	body := map[string]string{
		"message":      message,
		"access_token": c.APIToken,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ReplyComment status %d", resp.StatusCode)
	}
	return nil
}

// Send DM (Instagram messaging API via FB Graph)
// NOTE: DM API punya batasan; ini contoh pseudo endpoint, sesuaikan dgn endpoint real & permission.
// Untuk MVP, kirim link sebagai teks.
func (c *Client) SendDM(ctx context.Context, recipientIGUserID, message string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/me/messages", c.APIVersion)
	body := map[string]interface{}{
		"recipient":    map[string]string{"id": recipientIGUserID},
		"message":      map[string]string{"text": message},
		"access_token": c.APIToken,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("SendDM status %d", resp.StatusCode)
	}
	return nil
}
