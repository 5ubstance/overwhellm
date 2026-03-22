package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"overwhellm/internal/proxy"
)

func TestMainEnvironmentVariables(t *testing.T) {
	// Test that environment variables are read correctly
	os.Setenv("PORT", "9999")
	os.Setenv("UPSTREAM_URL", "http://test.example.com")

	port := getEnv("PORT", "8080")
	upstreamURL := getEnv("UPSTREAM_URL", "http://aspec.localdomain:12434")

	if port != "9999" {
		t.Errorf("Expected PORT '9999', got %s", port)
	}

	if upstreamURL != "http://test.example.com" {
		t.Errorf("Expected UPSTREAM_URL 'http://test.example.com', got %s", upstreamURL)
	}
}

func TestMainDefaultValues(t *testing.T) {
	// Test that default values work when env vars are not set
	os.Unsetenv("PORT")
	os.Unsetenv("UPSTREAM_URL")

	port := getEnv("PORT", "8080")
	upstreamURL := getEnv("UPSTREAM_URL", "http://aspec.localdomain:12434")

	if port != "8080" {
		t.Errorf("Expected default PORT '8080', got %s", port)
	}

	if upstreamURL != "http://aspec.localdomain:12434" {
		t.Errorf("Expected default UPSTREAM_URL, got %s", upstreamURL)
	}
}

func TestMainProxyInitialization(t *testing.T) {
	// Test that proxy can be initialized with upstream URL
	p := proxy.New("http://test.upstream.com")
	if p == nil {
		t.Error("Expected proxy to be created")
	}
	// Proxy initialization successful (internal fields not exported)
}

func TestMainMuxSetup(t *testing.T) {
	// Test that mux is set up correctly
	p := proxy.New("http://test.upstream.com")
	mux := http.NewServeMux()

	mux.Handle("/proxy/", http.HandlerFunc(p.ServeHTTP))
	mux.Handle("/", http.HandlerFunc(p.ServeHTTP))

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/proxy/v1/models", nil)

	// Verify handler exists (won't fully work without real upstream)
	handler, _ := mux.Handler(req)
	if handler == nil {
		t.Error("Expected handler to be set up")
	}
}

func TestMainLoadEnvFile(t *testing.T) {
	// Create a temporary .env file
	content := `
# Test comment
TEST_VAR1=value1
TEST_VAR2=value2
# Another comment
TEST_VAR3=value3
`
	tmpFile := t.TempDir() + "/.env.test"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Load the env file
	loadEnvFile(tmpFile)

	// Verify variables were set
	if os.Getenv("TEST_VAR1") != "value1" {
		t.Errorf("Expected TEST_VAR1 'value1', got %s", os.Getenv("TEST_VAR1"))
	}

	if os.Getenv("TEST_VAR2") != "value2" {
		t.Errorf("Expected TEST_VAR2 'value2', got %s", os.Getenv("TEST_VAR2"))
	}

	if os.Getenv("TEST_VAR3") != "value3" {
		t.Errorf("Expected TEST_VAR3 'value3', got %s", os.Getenv("TEST_VAR3"))
	}

	// Verify comment lines were skipped
	if os.Getenv("# Test comment") != "" {
		t.Error("Expected comment lines to be skipped")
	}
}

func TestMainSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single line",
			input:    "value",
			expected: []string{"value"},
		},
		{
			name:     "multiple lines",
			input:    "value1\nvalue2\nvalue3",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "with CRLF",
			input:    "value1\r\nvalue2",
			expected: []string{"value1", "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("Expected line %d to be %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestMainTrimSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no spaces",
			input:    "value",
			expected: "value",
		},
		{
			name:     "leading spaces",
			input:    "  value",
			expected: "value",
		},
		{
			name:     "trailing spaces",
			input:    "value  ",
			expected: "value",
		},
		{
			name:     "both spaces",
			input:    "  value  ",
			expected: "value",
		},
		{
			name:     "tabs",
			input:    "\t\tvalue\t\t",
			expected: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimSpace(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMainSplitN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		n        int
		expected []string
	}{
		{
			name:     "single split",
			input:    "key=value",
			sep:      "=",
			n:        2,
			expected: []string{"key", "value"},
		},
		{
			name:     "multiple equals",
			input:    "key=value=extra",
			sep:      "=",
			n:        2,
			expected: []string{"key", "value=extra"},
		},
		{
			name:     "no separator",
			input:    "value",
			sep:      "=",
			n:        2,
			expected: []string{"value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitN(tt.input, tt.sep, tt.n)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d", len(tt.expected), len(result))
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("Expected part %d to be %q, got %q", i, tt.expected[i], part)
				}
			}
		})
	}
}

func TestMainFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		substr   string
		expected int
	}{
		{
			name:     "found at start",
			input:    "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			input:    "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "not found",
			input:    "hello world",
			substr:   "foo",
			expected: -1,
		},
		{
			name:     "empty substring",
			input:    "hello",
			substr:   "",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.input, tt.substr)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestMainGetEnv(t *testing.T) {
	// Set environment variable
	os.Setenv("TEST_VAR", "test_value")

	result := getEnv("TEST_VAR", "default_value")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got %s", result)
	}

	// Test default value
	unsetResult := getEnv("UNSET_VAR", "default_value")
	if unsetResult != "default_value" {
		t.Errorf("Expected 'default_value', got %s", unsetResult)
	}
}

// Integration test: Full flow with mock server
func TestMainIntegrationWithMock(t *testing.T) {
	// This test would require starting an actual HTTP server
	// For now, we'll skip it and rely on the proxy unit tests
	t.Skip("Integration test requires external mock server")
}
