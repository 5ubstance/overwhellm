package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"overwhellm/internal/db"
	"overwhellm/internal/proxy"
	"overwhellm/internal/ui"
)

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	lines, _ := io.ReadAll(file)
	for _, line := range strings.Split(string(lines), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, value)
			}
		}
	}
}

func main() {
	// Load .env file
	loadEnvFile(".env")

	// Parse command line flags
	proxyPort := flag.String("port", os.Getenv("PROXY_PORT"), "Proxy server port")
	llamaURL := flag.String("llama-url", os.Getenv("LLAMA_CPP_URL"), "llama.cpp server URL")
	dbPath := flag.String("db", os.Getenv("DB_PATH"), "Database path")
	logPath := flag.String("log", os.Getenv("LOG_PATH"), "Log file path")
	flag.Parse()

	// Set defaults if still empty
	if *proxyPort == "" {
		*proxyPort = "9000"
	}
	if *llamaURL == "" {
		*llamaURL = "http://localhost:8080"
	}
	if *dbPath == "" {
		*dbPath = "./overwhellm.db"
	}
	if *logPath == "" {
		*logPath = "./overwhellm.log"
	}

	// Open log file
	logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Create buffered log writer
	logWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(logWriter)
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[MAIN] ")

	// Initialize database
	fmt.Println("📦 Initializing database...")
	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	fmt.Println("✅ Database initialized")

	// Initialize proxy
	fmt.Println("🔄 Initializing proxy...")
	proxyInstance, err := proxy.New(*llamaURL, database)
	if err != nil {
		log.Fatalf("Failed to initialize proxy: %v", err)
	}
	fmt.Println("✅ Proxy initialized")

	// Initialize UI
	fmt.Println("🎨 Initializing dashboard...")
	dashboard := ui.New(database)

	// Create router
	mux := http.NewServeMux()

	// Dashboard routes (root path)
	dashboard.SetupMux(mux)

	// Proxy routes - forward to llama.cpp (under /proxy prefix)
	mux.HandleFunc("/proxy/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Check if streaming
		if r.URL.Query().Get("stream") == "true" {
			proxyInstance.ServeStreaming(w, r)
		} else {
			proxyInstance.ServeHTTP(w, r)
		}
	})

	// Other OpenAI-compatible endpoints
	mux.HandleFunc("/proxy/v1/models", func(w http.ResponseWriter, r *http.Request) {
		proxyInstance.ServeHTTP(w, r)
	})

	// Health check (dashboard endpoint)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Create server
	server := &http.Server{
		Addr:         ":" + *proxyPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Println()
	fmt.Println("🚀 LLM Proxy Dashboard starting...")
	fmt.Println("   Proxy Port: " + *proxyPort)
	fmt.Println("   Llama URL: " + *llamaURL)
	fmt.Println("   Database: " + *dbPath)
	fmt.Println("   Dashboard: http://localhost:" + *proxyPort + "/")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
