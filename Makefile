.PHONY: all tools generate build test vet integration tidy clean connectors

OAPI_VERSION ?= latest

all: build test

# Install the code generator into ./bin
tools:
	GOBIN=$(CURDIR)/bin go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_VERSION)

# Regenerate psd2/ clients from specs/
generate:
	./scripts/generate.sh

build:
	go build ./...

# Build the connector binaries into ./bin
connectors:
	go build -o bin/rbhu-connector ./cmd/rbhu-connector
	go build -o bin/rbhu-connector-http ./cmd/rbhu-connector-http

vet:
	go vet ./...

# Unit tests (offline, default)
test:
	go test ./...

# Live sandbox tests (see integration_test.go for required env)
integration:
	go test -tags integration -run TestIntegration -v ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
