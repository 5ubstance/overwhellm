# Agent Guidelines for overwhellm

## Build, Lint, and Test Commands

**Build:**
```bash
CGO_ENABLED=0 go build -o overwhellm ./cmd/overwhellm
```

**Run:**
```bash
./overwhellm --port=8080 --upstream-url=http://myhost.mydomain:myport
# Or with config.json and .env support
./overwhellm
```

**Test:**
```bash
go test ./...
# Run a single test file:
go test ./internal/proxy -v
# Run a single test function:
go test -run TestParseStreamingTokenUsage ./internal/proxy -v
```

**Mock LLM Server:**
```bash
CGO_ENABLED=0 go build -o mock-llm ./cmd/mock-llm
./mock-llm --port=12434
```

## Code Style Guidelines

### Imports
- Use standard library imports first, then third-party, then internal packages
- Group imports with blank lines between categories
- Internal imports use the module path: `"overwhellm/internal/proxy"`

### Naming Conventions
- **Package names**: lowercase, short (e.g., `proxy`, `config`)
- **Types**: PascalCase (e.g., `Proxy`, `TokenStats`, `TokenTrackingReader`)
- **Functions**: camelCase for exported (e.g., `New`, `ServeHTTP`), lowercase for internal (e.g., `debug`, `errorf`)
- **Variables**: camelCase, descriptive (e.g., `finalPort`, `targetURL`, `chunkDuration`)
- **Constants**: PascalCase for exported (e.g., `LevelDebug`), lowercase for internal (e.g., `colorReset`)

### Types and Structs
- Use meaningful struct tags for JSON: `` `json:"field_name"` ``
- Prefer explicit types over `interface{}`; use `map[string]interface{}` for JSON parsing
- Define named types for common patterns (e.g., `LogLevel int`)

### Error Handling
- Check errors immediately after operations
- Return errors from functions; use `log.Fatalf` only in `main()` for fatal conditions
- Use descriptive error messages: `fmt.Errorf("invalid config.json: %v", err)`
- Handle `io.EOF` explicitly when reading streams

### Logging
- Use the custom logger: `trace()`, `debug()`, `info()`, `warn()`, `errorf()`, `critical()`
- All log lines include RFC3339 timestamps
- Log levels: TRACE < DEBUG < INFO < WARN < ERROR < CRITICAL
- Default log level: INFO
- Log to both stdout and file (configurable via `--log-file`)
- Use ANSI color codes for terminal output; colors reset at end of line
- Syntax-highlight JSON at DEBUG level (blue keys, green strings, cyan braces, yellow colons, magenta booleans, red null, bold numbers)

### Concurrency
- Use `sync.Mutex` for shared state (e.g., `TokenTrackingReader.mu`)
- Always unlock with `defer` when possible
- Use channels for signaling completion (`done chan struct{}`)
- Lock before accessing shared variables, unlock after

### HTTP and Networking
- Use `http.Client` with configurable timeouts
- Always `defer resp.Body.Close()` after receiving responses
- Copy headers from upstream to client response
- Set `X-Forwarded-For` header with client IP
- Handle streaming responses with `http.Flusher`

### Configuration Priority
1. CLI flags (highest priority)
2. config.json
3. .env file
4. Default values (lowest priority)

### Formatting
- Use `gofmt` / `go fmt` (standard Go formatting)
- Keep lines under 100 characters when possible
- Use blank lines to separate logical sections
- No trailing whitespace

### Documentation
- Exported types and functions should have package-level comments
- Complex logic should have inline comments explaining the "why"
- Log messages should be clear and actionable
