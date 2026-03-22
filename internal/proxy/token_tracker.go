package proxy

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// TokenStats tracks token usage and speed
type TokenStats struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Duration         float64
	TokensPerSecond  float64
}

// TokenTrackingReader wraps a response body to track tokens
type TokenTrackingReader struct {
	Body             io.ReadCloser
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Read reads from the underlying body and tracks tokens
func (r *TokenTrackingReader) Read(p []byte) (n int, err error) {
	return r.Body.Read(p)
}

// Close closes the underlying body
func (r *TokenTrackingReader) Close() error {
	return r.Body.Close()
}

// ParseTokenUsage extracts token usage from the response body
func ParseTokenUsage(body io.Reader) (*TokenStats, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return ParseTokenUsageFromBytes(bodyBytes)
}

// ParseTokenUsageFromBytes extracts token usage from response bytes
func ParseTokenUsageFromBytes(bodyBytes []byte) (*TokenStats, error) {
	var rawJSON map[string]interface{}

	if err := json.Unmarshal(bodyBytes, &rawJSON); err != nil {
		return nil, err
	}

	stats := &TokenStats{}

	if usage, ok := rawJSON["usage"].(map[string]interface{}); ok {
		if tokens, ok := usage["prompt_tokens"].(float64); ok {
			stats.PromptTokens = int(tokens)
		}
		if tokens, ok := usage["completion_tokens"].(float64); ok {
			stats.CompletionTokens = int(tokens)
		}
		if tokens, ok := usage["total_tokens"].(float64); ok {
			stats.TotalTokens = int(tokens)
		}
	}

	return stats, nil
}

// ParseStreamingTokenUsage extracts token usage from streaming SSE responses
func ParseStreamingTokenUsage(body io.Reader) (*TokenStats, error) {
	scanner := bufio.NewScanner(body)
	var lastData string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			lastData = strings.TrimPrefix(line, "data: ")
			if lastData == "[DONE]" {
				break
			}
		}
	}

	if lastData == "" || lastData == "[DONE]" {
		return nil, nil
	}

	return ParseTokenUsageFromBytes([]byte(lastData))
}
