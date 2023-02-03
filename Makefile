export GOBIN ?= $(shell pwd)/bin

GOLINT = $(GOBIN)/golangci-lint

GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

.PHONY: build
build:
	go build ./...

.PHONY: install
install:
	go mod download

.PHONY: test
test:
	go test -v -race ./...
	go test -v -trace=/dev/null .

.PHONY: cover
cover:
	go test -race -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html

# Note that installation via "go install" is not recommended
# (https://golangci-lint.run/usage/install/#install-from-source).
# If this causes problems, install a pre-built binary.
#
# When bumping the version here, then also bump the version in
# .github/workflows/golangci-lint.yml
$(GOLINT):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.0

.PHONY: lint
lint: $(GOLINT)
	@echo "Checking lint..."
	@$(GOLINT) run ./...
