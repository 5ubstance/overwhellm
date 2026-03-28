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

	"overwhellm/internal/proxy"
)

type Config struct {
	Port        string `json:"port"`
	UpstreamURL string `json:"upstream_url"`
	Timeout     int    `json:"timeout"`
	LogLevel    string `json:"log_level"`
	LogFile     string `json:"log_file"`
}

const (
	Version     = "0.1.0"
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
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

func loadConfigFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("invalid config.json: %v", err)
	}

	return &config, nil
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

func displayBanner(port, upstream string, timeout int, logLevel, logFile string) {
	bannerContent, err := os.ReadFile("./banner")
	if err != nil {
		fmt.Println()
		return
	}

	lines := splitLines(string(bannerContent))
	
	// Print banner lines with info on the right
	fmt.Printf("%s", colorCyan)
	for i, line := range lines {
		// Format each line with info on the right
		info := ""
		switch i {
		case 0:
			info = fmt.Sprintf("%soverwhellm starting...%s %s%s", colorGreen, colorBold, colorReset, Version)
		case 1:
			info = fmt.Sprintf("%s   Listen:%s :%s%s%s", colorBlue, colorReset, colorBold, port, colorReset)
		case 2:
			info = fmt.Sprintf("%s   Upstream:%s %s", colorBlue, colorReset, upstream)
		case 3:
			info = fmt.Sprintf("%s   Timeout:%s %d", colorBlue, colorReset, timeout)
		case 4:
			info = fmt.Sprintf("%s   Log Level:%s %s", colorBlue, colorReset, logLevel)
		case 5:
			info = fmt.Sprintf("%s   Log File:%s %s", colorBlue, colorReset, logFile)
		}
		
		// Pad banner line to ensure alignment
		if len(line) < 80 {
			fmt.Printf("%-80s%s\n", line, info)
		} else {
			fmt.Printf("%s\n", line)
		}
	}
	fmt.Printf("%s", colorReset)
	
	// Print footer
	fmt.Println()
	fmt.Printf("%sPress Ctrl+C to stop%s\n", colorCyan, colorReset)
}

func main() {
	// Load .env file
	loadEnvFile(".env")

	// Load config.json
	config, err := loadConfigFile("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			config = &Config{} // Use defaults
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Could not load config.json: %v\n", err)
			fmt.Fprintf(os.Stderr, "Continuing with defaults...\n")
			config = &Config{}
		}
	}

	// Parse command line flags (CLI overrides config.json and .env)
	port := flag.String("port", "", "Proxy server port")
	upstreamURL := flag.String("upstream-url", "", "Upstream LLM URL")
	timeout := flag.Int("timeout", 0, "HTTP client timeout in seconds")
	logLevel := flag.String("log-level", "", "Log level (TRACE, DEBUG, INFO, WARN, ERROR, CRITICAL)")
	logFile := flag.String("log-file", "", "Log file path")
	flag.Parse()

	// Apply configuration with priority: CLI > config.json > .env > defaults
	finalPort := *port
	if finalPort == "" && config.Port != "" {
		finalPort = config.Port
	} else if finalPort == "" {
		finalPort = getEnv("PORT", "8080")
	}

	finalUpstream := *upstreamURL
	if finalUpstream == "" && config.UpstreamURL != "" {
		finalUpstream = config.UpstreamURL
	} else if finalUpstream == "" {
		finalUpstream = getEnv("UPSTREAM_URL", "http://aspec.localdomain:12434")
	}

	finalTimeout := *timeout
	if finalTimeout == 0 && config.Timeout > 0 {
		finalTimeout = config.Timeout
	} else if finalTimeout == 0 {
		finalTimeout = getEnvInt("TIMEOUT", 60)
	}

	finalLogLevel := *logLevel
	if finalLogLevel == "" && config.LogLevel != "" {
		finalLogLevel = config.LogLevel
	} else if finalLogLevel == "" {
		finalLogLevel = getEnv("LOG_LEVEL", "INFO")
	}

	finalLogFile := *logFile
	if finalLogFile == "" && config.LogFile != "" {
		finalLogFile = config.LogFile
	} else if finalLogFile == "" {
		finalLogFile = getEnv("LOG_FILE", "overwhellm.log")
	}

	// Set log level and file
	proxy.SetLogLevel(finalLogLevel)
	proxy.SetLogFile(finalLogFile)

	// Create proxy with configured timeout
	p := proxy.New(finalUpstream, finalTimeout)

	// Create router
	mux := http.NewServeMux()

	// Proxy routes - forward to upstream
	mux.Handle("/proxy/", http.HandlerFunc(p.ServeHTTP))
	mux.Handle("/", http.HandlerFunc(p.ServeHTTP))

// Create server
	server := &http.Server{
		Addr:         ":" + finalPort,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	// Display banner
	displayBanner(finalPort, finalUpstream, finalTimeout, finalLogLevel, finalLogFile)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
