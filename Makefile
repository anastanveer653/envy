VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test cover lint audit snapshot clean

build: ## Build ./envy
	go build -trimpath -ldflags "$(LDFLAGS)" -o envy .

test: ## Run tests with the race detector
	go test -race ./...

cover: ## Write coverage.html
	go test -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

lint: ## Run golangci-lint
	golangci-lint run

audit: ## Scan this repository with envy itself
	go run . audit

snapshot: ## Build release archives locally without publishing
	goreleaser release --snapshot --clean

clean:
	rm -rf envy envy.exe dist coverage.out coverage.html
