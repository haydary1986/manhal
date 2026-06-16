package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const deepSeekURL = "https://api.deepseek.com/chat/completions"

// DeepSeek is a Provider backed by the DeepSeek chat API (OpenAI-compatible).
type DeepSeek struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewDeepSeek creates a DeepSeek provider. If apiKey is empty the provider is
// still constructed, but Chat returns an error (AI features stay disabled).
func NewDeepSeek(apiKey string) *DeepSeek {
	return &DeepSeek{
		apiKey: apiKey,
		model:  "deepseek-chat",
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

// Name implements Provider.
func (d *DeepSeek) Name() string { return "deepseek" }

// Chat implements Provider.
func (d *DeepSeek) Chat(ctx context.Context, messages []Message) (string, error) {
	if d.apiKey == "" {
		return "", fmt.Errorf("deepseek: API key not configured")
	}

	type reqMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := struct {
		Model    string   `json:"model"`
		Messages []reqMsg `json:"messages"`
	}{Model: d.model}
	for _, m := range messages {
		reqBody.Messages = append(reqBody.Messages, reqMsg{Role: m.Role, Content: m.Content})
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepSeekURL, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek: status %d: %s", resp.StatusCode, string(data))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("deepseek: empty response")
	}
	return out.Choices[0].Message.Content, nil
}
