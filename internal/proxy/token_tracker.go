package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

// TokenStats tracks token usage and speed
type TokenStats struct {
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	TotalDuration      time.Duration
	GenerationDuration time.Duration
	TokensPerSecond    float64
	FirstTokenTime     time.Time
	LastTokenTime      time.Time
	Chunks             int
	ChunksPerSecond    float64
}

// TokenTrackingReader wraps a response body to track tokens in real-time
type TokenTrackingReader struct {
	Body             io.ReadCloser
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	mu               sync.Mutex
	StartTime        time.Time
	FirstTokenTime   time.Time
	LastTokenTime    time.Time
	done             chan struct{}
	Chunks           int
}

// Read reads from the underlying body and tracks tokens in real-time
func (r *TokenTrackingReader) Read(p []byte) (n int, err error) {
	n, err = r.Body.Read(p)
	if n > 0 {
		r.mu.Lock()
		now := time.Now()
		if r.StartTime.IsZero() {
			r.StartTime = now
		}
		if r.FirstTokenTime.IsZero() {
			r.FirstTokenTime = now
		}
		r.LastTokenTime = now
		r.Chunks++
		if !r.LastTokenTime.IsZero() {
			chunkDuration := now.Sub(r.LastTokenTime).Seconds()
			r.trackChunksInBuffer(p[:n], now, chunkDuration, n)
		}
		r.trackTokensInBuffer(p[:n])
		r.mu.Unlock()
	}
	return n, err
}

// Close closes the underlying body
func (r *TokenTrackingReader) Close() error {
	close(r.done)
	return r.Body.Close()
}

// trackTokensInBuffer parses tokens as they arrive in the buffer
func (r *TokenTrackingReader) trackTokensInBuffer(data []byte) {
	if r.StartTime.IsZero() {
		r.StartTime = time.Now()
	}

	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		lineStr := string(line)
		if strings.HasPrefix(lineStr, "data: ") {
			dataStr := strings.TrimPrefix(lineStr, "data: ")
			if dataStr == "[DONE]" {
				continue
			}
			r.parseUsageFromJSON([]byte(dataStr))
		}
	}
}

// trackChunksInBuffer logs chunk timing info (DEBUG level only)
func (r *TokenTrackingReader) trackChunksInBuffer(data []byte, now time.Time, chunkDuration float64, n int) {
	debug("[PROXY] Chunk %d: %.3fs, %d bytes", r.Chunks, chunkDuration, n)
}

// parseUsageFromJSON extracts token counts from usage or timings object
func (r *TokenTrackingReader) parseUsageFromJSON(data []byte) error {
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		return err
	}

	// Try usage object first (standard OpenAI format)
	if usage, ok := rawJSON["usage"].(map[string]interface{}); ok {
		if tokens, ok := usage["prompt_tokens"].(float64); ok {
			r.PromptTokens = int(tokens)
		}
		if tokens, ok := usage["completion_tokens"].(float64); ok {
			r.CompletionTokens = int(tokens)
		}
		if tokens, ok := usage["total_tokens"].(float64); ok {
			r.TotalTokens = int(tokens)
		}
		return nil
	}

	// Fall back to timings object (vLLM-style)
	if timings, ok := rawJSON["timings"].(map[string]interface{}); ok {
		if promptN, ok := timings["prompt_n"].(float64); ok {
			r.PromptTokens = int(promptN)
		}
		if predictedN, ok := timings["predicted_n"].(float64); ok {
			r.CompletionTokens = int(predictedN)
		}
		return nil
	}

	return nil
}

// GetStats returns the current token statistics
func (r *TokenTrackingReader) GetStats() *TokenStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalDuration := time.Since(r.StartTime)
	var generationDuration time.Duration
	if !r.FirstTokenTime.IsZero() && !r.LastTokenTime.IsZero() {
		generationDuration = r.LastTokenTime.Sub(r.FirstTokenTime)
	}

	var tokensPerSecond float64
	if generationDuration > 0 && r.CompletionTokens > 0 {
		tokensPerSecond = float64(r.CompletionTokens) / generationDuration.Seconds()
	}

	var chunksPerSecond float64
	if generationDuration > 0 {
		chunksPerSecond = float64(r.Chunks) / generationDuration.Seconds()
	}

	return &TokenStats{
		PromptTokens:       r.PromptTokens,
		CompletionTokens:   r.CompletionTokens,
		TotalTokens:        r.TotalTokens,
		TotalDuration:      totalDuration,
		GenerationDuration: generationDuration,
		TokensPerSecond:    tokensPerSecond,
		FirstTokenTime:     r.FirstTokenTime,
		LastTokenTime:      r.LastTokenTime,
		Chunks:             r.Chunks,
		ChunksPerSecond:    chunksPerSecond,
	}
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
