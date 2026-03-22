package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"overwhellm/internal/proxy"
)

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	lines, _ := io.ReadAll(file)
	for _, line := range splitLines(string(lines)) {
		line = trimSpace(line)
		if line == "" || startsWith(line, "#") {
			continue
		}
		parts := splitN(line, "=", 2)
		if len(parts) == 2 {
			key := trimSpace(parts[0])
			value := trimSpace(parts[1])
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, value)
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, c := range s {
		if c == '\n' || c == '\r' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	// Remove empty lines that result from CRLF
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func splitN(s string, sep string, n int) []string {
	var result []string
	start := 0
	for n > 1 && start < len(s) {
		idx := findSubstring(s[start:], sep)
		if idx == -1 {
			break
		}
		result = append(result, s[start:start+idx])
		start += idx + len(sep)
		n--
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var parsed int
		fmt.Sscanf(value, "%d", &parsed)
		if parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

func main() {
	// Load .env file
	loadEnvFile(".env")

	// Parse command line flags
	port := flag.String("port", getEnv("PORT", "8080"), "Proxy server port")
	upstreamURL := flag.String("upstream-url", getEnv("UPSTREAM_URL", "http://aspec.localdomain:12434"), "Upstream LLM URL")
	timeout := flag.Int("timeout", getEnvInt("TIMEOUT", 60), "HTTP client timeout in seconds")
	flag.Parse()

	// Create proxy with configured timeout
	p := proxy.New(*upstreamURL, *timeout)

	// Create router
	mux := http.NewServeMux()

	// Proxy routes - forward to upstream
	mux.Handle("/proxy/", http.HandlerFunc(p.ServeHTTP))
	mux.Handle("/", http.HandlerFunc(p.ServeHTTP))

	// Create server
	server := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	fmt.Println()
	fmt.Println("🚀 overwhellm starting...")
	fmt.Println("   Listen: :" + *port)
	fmt.Println("   Upstream: " + *upstreamURL)
	fmt.Printf("   Timeout: %ds\n", *timeout)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
