# LLM Monitoring Proxy - Implementation Plan

## Overview

A high-performance monitoring proxy that sits between clients (e.g., opencode) and llama.cpp backend, tracking token usage, latency, and providing metrics for robustness and stability analysis.

## Architecture

```
Client → Go Proxy → llama.cpp
         ↓
    SQLite Database
```

## Technology Stack

| Component | Choice | Why |
|-----------|--------|-----|
| HTTP Server | **Gin** | Fast, production-ready, minimal dependencies |
| Database | **SQLite** | Single file, no ops, perfect for 10 req/min |
| Token Counting | **tiktoken-go** | Accurate, upgradeable later |
| Dashboard | **Chart.js + Go templates** | Interactive charts, no external dependencies |

## Requirements

- **Load**: ~10 requests/minute (low load)
- **Retention**: Indefinite
- **Multi-user**: Yes (identified by client IP)
- **Authentication**: No auth required
- **Caching**: No caching
- **Deployment**: Single binary or Docker

## Features

### Core Proxy
- Forward all OpenAI-compatible API requests to llama.cpp
- Capture request/response metadata
- Measure latency (total + time-to-first-token for streaming)
- Track token usage (input/output)

### Metrics Tracked
- Request count (total, by endpoint, by model, by client)
- Latency metrics (p50, p95, p99, average)
- Token throughput (tokens/second)
- Token usage (input/output totals)
- Error rates by status code

### Web Dashboard
- Summary cards (today/week/month stats)
- Time series charts (requests, latency, tokens)
- Recent requests table with filtering
- Detailed request view

## Project Structure

