SOURCE_FILES?=./...
TEST_PATTERN?=.
TEST_OPTIONS?=
OS=$(shell uname -s)
LDFLAGS=-ldflags "-X main.version=$(shell git describe --tags --always --dirty || echo dev) -X main.commit=$(shell git rev-parse HEAD || echo none)"

export PATH := ./bin:$(PATH)
export GO111MODULE := on
# enable consistent Go 1.12/1.13 GOPROXY behavior.
export GOPROXY = https://proxy.golang.org

bin/goreleaser:
	mkdir -p bin
	GOBIN=$(shell pwd)/bin go install github.com/goreleaser/goreleaser/v2@latest

bin/golangci-lint:
	mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b ./bin v2.1.2

bin/shellcheck:
	mkdir -p bin
ifeq ($(OS), Darwin)
	curl -sfL -o ./bin/shellcheck https://github.com/caarlos0/shellcheck-docker/releases/download/v0.4.6/shellcheck_darwin
else
	curl -sfL -o ./bin/shellcheck https://github.com/caarlos0/shellcheck-docker/releases/download/v0.4.6/shellcheck
endif
	chmod +x ./bin/shellcheck

setup: bin/golangci-lint bin/shellcheck ## Install all the build and lint dependencies
	go mod download
.PHONY: setup

install: ## build and install
	go install $(LDFLAGS) ./cmd/binst

test: ## Run all the tests
	go test $(TEST_OPTIONS) -failfast -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.txt $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=2m

cover: test ## Run all the tests and opens the coverage report
	go tool cover -html=coverage.txt

fmt: ## gofmt and goimports all go files
	find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do gofmt -w -s "$$file"; goimports -w "$$file"; done

lint: bin/golangci-lint ## Run all the linters
	./bin/golangci-lint run ./... --disable errcheck

ci: build test lint ## travis-ci entrypoint
	git diff .

build: ## Build a beta version of binstaller
	go build $(LDFLAGS) ./cmd/binst

.DEFAULT_GOAL := build

.PHONY: ci help clean

clean: ## clean up everything
	go clean ./...
	rm -f binstaller
	rm -rf ./bin ./dist ./vendor
	git gc --aggressive

# Absolutely awesome: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
