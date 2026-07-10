package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// client is a thin HTTP client for the tasksd REST API. The CLI encodes every
// command into an HTTP request using the shared api registry metadata, so the
// CLI stays in lockstep with the server automatically.
type client struct {
	base  string
	token string
	hc    *http.Client
}

func newClient() *client {
	base := os.Getenv("TASKS_URL")
	if base == "" {
		base = "http://127.0.0.1:7842"
	}
	return &client{
		base:  strings.TrimRight(base, "/"),
		token: os.Getenv("TASKS_TOKEN"),
		hc:    &http.Client{Timeout: 30 * time.Second},
	}
}

// request sends method+path(+query)(+body) and returns the raw response body.
func (c *client) request(method, path string, query url.Values, body map[string]any) ([]byte, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if len(body) > 0 {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach tasks server at %s: %w", c.base, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	jsonCT := strings.Contains(resp.Header.Get("Content-Type"), "json")

	if resp.StatusCode >= 400 {
		// Prefer a structured {"error": ...} message from a real tasks server.
		var e struct {
			Error string `json:"error"`
		}
		if jsonCT && json.Unmarshal(data, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("%s", e.Error)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("unauthorized (HTTP 401) from %s — set TASKS_TOKEN to match the server", c.base)
		}
		return nil, fmt.Errorf("server at %s returned HTTP %d for %s %s — is tasksd running here? "+
			"another service may be on this port (set TASKS_URL to point at your tasks server)",
			c.base, resp.StatusCode, method, path)
	}
	// A 2xx that isn't JSON means we're talking to the wrong server.
	if !jsonCT {
		return nil, fmt.Errorf("unexpected non-JSON response from %s (HTTP %d) — is a tasks server running there? "+
			"another service may be on this port (set TASKS_URL)", c.base, resp.StatusCode)
	}
	return data, nil
}
