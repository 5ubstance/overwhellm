package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"overwhellm/internal/db"
	"overwhellm/pkg/token"
)

type Proxy struct {
	client       *http.Client
	targetURL    string
	tokenCounter *token.Counter
	db           *db.DB
}

func New(targetURL string, db *db.DB) (*Proxy, error) {
	// Try to detect model from target URL or use default
	model := "cl100k_base"
	if strings.Contains(targetURL, "gpt-") || strings.Contains(targetURL, "chat") {
		model = "cl100k_base"
	}

	counter, err := token.NewCounter(model)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
		targetURL:    targetURL,
		tokenCounter: counter,
		db:           db,
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	// Read request body (only for methods that have bodies)
	var bodyBytes []byte
	var requestSize int
	var reqMap map[string]interface{}

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		requestSize = len(bodyBytes)

		// Parse request to extract model and messages
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
				http.Error(w, "Invalid JSON request", http.StatusBadRequest)
				return
			}
		}
	}

	// Extract model name
	model := "unknown"
	if m, ok := reqMap["model"].(string); ok {
		model = m
	}

	// Count input tokens
	tokensIn := 0
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		msgs := make([]map[string]interface{}, len(messages))
		for i, m := range messages {
			if msgMap, ok := m.(map[string]interface{}); ok {
				msgs[i] = msgMap
			}
		}
		tokensIn = p.tokenCounter.CountMessages(msgs)
	}

	// Prepare new request - strip /proxy prefix if present
	targetPath := r.URL.Path
	if strings.HasPrefix(targetPath, "/proxy") {
		targetPath = strings.TrimPrefix(targetPath, "/proxy")
	}

	newReq, err := http.NewRequestWithContext(r.Context(), r.Method, p.targetURL+targetPath, bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key := range r.Header {
		if key != "Host" && key != "Content-Length" {
			newReq.Header.Set(key, r.Header.Get(key))
		}
	}

	// Forward request
	resp, err := p.client.Do(newReq)
	if err != nil {
		recordRequest(p.db, &db.Request{
			ClientIP:         clientIP,
			UserAgent:        userAgent,
			Endpoint:         r.URL.Path,
			Model:            model,
			Method:           r.Method,
			StatusCode:       0,
			DurationMs:       int(time.Since(startTime).Milliseconds()),
			TokensInput:      tokensIn,
			RequestSizeBytes: requestSize,
		})
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	responseSize := len(respBodyBytes)
	statusCode := resp.StatusCode
	_ = int(time.Since(startTime).Milliseconds()) // durationMs

	// Parse response to extract output tokens
	var respMap map[string]interface{}
	var tokensOut int
	var ttftMs int

	if err := json.Unmarshal(respBodyBytes, &respMap); err == nil {
		// Non-streaming response
		if usage, ok := respMap["usage"].(map[string]interface{}); ok {
			if tokensInVal, ok := usage["prompt_tokens"].(float64); ok {
				tokensIn = int(tokensInVal)
			}
			if tokensOutVal, ok := usage["completion_tokens"].(float64); ok {
				tokensOut = int(tokensOutVal)
			}
		} else if choices, ok := respMap["choices"].([]interface{}); ok && len(choices) > 0 {
			if choiceMap, ok := choices[0].(map[string]interface{}); ok {
				tokensOut = p.tokenCounter.CountResponse(choiceMap)
			}
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", responseSize))
	w.Header().Set("Connection", "close")
	w.WriteHeader(statusCode)
	w.Write(respBodyBytes)

	// Record request (commented out for now)
	// fmt.Printf("[PROXY] Recording request...\n")
	// go func() {
	// 	recordRequest(p.db, &db.Request{
	// 		ClientIP:          clientIP,
	// 		UserAgent:         userAgent,
	// 		Endpoint:          r.URL.Path,
	// 		Model:             model,
	// 		Method:            r.Method,
	// 		StatusCode:        statusCode,
	// 		DurationMs:        int(time.Since(startTime).Milliseconds()),
	// 		TTFTMs:            ttftMs,
	// 		TokensInput:       tokensIn,
	// 		TokensOutput:      tokensOut,
	// 		RequestSizeBytes:  requestSize,
	// 		ResponseSizeBytes: responseSize,
	// 	})
	// }()
}

func (p *Proxy) ServeStreaming(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	firstTokenTime := time.Now()
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	// Read request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	requestSize := len(bodyBytes)

	// Parse request to extract model and messages
	var reqMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Extract model name
	model := "unknown"
	if m, ok := reqMap["model"].(string); ok {
		model = m
	}

	// Count input tokens
	tokensIn := 0
	if messages, ok := reqMap["messages"].([]interface{}); ok {
		msgs := make([]map[string]interface{}, len(messages))
		for i, m := range messages {
			if msgMap, ok := m.(map[string]interface{}); ok {
				msgs[i] = msgMap
			}
		}
		tokensIn = p.tokenCounter.CountMessages(msgs)
	}

	// Prepare new request - strip /proxy prefix if present
	targetPath := r.URL.Path
	if strings.HasPrefix(targetPath, "/proxy") {
		targetPath = strings.TrimPrefix(targetPath, "/proxy")
	}

	newReq, err := http.NewRequestWithContext(r.Context(), r.Method, p.targetURL+targetPath, bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key := range r.Header {
		if key != "Host" && key != "Content-Length" {
			newReq.Header.Set(key, r.Header.Get(key))
		}
	}

	// Forward request
	resp, err := p.client.Do(newReq)
	if err != nil {
		recordRequest(p.db, &db.Request{
			ClientIP:         clientIP,
			UserAgent:        userAgent,
			Endpoint:         r.URL.Path,
			Model:            model,
			Method:           r.Method,
			StatusCode:       0,
			DurationMs:       int(time.Since(startTime).Milliseconds()),
			TokensInput:      tokensIn,
			RequestSizeBytes: requestSize,
		})
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Set streaming headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	var fullResponse strings.Builder
	tokensOut := 0

	// Stream response
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// Record time to first token
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
			}

			// Write to client
			w.Write(buf[:n])
			fullResponse.Write(buf[:n])

			// Flush to client
			flusher.Flush()
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			break
		}
	}

	durationMs := int(time.Since(startTime).Milliseconds())
	ttftMs := int(firstTokenTime.Sub(startTime).Milliseconds())

	// Count output tokens from full response
	tokensOut = p.countStreamingTokens(fullResponse.String())

	// Record request
	recordRequest(p.db, &db.Request{
		ClientIP:          clientIP,
		UserAgent:         userAgent,
		Endpoint:          r.URL.Path,
		Model:             model,
		Method:            r.Method,
		StatusCode:        http.StatusOK,
		DurationMs:        durationMs,
		TTFTMs:            ttftMs,
		TokensInput:       tokensIn,
		TokensOutput:      tokensOut,
		RequestSizeBytes:  requestSize,
		ResponseSizeBytes: fullResponse.Len(),
	})
}

func (p *Proxy) countStreamingTokens(response string) int {
	// Parse SSE format: "data: {...}\n\n"
	lines := strings.Split(response, "\n")
	var contentBuilder strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
				if choiceMap, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choiceMap["delta"].(map[string]interface{}); ok {
						if text, ok := delta["content"].(string); ok {
							contentBuilder.WriteString(text)
						}
					}
				}
			}
		}
	}

	return p.tokenCounter.CountTokens(contentBuilder.String())
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func recordRequest(db *db.DB, req *db.Request) {
	fmt.Printf("[DB] Recording request: %s %s\n", req.Method, req.Endpoint)
	if err := db.CreateRequest(req); err != nil {
		fmt.Printf("[DB] Failed to record request: %v\n", err)
	} else {
		fmt.Printf("[DB] Request recorded successfully\n")
	}
}
