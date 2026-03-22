package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// TokenStats tracks token usage and speed
type TokenStats struct {
	PromptTokens         int
	CompletionTokens     int
	TotalTokens          int
	Duration             float64
	TokensPerSecond      float64
	StreamingStartTime   time.Time
	LastChunkTime        time.Time
	Chunks               int
	ChunkDurationSeconds float64
	ChunksPerSecond      float64
}

// TokenTrackingReader wraps a response body to track tokens in real-time
type TokenTrackingReader struct {
	Body             io.ReadCloser
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	mu               sync.Mutex
	StartTime        time.Time
	done             chan struct{}
	Chunks           int
	LastChunkTime    time.Time
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
		r.Chunks++
		if !r.LastChunkTime.IsZero() {
			chunkDuration := now.Sub(r.LastChunkTime).Seconds()
			fmt.Printf("[PROXY] \033[36mChunk %d:\033[0m %.3fs since last chunk\n", r.Chunks, chunkDuration)
		}
		r.LastChunkTime = now
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
			if err := r.parseUsageFromJSON([]byte(dataStr)); err != nil {
				// Ignore parse errors, just skip
			}
		}
	}
}

// parseUsageFromJSON extracts token counts from usage object
func (r *TokenTrackingReader) parseUsageFromJSON(data []byte) error {
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		return err
	}

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
	}
	return nil
}

// GetStats returns the current token statistics
func (r *TokenTrackingReader) GetStats() *TokenStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	duration := time.Since(r.StartTime).Seconds()
	tokensPerSecond := float64(r.TotalTokens) / duration
	var chunkDuration float64
	var chunksPerSecond float64
	if r.Chunks > 1 {
		chunkDuration = duration / float64(r.Chunks-1)
		chunksPerSecond = float64(r.Chunks) / duration
	}

	return &TokenStats{
		PromptTokens:         r.PromptTokens,
		CompletionTokens:     r.CompletionTokens,
		TotalTokens:          r.TotalTokens,
		Duration:             duration,
		TokensPerSecond:      tokensPerSecond,
		StreamingStartTime:   r.StartTime,
		LastChunkTime:        r.LastChunkTime,
		Chunks:               r.Chunks,
		ChunkDurationSeconds: chunkDuration,
		ChunksPerSecond:      chunksPerSecond,
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