```
overwhellm/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, server setup
├── internal/
│   ├── db/
│   │   └── db.go            # SQLite operations
│   ├── proxy/
│   │   └── proxy.go         # Request forwarding logic
│   └── ui/
│       ├── dashboard.go     # HTTP handlers
│       ├── static/
│       │   ├── index.html   # Dashboard UI
│       │   ├── styles.css   # Styling
│       │   └── app.js       # Charts & interactivity
│       └── templates/
│           └── metrics.html # HTML template
├── pkg/
│   └── token/
│       └── counter.go       # Token counting
├── deployments/
│   ├── docker-compose.yml   # Deployment config
│   └── Dockerfile           # Container build
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

## Database Schema

```sql
CREATE TABLE requests (
    id TEXT PRIMARY KEY,              -- UUID
    client_ip TEXT NOT NULL,          -- Client IP address
    user_agent TEXT,                  -- Optional: user agent
    endpoint TEXT NOT NULL,           -- e.g., /v1/chat/completions
    model TEXT,                       -- Requested model name
    method TEXT DEFAULT 'POST',       -- HTTP method
    status_code INTEGER,              -- Response status
    duration_ms INTEGER,              -- Total duration in ms
    ttft_ms INTEGER,                  -- Time to first token (streaming only)
    tokens_input INTEGER,             -- Input tokens
    tokens_output INTEGER,            -- Output tokens
    request_size_bytes INTEGER,       -- Request body size
    response_size_bytes INTEGER,      -- Response body size
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_created_at ON requests(created_at);
CREATE INDEX idx_client_ip ON requests(client_ip);
CREATE INDEX idx_model ON requests(model);
CREATE INDEX idx_status_code ON requests(status_code);
```

## Configuration (.env)

```bash
# Server configuration
PROXY_PORT=9000
LLAMA_CPP_URL=http://localhost:8080

# Database
DB_PATH=./overwhellm.db

# Optional: UI port (if separate from proxy)
UI_PORT=9001
```

## API Endpoints

### Proxy Endpoints (forward to llama.cpp)
- `POST /v1/chat/completions` - Chat completion (streaming/non-streaming)
- `GET /v1/models` - List available models
- Any other OpenAI-compatible endpoint

### Dashboard Endpoints
- `GET /` - Main dashboard page
- `GET /api/metrics/summary` - Summary stats (today/week/month)
- `GET /api/metrics/trends` - Time series data
- `GET /api/requests` - Paginated list of requests
- `GET /api/requests/:id` - Single request details

## Go Dependencies

```go
module overwhellm

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/mattn/go-sqlite3 v1.14.19
    github.com/pkoukk/tiktoken-go v0.1.5
    github.com/google/uuid v1.4.0
)
```

## Implementation Phases

### Phase 1: Core Proxy (Day 1)
- [ ] Initialize Go module
- [ ] Create directory structure
- [ ] Set up Gin router with middleware
- [ ] Configure SQLite connection
- [ ] Implement proxy middleware
- [ ] Handle streaming responses (SSE)
- [ ] Measure timing metrics
- [ ] Forward to llama.cpp

### Phase 2: Token Counter (Day 2)
- [ ] Implement tiktoken-based counter
- [ ] Extract tokens from request/response
- [ ] Handle different message formats
- [ ] Cache tokenizer per model

### Phase 3: Database Layer (Day 2)
- [ ] Create SQLite tables
- [ ] Implement insert/query functions
- [ ] Add error handling
- [ ] Optimize queries

### Phase 4: Dashboard UI (Day 3-4)
- [ ] Design HTML layout
- [ ] Implement Chart.js visualizations
- [ ] Add date range filtering
- [ ] Create API endpoints
- [ ] Add sortable tables

### Phase 5: Testing & Polish (Day 4-5)
- [ ] Test with llama.cpp
- [ ] Verify token counting accuracy
- [ ] Optimize queries
- [ ] Add error handling
- [ ] Write documentation

## Key Design Decisions

### 1. Single Binary
- Proxy and UI served from same binary
- Simplifies deployment
- Can split later if needed

### 2. Token Counting
- Use tiktoken for accuracy
- Adds ~1-2ms per request (acceptable)
- Can optimize later if needed

### 3. SQLite
- WAL mode for better concurrency
- Indefinite retention (no auto-cleanup)
- Estimated size: ~10-20MB after 3 months

### 4. Streaming Support
- Support both streaming and non-streaming
- Better for TTFT measurement
- Buffer response chunks for token counting

## Potential Challenges

### Challenge 1: Token Counting on Streaming
**Problem**: Can't count output tokens until stream completes
**Solution**: Buffer response chunks, count at end, store TTFT separately

### Challenge 2: SQLite Concurrency
**Problem**: Multiple requests writing simultaneously
**Solution**: SQLite handles this well, use WAL mode

### Challenge 3: Large Response Bodies
**Problem**: Storing full request/response in DB
**Solution**: Don't store bodies, only metadata (tokens, sizes, timestamps)

## Testing Strategy

1. **Unit Tests**: Token counter accuracy
2. **Integration Tests**: Proxy forwarding to mock llama.cpp
3. **Load Testing**: Verify performance at 10+ req/min
4. **UI Tests**: Chart rendering, filtering works

## Deployment

### Docker Compose (Recommended)

```yaml
version: '3.8'
services:
  overwhellm:
    build: .
    ports:
      - "9000:9000"
    environment:
      - LLAMA_CPP_URL=http://llama-server:8080
    depends_on:
      - llama-server

  llama-server:
    image: ghcr.io/ggerganov/llama.cpp:server
    command: -m model.gguf --port 8080
    ports:
      - "8080:8080"
```

### Single Binary

```bash
go build -o overwhellm cmd/server/main.go
./overwhellm --port 9000 --llama-url http://localhost:8080
```

## Success Criteria

- [ ] All requests logged to SQLite
- [ ] Token usage tracking (input/output)
- [ ] Latency metrics (total + TTFT)
- [ ] Interactive web dashboard
- [ ] No external dependencies (just SQLite)
- [ ] Easy to query data with SQLite CLI
- [ ] Can add Prometheus later if needed

## Future Enhancements

- Prometheus/Grafana integration
- Real-time alerts for latency spikes
- Request caching for repeated queries
- Authentication/API keys
- Multi-model routing
- Cost tracking with per-token pricing
- Prompt versioning
- Session tracking for multi-turn conversations
