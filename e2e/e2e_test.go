package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overwhellm/internal/proxy"
)

// TestE2EBasicFlow tests the full proxy flow with a mock upstream server
func TestE2EBasicFlow(t *testing.T) {
	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[{"id":"mock-model-1","object":"model","owned_by":"mock"}]}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["object"] != "list" {
		t.Errorf("Expected object 'list', got %v", response["object"])
	}
}

// TestE2EHeadersPreserved tests that headers are correctly passed through
func TestE2EHeadersPreserved(t *testing.T) {
	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that custom header was passed through
		customHeader := r.Header.Get("X-Custom-Header")
		if customHeader != "test-value" {
			t.Errorf("Expected X-Custom-Header 'test-value', got %s", customHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Create test request with custom headers
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	req.Header.Set("X-Custom-Header", "test-value")
	req.Header.Set("X-Another-Header", "another-value")
	req.RemoteAddr = "10.0.0.1:54321"

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
}

// TestE2EClientIP tests that client IP is correctly extracted and forwarded
func TestE2EClientIP(t *testing.T) {
	var receivedIP string

	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xff := r.Header.Get("X-Forwarded-For")
		receivedIP = xff
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	p.ServeHTTP(w, req)

	// Check that client IP was forwarded
	if receivedIP != "192.168.1.100" {
		t.Errorf("Expected X-Forwarded-For '192.168.1.100', got %s", receivedIP)
	}
}

// TestE2EPathPrefixStripping tests that /proxy prefix is correctly stripped
func TestE2EPathPrefixStripping(t *testing.T) {
	var receivedPath string

	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Create test request with /proxy prefix
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	// Check that prefix was stripped
	if receivedPath != "/v1/models" {
		t.Errorf("Expected path '/v1/models', got %s", receivedPath)
	}
}

// TestE2EWithoutPrefix tests requests without /proxy prefix
func TestE2EWithoutPrefix(t *testing.T) {
	var receivedPath string

	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Create test request without /proxy prefix
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	// Check that path was not modified
	if receivedPath != "/v1/models" {
		t.Errorf("Expected path '/v1/models', got %s", receivedPath)
	}
}

// TestE2EMultipleRequests tests multiple sequential requests
func TestE2EMultipleRequests(t *testing.T) {
	requestCount := 0

	// Create mock upstream server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request":` + string(rune(requestCount)) + `}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
		w := httptest.NewRecorder()
		p.ServeHTTP(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check that response contains the request number
		expected := `{"request":` + string(rune(i+1)) + `}`
		if !strings.Contains(string(body), expected) {
			t.Errorf("Request %d: expected %s, got %s", i, expected, string(body))
		}
	}

	if requestCount != 5 {
		t.Errorf("Expected 5 requests, got %d", requestCount)
	}
}

// TestE2EErrorPropagation tests that errors from upstream are propagated
func TestE2EErrorPropagation(t *testing.T) {
	// Create mock upstream server that returns error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer mockServer.Close()

	// Create proxy
	p := proxy.New(mockServer.URL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)
	w := httptest.NewRecorder()

	// Serve request
	p.ServeHTTP(w, req)

	// Check that error status was propagated
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}
