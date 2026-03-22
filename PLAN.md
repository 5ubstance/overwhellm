# Transparent Proxy Implementation Plan

## Overview

Build a transparent HTTP proxy in Go that forwards traffic from opencode sessions to a remote LLM (aspec.localdomain:12434). The proxy listens on localhost:8080 and transparently forwards all requests to the upstream LLM.

## Architecture

```
opencode session (localhost:8080)
        ↓
    Go Proxy (port 8080)
        ↓
aspec.localdomain:12434 (upstream LLM)
```

## Technical Specifications

### Proxy Configuration
- **Listen port**: 8080 (configurable via `PORT` env)
- **Upstream URL**: http://aspec.localdomain:12434 (configurable via `UPSTREAM_URL` env)
- **HTTP client timeout**: 10 seconds
- **HTTP server timeout**: 10 seconds

### Behavior
- Transparent pass-through of all HTTP requests/responses
- Preserve all headers except `Host`
- Strip `/proxy` prefix from request path if present
- Stream responses byte-by-byte for efficiency
- Log all requests to stdout with timestamps
- No health check endpoint
- No graceful shutdown (keep simple)
- No retry logic
- Simple error propagation (upstream errors returned as-is)

### Mock Server Configuration
- **Listen port**: 9090 (configurable via `PORT` env)
- **Endpoint**: `/v1/models` (GET only)
- **Response**: Simple JSON (non-streaming)
- **Purpose**: Testing without live LLM backend

## Project Structure

```
/home/philippe/projects/overwhellm/
├── cmd/
│   ├── overwhellm/
│   │   └── main.go              # Proxy server entry point
│   └── mock-llm/
│       └── main.go              # Mock LLM server (standalone)
├── internal/
│   ├── proxy/
│   │   ├── proxy.go             # Proxy handler logic
│   │   └── proxy_test.go        # Unit tests for proxy
│   └── mocks/
│       └── mock_server.go       # In-memory mock server utilities
├── e2e/
│   └── e2e_test.go              # End-to-end tests
├── go.mod
├── go.sum
├── .env                         # Proxy configuration
└── .env.mock                    # Mock server configuration
```

## Implementation Phases

### Phase 1: Project Setup & Go Module

**Files to Create:**
- `go.mod` - Minimal module definition (no external dependencies)
- `.env` - Proxy configuration template
- `.env.mock` - Mock server configuration template
- `PLAN.md` - This implementation plan

**Configuration:**

`.env`:
```bash
# overwhellm Proxy Configuration
PORT=8080
UPSTREAM_URL=http://aspec.localdomain:12434
```

`.env.mock`:
```bash
# Mock LLM Server Configuration
PORT=9090
ENDPOINT=/v1/models
```

**Verification:**
- Build module: `go build -o /dev/null ./...`
- Run tests: `go test ./...`

---

### Phase 2: Mock LLM Server

**File:** `cmd/mock-llm/main.go`

**Features:**
- Standalone binary: `./mock-llm`
- Listen on port 9090 (configurable via `PORT` env)
- Single endpoint: `/v1/models` (GET)
- Response: Simple JSON
  ```json
  {
    "object": "list",
    "data": [
      {
        "id": "mock-model-1",
        "object": "model",
        "owned_by": "mock"
      }
    ]
  }
  ```
- HTTP status: 200 OK
- Headers: `Content-Type: application/json`
- No streaming support
- Log requests to stdout

**Logging Format:**
```
[MOCK] 2026/03/21 12:00:00 GET /v1/models -> 200 OK
```

**Testing:**
- Unit test: Verify JSON response format
- Manual test: `curl http://localhost:9090/v1/models`

---

### Phase 3: Proxy Core (Unit Tests)

**File:** `internal/proxy/proxy.go`

**Core Functions:**

1. **`type Proxy struct`**
   - `client *http.Client` - HTTP client with 10s timeout
   - `targetURL string` - Upstream URL
   - `targetPrefix string` - URL path prefix to strip (e.g., `/proxy`)

2. **`func New(targetURL string) *Proxy`**
   - Create proxy instance
   - Configure HTTP client with 10s timeout
   - Set default target prefix to `/proxy`

3. **`func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request)`**
   - Main HTTP handler
   - Extract client IP
   - Log request: `[PROXY] timestamp method path -> status (duration)`
   - Forward request to upstream
   - Copy response headers
   - Stream response body
   - Log response status

4. **`func (p *Proxy) forwardRequest(r *http.Request) (*http.Request, error)`**
   - Create new request to upstream URL
   - Strip `/proxy` prefix from path if present
   - Copy all headers except `Host`
   - Set `X-Forwarded-For` header with client IP

5. **`func (p *Proxy) copyResponse(w http.ResponseWriter, resp *http.Response) error`**
   - Copy response headers
   - Copy status code
   - Stream response body to client
   - Handle errors

6. **`func getClientIP(r *http.Request) string`**
   - Extract from `X-Forwarded-For` header
   - Fallback to `X-Real-IP` header
   - Fallback to `RemoteAddr`

