package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	colorReset   = "\033[0m"
	colorGreen   = "\033[32m"
	colorBlue    = "\033[34m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
	colorMagenta = "\033[35m"
)

// Proxy represents the HTTP proxy
type Proxy struct {
	client       *http.Client
	targetURL    string
	targetPrefix string
}

// New creates a new proxy instance with configurable timeout
func New(targetURL string, timeoutSeconds int) *Proxy {
	timeout := time.Duration(timeoutSeconds) * time.Second
	fmt.Printf("[PROXY]%s Initialized with timeout:%s %d seconds\n", colorBlue, colorReset, timeoutSeconds)
	return &Proxy{
		client: &http.Client{
			Timeout: timeout,
		},
		targetURL:    targetURL,
		targetPrefix: "/proxy",
	}
}

// ServeHTTP handles incoming HTTP requests
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	clientIP := getClientIP(r)

	fmt.Printf("[PROXY]%s === Request Start ===%s\n", colorCyan, colorReset)
	fmt.Printf("[PROXY] %sMethod:%s %s\n", colorBlue, colorReset, r.Method)
	fmt.Printf("[PROXY] %sPath:%s %s\n", colorBlue, colorReset, r.URL.Path)
	fmt.Printf("[PROXY] %sClient IP:%s %s\n", colorBlue, colorReset, clientIP)
	fmt.Printf("[PROXY] %sUser Agent:%s %s\n", colorBlue, colorReset, r.UserAgent())
	fmt.Printf("[PROXY] %sTarget URL:%s %s\n", colorBlue, colorReset, p.targetURL)

	newReq, err := p.forwardRequest(r)
	if err != nil {
		log.Printf("[PROXY] === Request Failed ===")
		log.Printf("[PROXY] Error creating request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create forward request: %v", err), http.StatusBadGateway)
		return
	}

	fmt.Printf("[PROXY] %sForwarding to:%s %s\n", colorBlue, colorReset, newReq.URL.String())
	fmt.Printf("[PROXY]%s === Request Sent ===%s\n", colorCyan, colorReset)

	resp, err := p.client.Do(newReq)
	if err != nil {
		fmt.Printf("[PROXY]%s === Request Failed ===%s\n", colorRed, colorReset)
		fmt.Printf("[PROXY] %sError from upstream:%s %v\n", colorRed, colorReset, err)
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("[PROXY]%s === Response Received ===%s\n", colorCyan, colorReset)
	fmt.Printf("[PROXY] %sStatus:%s %d\n", colorBlue, colorReset, resp.StatusCode)

	for key := range resp.Header {
		w.Header().Set(key, resp.Header.Get(key))
	}

	w.WriteHeader(resp.StatusCode)

	duration := time.Since(startTime)
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "application/x-ndjson") {
		tracker := &TokenTrackingReader{
			Body:      resp.Body,
			StartTime: startTime,
			done:      make(chan struct{}),
		}

		go func() {
			buf := make([]byte, 8192)
			for {
				n, err := tracker.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
					w.(http.Flusher).Flush()
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
			}
			tracker.Close()
		}()

		<-tracker.done
		stats := tracker.GetStats()

		fmt.Printf("[PROXY] === Request Complete ===%s\n", colorGreen, colorReset)
		fmt.Printf("[PROXY] %sDuration:%s %.3fs\n", colorBlue, colorReset, duration.Seconds())
		fmt.Printf("[PROXY] %sChunks:%s %d\n", colorCyan, colorReset, stats.Chunks)
		if stats.Chunks > 1 {
			fmt.Printf("[PROXY] %sAvg chunk duration:%s %.3fs\n", colorYellow, colorReset, stats.ChunkDurationSeconds)
			fmt.Printf("[PROXY] %sChunks/sec:%s %.2f\n", colorMagenta, colorReset, stats.ChunksPerSecond)
		}

		if stats.TotalTokens > 0 {
			fmt.Printf("[PROXY] %sTokens:%s %d total (%d prompt + %d completion)\n", colorCyan, colorReset, stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens)
			fmt.Printf("[PROXY] %sSpeed:%s %.2f tokens/sec\n", colorMagenta, colorReset, stats.TokensPerSecond)
		}
		fmt.Printf("[PROXY] %sStatus:%s %d\n", colorBlue, colorReset, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[PROXY] === Error reading response ===%s\n", colorReset)
		fmt.Printf("[PROXY] %sError:%s %v\n", colorRed, colorReset, err)
		return
	}

	w.Write(body)

	fmt.Printf("[PROXY] === Request Complete ===%s\n", colorGreen, colorReset)
	fmt.Printf("[PROXY] %sDuration:%s %.3fs\n", colorBlue, colorReset, duration.Seconds())

	stats, _ := ParseTokenUsageFromBytes(body)
	if stats != nil && stats.TotalTokens > 0 {
		fmt.Printf("[PROXY] %sTokens:%s %d total (%d prompt + %d completion)\n", colorCyan, colorReset, stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens)
		tokensPerSec := float64(stats.TotalTokens) / duration.Seconds()
		fmt.Printf("[PROXY] %sSpeed:%s %.2f tokens/sec\n", colorMagenta, colorReset, tokensPerSec)
	}
	fmt.Printf("[PROXY] %sStatus:%s %d\n", colorBlue, colorReset, resp.StatusCode)
}

// forwardRequest creates a new request to the upstream server
func (p *Proxy) forwardRequest(r *http.Request) (*http.Request, error) {
	fmt.Printf("[PROXY] %sProcessing request...%s\n", colorCyan, colorReset)
	fmt.Printf("[PROXY] %sOriginal URL:%s %s\n", colorBlue, colorReset, r.URL.String())

	// Strip /proxy prefix if present
	targetPath := r.URL.Path
	if strings.HasPrefix(targetPath, p.targetPrefix) {
		targetPath = strings.TrimPrefix(targetPath, p.targetPrefix)
		fmt.Printf("[PROXY] %sStripped prefix,%s new path: %s\n", colorYellow, colorReset, targetPath)
	}

	// Construct full URL
	targetURL := p.targetURL + targetPath
	fmt.Printf("[PROXY] %sConstructed target URL:%s %s\n", colorBlue, colorReset, targetURL)

	// Create new request
	newReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		fmt.Printf("[PROXY] %sError creating request:%s %v\n", colorRed, colorReset, err)
		return nil, err
	}

	fmt.Printf("[PROXY] %sRequest method:%s %s\n", colorBlue, colorReset, r.Method)

	// Copy headers (except Host)
	headerCount := 0
	for key := range r.Header {
		if key != "Host" && key != "Content-Length" {
			newReq.Header.Set(key, r.Header.Get(key))
			headerCount++
		}
	}
	fmt.Printf("[PROXY] %sCopied %d headers (excluding Host and Content-Length)\n", colorBlue, headerCount)

	// Set X-Forwarded-For header
	newReq.Header.Set("X-Forwarded-For", getClientIP(r))

	return newReq, nil
}

// copyHeaders copies headers from one HTTP header map to another
func copyHeaders(dst, src http.Header) {
	for key := range src {
		if key != "Host" {
			dst.Set(key, src.Get(key))
		}
	}
}

// getClientIP extracts the client IP from the request
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
