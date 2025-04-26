package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	target string
	client *http.Client
}

func New(target string, tlsConfig *tls.Config) *Client {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       30 * time.Second,
			DisableKeepAlives:     false,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		},
	}

	return &Client{target, client}
}

type MailTemplateOptions[T any] struct {
	TemplateGroup string   `json:"template_group"`
	TemplateNames []string `json:"template_name"`
	Targets       []string `json:"targets"`
	Data          T        `json:"data"`
}

func (c *Client) Send(ctx context.Context, options *MailTemplateOptions[any]) error {
	msg, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.target, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("unexpected status code: %d", res.StatusCode)
		}
		body := string(bodyBytes)
		return fmt.Errorf("unexpected status code: %d,%s", res.StatusCode, body)
	}

	return nil
}
