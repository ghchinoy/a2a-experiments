// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interactions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/interactions"

// Client is a Go wrapper for the Gemini Interactions REST API.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Interactions API client.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Create initiates a new interaction turn.
func (c *Client) Create(ctx context.Context, req *InteractionRequest) (*InteractionResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", c.BaseURL, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	return c.do(httpReq)
}

// Get retrieves the status and result of an existing interaction.
func (c *Client) Get(ctx context.Context, id string) (*InteractionResponse, error) {
	url := fmt.Sprintf("%s/%s?key=%s", c.BaseURL, id, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(httpReq)
}

// Delete removes a stored interaction.
func (c *Client) Delete(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/%s?key=%s", c.BaseURL, id, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	_, err = c.do(httpReq)
	return err
}

// WaitForCompletion polls an interaction until its status is a terminal state.
func (c *Client) WaitForCompletion(ctx context.Context, id string, interval time.Duration) (*InteractionResponse, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			resp, err := c.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			
			status := strings.ToLower(resp.Status)
			// Keep polling if it's in a known active state
			if status == "working" || status == "in_progress" || status == "pending" {
				continue
			}
			
			// Return if it's likely a terminal state (completed, failed, etc.)
			return resp, nil
		}
	}
}

func (c *Client) do(req *http.Request) (*InteractionResponse, error) {
	// Ensure we import "strings" in this file
	// ... (rest of the do method remains same)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if len(body) == 0 {
		return nil, nil
	}

	var interactionResp InteractionResponse
	if err := json.Unmarshal(body, &interactionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &interactionResp, nil
}
