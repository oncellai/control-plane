.PHONY: build test lint clean

build:
	go build ./...

test:
	@echo "=== Control Plane Tests ==="
	go test ./... -v

lint:
	go vet ./...

clean:
	go clean
