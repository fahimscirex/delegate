package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is kept as a raw map so provider-specific fields (e.g. DeepSeek's
// reasoning_content) round-trip untouched — some providers require them back.
type Message = map[string]any

type ToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

type chatResponse struct {
	Choices []struct {
		Message      json.RawMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error any   `json:"error"`
}

type Client struct {
	URL     string
	Key     string
	Headers map[string]string
	HTTP    *http.Client
}

func newClient(p Provider, key, session string) *Client {
	h := map[string]string{}
	for k, v := range p.Headers {
		h[k] = strings.ReplaceAll(v, "{session}", session)
	}
	return &Client{
		URL:     strings.TrimRight(p.BaseURL, "/") + "/chat/completions",
		Key:     key,
		Headers: h,
		HTTP:    &http.Client{Timeout: 10 * time.Minute},
	}
}

// Chat sends one completion request; retries 429/5xx/network errors with backoff.
func (c *Client) Chat(body map[string]any) (Message, []ToolCall, Usage, error) {
	payload, _ := json.Marshal(body)
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
		req, _ := http.NewRequest("POST", c.URL, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Key)
		for k, v := range c.Headers {
			req.Header.Set(k, v)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %.300s", resp.StatusCode, raw)
			continue
		}
		var r chatResponse
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, nil, Usage{}, fmt.Errorf("HTTP %d, bad JSON: %.300s", resp.StatusCode, raw)
		}
		if resp.StatusCode != 200 || r.Error != nil || len(r.Choices) == 0 {
			return nil, nil, Usage{}, fmt.Errorf("HTTP %d: %.500s", resp.StatusCode, raw)
		}
		var msg Message
		var tc struct {
			ToolCalls []ToolCall `json:"tool_calls"`
		}
		json.Unmarshal(r.Choices[0].Message, &msg)
		json.Unmarshal(r.Choices[0].Message, &tc)
		return msg, tc.ToolCalls, r.Usage, nil
	}
	return nil, nil, Usage{}, lastErr
}
