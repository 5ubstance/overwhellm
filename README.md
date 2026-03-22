# Overwhellm - Transparent HTTP Proxy for LLM Sessions

A transparent HTTP proxy that forwards traffic from opencode sessions to a remote LLM running on `aspec:12434`. The proxy supports streaming responses (SSE format) so users see tokens as they're generated.

## Features

- **Transparent forwarding**: Forwards requests from `localhost:8080` to upstream LLM
- **Streaming support**: Detects streaming from query params or request body
- **SSE format**: Delivers streaming responses in Server-Sent Events format
- **Detailed logging**: Debug-friendly request/response logging
- **Configurable timeout**: 120-second timeout for upstream connections

## Quick Start

### Prerequisites

- Go 1.21+
- opencode client configured to use proxy

### Running the Proxy

1. **Start the mock LLM server** (for testing):
   ```bash
   ./mock-llm
   ```

2. **Start the proxy**:
   ```bash
   ./overwhellm
   ```

3. **Configure opencode** to use the proxy:
   - Set the proxy URL in opencode configuration
   - Requests will be forwarded through `localhost:8080`

### Configuration

#### Proxy (`.env`)

```bash
# Proxy configuration
PROXY_PORT=8080
UPSTREAM_URL=http://aspec:12434
TIMEOUT_SECONDS=120
LOG_LEVEL=info
```

#### Mock LLM (`.env.mock`)

```bash
# Mock server configuration
MOCK_PORT=12434
MOCK_RESPONSE_DELAY_MS=50
```

## Usage

### Request Flow

```
opencode client
       ↓
localhost:8080 (proxy)
       ↓
aspec:12434 (upstream LLM)
```

### Streaming Detection

The proxy automatically detects streaming requests from:

1. **Query parameter**: `?stream=true`
2. **Request body**: `"stream": true` in JSON payload

### Logging

Enable detailed logging with:

```bash
./overwhellm -timeout=120
```

Logs show:
- Request metadata (method, path, client IP)
- Forwarding details
- Response status and timing
- Streaming progress

## Testing

### Run unit tests

```bash
go test ./...
```

### Run e2e tests

```bash
go test ./e2e -v
```

### Test with mock server

1. Start mock server:
   ```bash
   ./mock-llm
   ```

2. Send test request:
   ```bash
   curl -X POST http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "test",
       "messages": [{"role": "user", "content": "Hello"}],
       "stream": true
     }'
   ```

### Build binaries

```bash
# Build proxy
CGO_ENABLED=0 go build -o overwhellm ./cmd/overwhellm

# Build mock server
CGO_ENABLED=0 go build -o mock-llm ./cmd/mock-llm
```

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ opencode    │────▶│ proxy:8080   │────▶│ aspec:12434 │
│ client      │     │              │     │ LLM         │
└─────────────┘     └──────────────┘     └─────────────┘
                      ↓
                   Streaming
                   (SSE)
```

## Troubleshooting

### Common Issues

1. **"invalid Read on closed Body" error**
   - Fixed in latest version: Body bytes are now preserved during streaming detection

2. **Connection timeout**
   - Check upstream LLM is running on `aspec:12434`
   - Verify network connectivity
   - Increase timeout with `-timeout` flag

3. **Streaming not working**
   - Verify request includes `"stream": true` or `?stream=true`
   - Check logs for streaming detection messages

### Debug Mode

Enable verbose logging:
```bash
export LOG_LEVEL=debug
./overwhellm
```

## License

MIT