package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	info("%sInitialized with timeout:%s %d seconds", colorBlue, colorReset, timeoutSeconds)
	info("%sTarget URL:%s %s", colorBlue, colorReset, targetURL)
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

	debug("%s=== Request Start ===%s", colorCyan, colorReset)
	debug("%sMethod:%s %s", colorBlue, colorReset, r.Method)
	debug("%sPath:%s %s", colorBlue, colorReset, r.URL.Path)
	debug("%sClient IP:%s %s", colorBlue, colorReset, clientIP)
	debug("%sUser Agent:%s %s", colorBlue, colorReset, r.UserAgent())
	debug("%sTarget URL:%s %s", colorBlue, colorReset, p.targetURL)

	newReq, err := p.forwardRequest(r)
	if err != nil {
		errorf("[PROXY] === Request Failed ===")
		errorf("[PROXY] Error creating request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create forward request: %v", err), http.StatusBadGateway)
		return
	}

	debug("%sForwarding to:%s %s", colorBlue, colorReset, newReq.URL.String())
	debug("%s=== Request Sent ===%s", colorCyan, colorReset)

	resp, err := p.client.Do(newReq)
	if err != nil {
		errorf("%s=== Request Failed ===%s", colorRed, colorReset)
		errorf("%sError from upstream:%s %v", colorRed, colorReset, err)
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	debug("%s=== Response Received ===%s", colorCyan, colorReset)
	debug("%sStatus:%s %d", colorBlue, colorReset, resp.StatusCode)

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

		info("%s=== Request Complete ===%s", colorGreen, colorReset)
		info("%sTotal duration:%s %.3fs", colorBlue, colorReset, stats.TotalDuration.Seconds())
		if stats.GenerationDuration > 0 {
			info("%sGeneration time:%s %.3fs", colorYellow, colorReset, stats.GenerationDuration.Seconds())
		}
		info("%sChunks:%s %d", colorCyan, colorReset, stats.Chunks)
		if stats.Chunks > 1 && stats.GenerationDuration > 0 {
			info("%sChunks/sec:%s %.2f", colorMagenta, colorReset, stats.ChunksPerSecond)
		}

		if stats.CompletionTokens > 0 {
			info("%sTokens:%s %d total (%d prompt + %d completion)", colorCyan, colorReset, stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens)
			info("%sSpeed:%s %.2f tokens/sec (completion)", colorMagenta, colorReset, stats.TokensPerSecond)
		}
		info("%sStatus:%s %d", colorBlue, colorReset, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errorf("[PROXY] === Error reading response ===%s", colorReset)
		errorf("[PROXY] %sError:%s %v", colorRed, colorReset, err)
		return
	}

	w.Write(body)

	info("%s=== Request Complete ===%s", colorGreen, colorReset)
	info("%sDuration:%s %.3fs", colorBlue, colorReset, duration.Seconds())

	stats, _ := ParseTokenUsageFromBytes(body)
	if stats != nil && stats.TotalTokens > 0 {
		info("%sTokens:%s %d total (%d prompt + %d completion)", colorCyan, colorReset, stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens)
		tokensPerSec := float64(stats.CompletionTokens) / duration.Seconds()
		info("%sSpeed:%s %.2f tokens/sec (completion)", colorMagenta, colorReset, tokensPerSec)
	}
	info("%sStatus:%s %d", colorBlue, colorReset, resp.StatusCode)
}

// forwardRequest creates a new request to the upstream server
func (p *Proxy) forwardRequest(r *http.Request) (*http.Request, error) {
	debug("%sProcessing request...%s", colorCyan, colorReset)
	debug("%sOriginal URL:%s %s", colorBlue, colorReset, r.URL.String())

	// Show request payload at DEBUG level
	if r.Body != nil && r.ContentLength > 0 {
		bodyBytes, _ := io.ReadAll(r.Body)
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
			debug("%sRequest payload:%s %s", colorYellow, colorReset, colorBlue+colorBold+colorJSONHighlight(prettyJSON.String())+colorReset)
		} else {
			debug("%sRequest payload:%s %s", colorYellow, colorReset, string(bodyBytes))
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Strip /proxy prefix if present
	targetPath := r.URL.Path
	if strings.HasPrefix(targetPath, p.targetPrefix) {
		targetPath = strings.TrimPrefix(targetPath, p.targetPrefix)
		warn("%sStripped prefix,%s new path: %s", colorYellow, colorReset, targetPath)
	}

	// Construct full URL
	targetURL := p.targetURL + targetPath
	debug("%sConstructed target URL:%s %s", colorBlue, colorReset, targetURL)

	// Create new request
	newReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		errorf("%sError creating request:%s %v", colorRed, colorReset, err)
		return nil, err
	}

	debug("%sRequest method:%s %s", colorBlue, colorReset, r.Method)

	// Copy headers (except Host)
	headerCount := 0
	for key := range r.Header {
		if key != "Host" && key != "Content-Length" {
			newReq.Header.Set(key, r.Header.Get(key))
			headerCount++
		}
	}
	debug("%sCopied %d headers (excluding Host and Content-Length)", colorBlue, headerCount)

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

// colorJSONHighlight adds ANSI color codes to JSON for syntax highlighting
func colorJSONHighlight(jsonStr string) string {
	var result strings.Builder
	inString := false
	escape := false
	i := 0

	for i < len(jsonStr) {
		ch := jsonStr[i]

		if escape {
			result.WriteByte(ch)
			escape = false
			i++
			continue
		}

		if ch == '\\' && inString {
			result.WriteString(colorReset + string(ch))
			escape = true
			i++
			continue
		}

		if ch == '"' && !escape {
			inString = !inString
			if inString {
				result.WriteString(colorGreen)
			} else {
				result.WriteString(colorReset)
			}
			result.WriteByte(ch)
			i++
			continue
		}

		if inString {
			result.WriteByte(ch)
			i++
			continue
		}

		if ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ',' {
			result.WriteString(colorCyan)
			result.WriteByte(ch)
			result.WriteString(colorReset)
			i++
			continue
		}

		if ch == ':' {
			result.WriteString(colorYellow)
			result.WriteByte(ch)
			result.WriteString(colorReset)
			i++
			continue
		}

		if ch == 't' && i+4 < len(jsonStr) && jsonStr[i:i+4] == "true" {
			result.WriteString(colorMagenta + "true" + colorReset)
			i += 4
			continue
		}

		if ch == 'f' && i+5 < len(jsonStr) && jsonStr[i:i+5] == "false" {
			result.WriteString(colorMagenta + "false" + colorReset)
			i += 5
			continue
		}

		if ch == 'n' && i+4 < len(jsonStr) && jsonStr[i:i+4] == "null" {
			result.WriteString(colorRed + "null" + colorReset)
			i += 4
			continue
		}

		if (ch >= '0' && ch <= '9') || ch == '-' {
			// Find end of number
			j := i
			for j < len(jsonStr) && ((jsonStr[j] >= '0' && jsonStr[j] <= '9') || jsonStr[j] == '.' || jsonStr[j] == '-' || jsonStr[j] == 'e' || jsonStr[j] == 'E') {
				j++
			}
			result.WriteString(colorBold + jsonStr[i:j] + colorReset)
			i = j
			continue
		}

		result.WriteByte(ch)
		i++
	}

	return result.String()
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