**Unit Tests:** `internal/proxy/proxy_test.go`

**Test Cases:**
1. `TestProxyNew` - Verify proxy creation
2. `TestProxyForwardRequest` - Verify request forwarding logic
3. `TestProxyStripPrefix` - Verify `/proxy` prefix stripping
4. `TestProxyCopyHeaders` - Verify header copying (except Host)
5. `TestProxyGetClientIP` - Verify client IP extraction from headers
6. `TestProxyServeHTTP` - Integration test with mock server
7. `TestProxyTimeout` - Verify timeout handling

---

### Phase 4: Main Entry Point (Integration Tests)

**File:** `cmd/overwhellm/main.go`

**Features:**
- Parse environment variables with defaults
- Initialize proxy with upstream URL
- Create HTTP mux (all paths → proxy handler)
- Create HTTP server with 10s timeouts
- Start listening on configured port
- Log startup info
- No signal handling (keep simple)

**Startup Log:**
```
🚀 overwhellm starting...
   Listen: :8080
   Upstream: http://aspec.localdomain:12434
   Timeout: 10s
```

**Integration Tests:** `cmd/overwhellm/main_test.go`

**Test Cases:**
1. `TestMainStartup` - Verify server starts
2. `TestEnvironmentVariables` - Verify env var parsing
3. `TestProxyEndpoint` - Full integration test with mock server

---

### Phase 5: End-to-End Tests

**File:** `e2e/e2e_test.go`

**Test Scenario:**
1. Start mock server on port 9090
2. Start proxy on port 8080
3. Send request to proxy (simulate opencode)
4. Verify response matches mock response
5. Verify request logged correctly
6. Stop servers

**Test Cases:**
1. `TestE2EBasicFlow` - Request → Proxy → Mock → Proxy → Response
2. `TestE2EHeadersPreserved` - Verify all headers passed through
3. `TestE2EClientIP` - Verify client IP extraction

---

## Testing Strategy

### Unit Tests
- Test proxy functions in isolation
- Use `httptest` server for upstream simulation
- Fast execution (< 1 second per test)
- Minimum coverage: 50% (to be improved later)

### Integration Tests
- Start actual HTTP server
- Test full request/response cycle
- Verify environment variable handling

### E2E Tests
- Full system test
- Simulate real client behavior
- Verify transparency

---

## Build Commands

```bash
# Build mock server
go build -o mock-llm ./cmd/mock-llm

# Build proxy
go build -o overwhellm ./cmd/overwhellm

# Run tests
go test ./...
go test -v ./...

# Run mock server
./mock-llm

# Run proxy
./overwhellm
```

---

## Usage with opencode

Configure opencode API settings:
- **Base URL**: `http://localhost:8080`
- **Endpoint**: `/v1/chat/completions`
- **Streaming**: Enable for chat completions

**Traffic flow:**
```
opencode → localhost:8080 → forward → aspec.localdomain:12434 → forward → opencode
```

---

## Key Design Decisions

1. **Minimal dependencies**: Only Go standard library
2. **Streaming**: Byte-by-byte forwarding for efficiency
3. **Logging**: Simple stdout format with timestamps `[PROXY] timestamp`
4. **Timeouts**: 10-second client and server timeouts
5. **No health checks**: Simple pass-through behavior
6. **No modifications**: Transparent forwarding, no API format changes
7. **No graceful shutdown**: Keep implementation simple
8. **Mock server for testing**: Enable testing without live LLM backend
9. **50% minimum test coverage**: Will be improved in future iterations

---

## Implementation Order

1. **Setup Phase**
   - Create `go.mod`
   - Create `.env` files
   - Verify module builds

2. **Mock Server Phase**
   - Create `cmd/mock-llm/main.go`
   - Write unit tests
   - Test manually with curl

3. **Proxy Core Phase**
   - Create `internal/proxy/proxy.go`
   - Write unit tests
   - Run `go test ./internal/proxy`

4. **Main Entry Phase**
   - Create `cmd/overwhellm/main.go`
   - Write integration tests
   - Run `go test ./cmd/overwhellm`

5. **End-to-End Phase**
   - Create E2E tests
   - Run full system test
   - Verify with opencode client

---

## Verification Checklist

- [ ] Mock server runs and responds correctly
- [ ] Proxy forwards requests to upstream
- [ ] Proxy preserves headers correctly
- [ ] Proxy strips `/proxy` prefix
- [ ] Proxy logs requests to stdout
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] E2E tests pass
- [ ] Test coverage >= 50%
- [ ] Manual testing with curl works
- [ ] Integration with opencode works

---

## Future Enhancements (Out of Scope)

- Graceful shutdown on SIGINT/SIGTERM
- Health check endpoint (`/health`)
- Token counting and metrics
- Retry logic for failed requests
- Multiple upstream servers with load balancing
- Authentication and authorization
- Rate limiting
- Response caching
- Enhanced logging (file output, log rotation)
- Higher test coverage (> 50%)
- Streaming support for chat completions
- Support for more OpenAI API endpoints