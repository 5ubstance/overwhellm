package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyNew(t *testing.T) {
	p := New("http://example.com", 60)
	if p.client == nil {
		t.Error("Expected client to be initialized")
	}
	if p.targetURL != "http://example.com" {
		t.Errorf("Expected targetURL 'http://example.com', got %s", p.targetURL)
	}
	if p.targetPrefix != "/proxy" {
		t.Errorf("Expected targetPrefix '/proxy', got %s", p.targetPrefix)
	}
}

func TestProxyForwardRequest(t *testing.T) {
	p := New("http://upstream.com", 60)

	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	newReq, err := p.forwardRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if newReq.URL.String() != "http://upstream.com/v1/models" {
		t.Errorf("Expected URL 'http://upstream.com/v1/models', got %s", newReq.URL.String())
	}

	// Check X-Forwarded-For header
	xff := newReq.Header.Get("X-Forwarded-For")
	if xff != "192.168.1.1" {
		t.Errorf("Expected X-Forwarded-For '192.168.1.1', got %s", xff)
	}

	// Check Host header is not copied
	if newReq.Header.Get("Host") != "" {
		t.Error("Expected Host header not to be copied")
	}
}

func TestProxyForwardRequestNoPrefix(t *testing.T) {
	p := New("http://upstream.com", 60)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	newReq, err := p.forwardRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if newReq.URL.String() != "http://upstream.com/v1/models" {
		t.Errorf("Expected URL 'http://upstream.com/v1/models', got %s", newReq.URL.String())
	}
}

func TestProxyCopyHeaders(t *testing.T) {
	src := make(http.Header)
	src.Set("Content-Type", "application/json")
	src.Set("X-Custom-Header", "test-value")
	src.Set("Host", "example.com")

	dst := make(http.Header)
	copyHeaders(dst, src)

	if dst.Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type to be copied")
	}

	if dst.Get("X-Custom-Header") != "test-value" {
		t.Error("Expected X-Custom-Header to be copied")
	}

	if dst.Get("Host") != "" {
		t.Error("Expected Host header not to be copied")
	}
}

func TestProxyGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		expected string
	}{
		{
			name: "X-Forwarded-For header",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
				return req
			}(),
			expected: "192.168.1.1",
		},
		{
			name: "X-Real-IP header",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Real-IP", "10.0.0.1")
				return req
			}(),
			expected: "10.0.0.1",
		},
		{
			name: "RemoteAddr fallback",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			}(),
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := getClientIP(tt.req)
			if ip != tt.expected {
				t.Errorf("Expected IP %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestProxyServeHTTP(t *testing.T) {
	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"test": "response"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := New(mockServer.URL, 60)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	p.ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test") || !strings.Contains(body, "response") {
		t.Errorf("Expected response body to contain test response, got %s", body)
	}
}
