#!/bin/bash

# Overwhellm Deployment Script

set -e

# Configuration
PROXY_PORT=${PROXY_PORT:-9000}
LLAMA_URL=${LLAMA_URL:-http://localhost:8080}
DB_PATH=${DB_PATH:-./overwhellm.db}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Starting Overwhellm Dashboard${NC}"
echo ""
echo "Configuration:"
echo "  Proxy Port:  $PROXY_PORT"
echo "  Llama URL:   $LLAMA_URL"
echo "  Database:    $DB_PATH"
echo ""

# Check if llama.cpp is running
echo -e "${YELLOW}🔍 Checking llama.cpp server...${NC}"
if curl -s --connect-timeout 2 "$LLAMA_URL/v1/models" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ llama.cpp server is running${NC}"
else
    echo -e "${RED}⚠️  llama.cpp server is not responding at $LLAMA_URL${NC}"
    echo "   You can still start the proxy, but requests will fail until llama.cpp is available"
fi

# Start the proxy
echo ""
echo -e "${GREEN}📦 Starting proxy...${NC}"
exec ./overwhellm --port "$PROXY_PORT" --llama-url "$LLAMA_URL" --db "$DB_PATH"
