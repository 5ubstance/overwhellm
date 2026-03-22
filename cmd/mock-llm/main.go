package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func main() {
	port := flag.String("port", getEnv("PORT", "9090"), "Mock server port")
	flag.Parse()

	http.HandleFunc("/v1/models", handleModels)
	http.HandleFunc("/v1/chat/completions", handleChatCompletions)

	server := &http.Server{
		Addr:         ":" + *port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println()
	fmt.Printf("%s🎭 %sMock LLM Server starting...%s\n", colorGreen, colorBold, colorReset)
	fmt.Printf("%s   Listen:%s :%s\n", colorBlue, colorReset, colorBold+*port+colorReset)
	fmt.Printf("%s   Endpoint:%s %s/v1/models%s\n", colorBlue, colorReset, colorCyan, colorReset)
	fmt.Println()
	fmt.Printf("%sPress Ctrl+C to stop%s\n", colorCyan, colorReset)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       "mock-model-1",
				"object":   "model",
				"owned_by": "mock",
			},
		},
	}

	body, err := json.Marshal(response)
	if err != nil {
		log.Printf("[MOCK]%s Error marshaling response:%s %v", colorRed, colorReset, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)

	duration := time.Since(startTime)
	log.Printf("[MOCK]%s %s %s-> 200 OK%s (%.3fs)", colorBlue, r.Method, colorGreen, colorReset, duration.Seconds())
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	var reqMap map[string]interface{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[MOCK]%s Error reading request body:%s %v", colorRed, colorReset, err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &reqMap); err != nil {
		log.Printf("[MOCK]%s Error parsing JSON:%s %v", colorRed, colorReset, err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Extract model and messages from request
	model := "mock-model-1"
	if m, ok := reqMap["model"].(string); ok {
		model = m
	}

	// Create mock response
	response := map[string]interface{}{
		"id":      "chatcmpl-mock-12345",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "This is a mock response from the test server.",
				},
				"logprobs":      nil,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 15,
			"total_tokens":      25,
		},
	}

	respBody, err := json.Marshal(response)
	if err != nil {
		log.Printf("[MOCK]%s Error marshaling response:%s %v", colorRed, colorReset, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)

	duration := time.Since(startTime)
	log.Printf("[MOCK]%s %s %s-> 200 OK%s (%.3fs)", colorBlue, r.Method, colorGreen, colorReset, duration.Seconds())
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
