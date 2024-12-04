GO ?= go

default: build

.PHONY: build
build:
	go build ./cmd/migration-manager
	go build ./cmd/migration-managerd
	go build ./cmd/migration-manager-worker

.PHONY: test
test:
	go test ./... -v -cover

.PHONY: static-analysis
static-analysis:
ifeq ($(shell command -v go-licenses),)
	(cd / ; $(GO) install -v -x github.com/google/go-licenses@latest)
endif
ifeq ($(shell command -v golangci-lint),)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
endif
ifeq ($(shell command -v shellcheck),)
	echo "Please install shellcheck"
	exit 1
endif
	go-licenses check ./...
	shellcheck --shell sh internal/worker/scripts/*.sh
	golangci-lint run ./...
	run-parts $(shell run-parts -V >/dev/null 2>&1 && echo -n "--verbose --exit-on-error --regex '.sh'") scripts/lint

.PHONY: clean
clean:
	rm -f migration-manager migration-managerd migration-manager-worker
