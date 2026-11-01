package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// ListModels sends only the credential header, never prompts or Telegram data.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model_list_http_%d", resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return nil, fmt.Errorf("invalid_model_list")
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid_model_list")
	}
	seen := map[string]bool{}
	result := []string{}
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" && len(id) <= 200 && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	sort.Strings(result)
	return result, nil
}
