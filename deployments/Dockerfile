FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 go build -o overwhellm ./cmd/server

# Final image
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/overwhellm .

# Expose port
EXPOSE 9000

# Default command
CMD ["./overwhellm", "--port", "9000", "--llama-url", "http://localhost:8080", "--db", "/app/overwhellm.db"]
