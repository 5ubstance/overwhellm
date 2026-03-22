package token

import (
	"strings"
)

type Counter struct {
	model string
}

func NewCounter(model string) (*Counter, error) {
	return &Counter{
		model: model,
	}, nil
}

// Simple word-based token estimation
// This is less accurate than tiktoken but works without CGO
func (c *Counter) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Basic estimation: count words and punctuation
	// Average English word is ~4-5 characters, ~1.3 tokens per word
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Split by whitespace
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	// Count tokens: ~1.3 tokens per word on average
	// This is a rough estimate; tiktoken would be more accurate
	total := 0
	for _, word := range words {
		// Count the word and any attached punctuation
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if word != "" {
			// Estimate: each word is about 1.3 tokens
			// For simplicity, we'll use 1 token per word as a baseline
			// and add bonus for longer words
			total += 1
			if len(word) > 6 {
				total += len(word) / 6 // Bonus token for long words
			}
		}
	}

	return total
}

func (c *Counter) CountMessages(messages []map[string]interface{}) int {
	total := 0
	for _, msg := range messages {
		if content, ok := msg["content"].(string); ok {
			total += c.CountTokens(content)
		}
		// Count role and name if present
		if role, ok := msg["role"].(string); ok {
			total += c.CountTokens(role)
		}
		if name, ok := msg["name"].(string); ok {
			total += c.CountTokens(name)
		}
	}
	return total
}

func (c *Counter) CountResponse(choice map[string]interface{}) int {
	if content, ok := choice["message"].(map[string]interface{}); ok {
		if text, ok := content["content"].(string); ok {
			return c.CountTokens(text)
		}
	}
	return 0
}
