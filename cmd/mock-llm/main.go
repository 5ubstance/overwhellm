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
	fmt.Println("🎭 Mock LLM Server starting...")
	fmt.Println("   Listen: :" + *port)
	fmt.Println("   Endpoint: /v1/models")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

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
		log.Printf("[MOCK] Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)

	duration := time.Since(startTime)
	log.Printf("[MOCK] %s %s -> 200 OK (%.3fs)", r.Method, r.URL.Path, duration.Seconds())
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
		log.Printf("[MOCK] Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &reqMap); err != nil {
		log.Printf("[MOCK] Error parsing JSON: %v", err)
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
		log.Printf("[MOCK] Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)

	duration := time.Since(startTime)
	log.Printf("[MOCK] %s %s -> 200 OK (%.3fs)", r.Method, r.URL.Path, duration.Seconds())
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
