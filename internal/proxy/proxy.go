package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
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
	log.Printf("[PROXY] Initialized with timeout: %d seconds", timeoutSeconds)
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

	log.Printf("[PROXY] === Request Start ===")
	log.Printf("[PROXY] Method: %s", r.Method)
	log.Printf("[PROXY] Path: %s", r.URL.Path)
	log.Printf("[PROXY] Client IP: %s", clientIP)
	log.Printf("[PROXY] User Agent: %s", r.UserAgent())
	log.Printf("[PROXY] Target URL: %s", p.targetURL)

	// Forward the request
	newReq, err := p.forwardRequest(r)
	if err != nil {
		log.Printf("[PROXY] === Request Failed ===")
		log.Printf("[PROXY] Error creating request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create forward request: %v", err), http.StatusBadGateway)
		return
	}

	log.Printf("[PROXY] Forwarding to: %s", newReq.URL.String())
	log.Printf("[PROXY] === Request Sent ===")

	resp, err := p.client.Do(newReq)
	if err != nil {
		log.Printf("[PROXY] === Request Failed ===")
		log.Printf("[PROXY] Error from upstream: %v", err)
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	log.Printf("[PROXY] === Response Received ===")
	log.Printf("[PROXY] Status: %d", resp.StatusCode)

	// Copy response headers
	for key := range resp.Header {
		w.Header().Set(key, resp.Header.Get(key))
	}

	w.WriteHeader(resp.StatusCode)

	// Stream response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("[PROXY] === Error copying response ===")
		log.Printf("[PROXY] Error: %v", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[PROXY] === Request Complete ===")
	log.Printf("[PROXY] Duration: %.3fs", duration.Seconds())
	log.Printf("[PROXY] Status: %d", resp.StatusCode)
}

// forwardRequest creates a new request to the upstream server
func (p *Proxy) forwardRequest(r *http.Request) (*http.Request, error) {
	log.Printf("[PROXY] Processing request...")
	log.Printf("[PROXY] Original URL: %s", r.URL.String())

	// Strip /proxy prefix if present
	targetPath := r.URL.Path
	if strings.HasPrefix(targetPath, p.targetPrefix) {
		targetPath = strings.TrimPrefix(targetPath, p.targetPrefix)
		log.Printf("[PROXY] Stripped prefix, new path: %s", targetPath)
	}

	// Construct full URL
	targetURL := p.targetURL + targetPath
	log.Printf("[PROXY] Constructed target URL: %s", targetURL)

	// Create new request
	newReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("[PROXY] Error creating request: %v", err)
		return nil, err
	}

	log.Printf("[PROXY] Request method: %s", r.Method)

	// Copy headers (except Host)
	headerCount := 0
	for key := range r.Header {
		if key != "Host" && key != "Content-Length" {
			newReq.Header.Set(key, r.Header.Get(key))
			headerCount++
		}
	}
	log.Printf("[PROXY] Copied %d headers (excluding Host and Content-Length)", headerCount)

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
