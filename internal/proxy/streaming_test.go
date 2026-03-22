package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProxyIsStreamingRequest(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		url      string
		body     string
		expected bool
	}{
		{
			name:     "stream query param true",
			method:   http.MethodGet,
			url:      "/v1/models?stream=true",
			body:     "",
			expected: true,
		},
		{
			name:     "stream query param false",
			method:   http.MethodGet,
			url:      "/v1/models?stream=false",
			body:     "",
			expected: false,
		},
		{
			name:     "stream in body true",
			method:   http.MethodPost,
			url:      "/v1/chat/completions",
			body:     `{"model":"test","stream":true}`,
			expected: true,
		},
		{
			name:     "stream in body false",
			method:   http.MethodPost,
			url:      "/v1/chat/completions",
			body:     `{"model":"test","stream":false}`,
			expected: false,
		},
		{
			name:     "no stream",
			method:   http.MethodGet,
			url:      "/v1/models",
			body:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.url, body)
			req.Header.Set("Content-Type", "application/json")

			result, _, err := isStreamingRequest(req)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected streaming=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestProxyServeStreaming(t *testing.T) {
	// Create mock streaming server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send multiple chunks
		for i := 0; i < 5; i++ {
			data := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "test-model",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "chunk" + string(rune('0'+i)),
						},
						"finish_reason": nil,
					},
				},
			}

			jsonData, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	// Create proxy with 60s timeout
	p := New(mockServer.URL, 60)

	// Create streaming request
	reqBody := `{"model":"test-model","messages":[{"role":"user","content":"Hello"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve streaming request
	p.ServeStreaming(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	bodyStr := string(body)

	// Verify SSE format
	if !strings.Contains(bodyStr, "data: ") {
		t.Error("Expected SSE data format")
	}

	// Verify multiple chunks
	if strings.Count(bodyStr, "data: ") < 3 {
		t.Error("Expected multiple streaming chunks")
	}
}

func TestProxyStreamingChunks(t *testing.T) {
	// Create mock streaming server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send single character chunks
		response := "Hello"
		for _, ch := range response {
			data := map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"delta": map[string]interface{}{
							"content": string(ch),
						},
					},
				},
			}
			jsonData, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	// Create proxy
	p := New(mockServer.URL, 60)

	// Create streaming request
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve streaming request
	p.ServeStreaming(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	bodyStr := string(body)

	// Verify all chunks received (check for individual characters)
	if !strings.Contains(bodyStr, `"content":"H"`) ||
		!strings.Contains(bodyStr, `"content":"e"`) ||
		!strings.Contains(bodyStr, `"content":"l"`) ||
		!strings.Contains(bodyStr, `"content":"o"`) {
		t.Error("Expected individual characters in response")
	}
}
