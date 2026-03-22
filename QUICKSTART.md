# Quick Start Guide

## 1. Start llama.cpp Server

Make sure your llama.cpp server is running:

```bash
# Example: Start llama.cpp server on port 8080
llama-server -m model.gguf --port 8080
```

## 2. Start the Proxy

```bash
# Run with defaults (proxy on port 9000, llama.cpp on localhost:8080)
./overwhellm

# Or with custom ports
./overwhellm --port 9000 --llama-url http://localhost:8080 --db ./overwhellm.db
```

## 3. Configure Your Client

Point your client (opencode, etc.) to the proxy:

```json
{
  "base_url": "http://localhost:9000/proxy",
  "api_key": "any-value"
}
```

**Important**: All LLM API requests should use the `/proxy/v1/...` endpoints. The dashboard is at `http://localhost:9000/`.

## 4. Access the Dashboard

Open your browser to: **http://localhost:9000**

You'll see:
- Summary statistics (total requests, tokens, latency)
- Charts showing request trends
- Recent requests table

## 5. Query the Database

The database is stored as a JSON file (`overwhellm.db`):

```bash
# View all requests
cat overwhellm.db | jq '.'

# Count requests
cat overwhellm.db | jq 'length'

# Get today's stats
jq '[.[] | select(.created_at | startswith(now | strftime("%Y-%m-%d")))] | length' overwhellm.db
```

## Example Usage

```bash
# Make a request through the proxy (note the /proxy prefix)
curl http://localhost:9000/proxy/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": false
  }'

# Check that the request was logged
cat overwhellm.db | jq '.[-1]'
```

## Docker Deployment

```bash
cd deployments
docker-compose up -d

# Access dashboard
open http://localhost:9000
```

## Troubleshooting

### Proxy won't start
- Check port 9000 is not in use: `lsof -i :9000`
- Check llama.cpp is running: `curl http://localhost:8080/v1/models`

### No requests appearing in dashboard
- Check proxy logs
- Verify client is pointing to http://localhost:9000
- Check llama.cpp URL is correct

### Database file too large
- The JSON file grows with each request
- For large deployments, consider periodic cleanup:
  ```bash
  jq '[.[] | select(.created_at > (now | strftime("%Y-%m-%d")))]' overwhellm.db > temp.db && mv temp.db overwhellm.db
  ```
