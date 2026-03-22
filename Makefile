.PHONY: build run test clean

build:
	CGO_ENABLED=0 go build -o overwhellm ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

clean:
	rm -f overwhellm
	rm -f overwhellm.db
	rm -f overwhellm.db-wal
	rm -f overwhellm.db-shm

run-dev:
	go run ./cmd/server --port 9000 --llama-url http://localhost:8080 --db ./overwhellm.db
