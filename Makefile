.PHONY: all tools generate build test vet integration tidy clean

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
