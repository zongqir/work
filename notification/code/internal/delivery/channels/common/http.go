package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func PostJSON(ctx context.Context, httpClient *http.Client, requestURL string, payload any, headers map[string]string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}
	return Post(ctx, httpClient, requestURL, "application/json", data, headers)
}

func Post(ctx context.Context, httpClient *http.Client, targetURL, contentType string, body []byte, headers map[string]string) error {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return fmt.Errorf("invalid target url %q: %w", targetURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request failed: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if len(respBody) == 0 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return fmt.Errorf("unexpected status code: %d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
}
