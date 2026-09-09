package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"res-downloader/internal/control"
	"time"
)

type response struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type client struct {
	sessionPath string
	http        *http.Client
}

func newClient(path string, timeout time.Duration) *client {
	return &client{sessionPath: path, http: &http.Client{
		Timeout: timeout,
		// Never send a local credential through environment-configured proxies
		// or follow redirects to another endpoint.
		Transport: &http.Transport{
			Proxy: nil, DisableKeepAlives: true,
			DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *client) call(ctx context.Context, op operation, raw json.RawMessage) (*response, error) {
	arguments, err := op.arguments(raw)
	if err != nil {
		return nil, err
	}
	// Re-read on every call so a long-lived MCP process follows app restarts.
	session, err := control.ReadSession(c.sessionPath)
	if err != nil {
		return nil, fail("unavailable", err.Error())
	}
	payload, err := json.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, session.URL+op.path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+session.Token)
	request.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(request)
	if err != nil {
		// Mutating requests are not retried: a lost response may still mean
		// that the task was created or changed successfully.
		return nil, fail("unavailable", "desktop request failed or timed out; check that the app is running and query task state before retrying")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fail("unauthorized", "local session expired; retry with the current desktop session")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fail("api_error", fmt.Sprintf("desktop API returned HTTP %d", resp.StatusCode))
	}
	const maxResponse = 64 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(data) > maxResponse {
		return nil, fail("api_error", "desktop response could not be read or exceeds 64 MiB; use a smaller resource page")
	}
	var result response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fail("api_error", "desktop API returned invalid JSON")
	}
	if result.Code != 1 {
		return nil, fail("operation_failed", result.Message)
	}
	if op.name == "get_download" {
		var tasks []json.RawMessage
		if err := json.Unmarshal(result.Data, &tasks); err != nil {
			return nil, fail("api_error", "desktop API returned invalid tasks")
		}
		for _, task := range tasks {
			var record struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(task, &record) == nil && record.ID == arguments["id"] {
				result.Data = task
				return &result, nil
			}
		}
		return nil, fail("not_found", "download task not found")
	}
	return &result, nil
}
