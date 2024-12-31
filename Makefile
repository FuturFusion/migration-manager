GO ?= go
DETECTED_LIBNBD_VERSION = $(shell dpkg-query --showformat='$${Version}' -W libnbd-dev || echo "0.0.0-libnbd-not-found")

default: build

.PHONY: build
build: build-dependencies migration-manager
	go build -o ./bin/migration-managerd ./cmd/migration-managerd
	go build -o ./bin/migration-manager-worker ./cmd/migration-manager-worker

.PHONY: build-dependencies
build-dependencies:
	@if ! dpkg --compare-versions 1.20 "<=" ${DETECTED_LIBNBD_VERSION}; then \
		echo "Please install libnbd-dev with version >= 1.20"; \
		exit 1; \
	fi

.PHONY: migration-manager
migration-manager:
	mkdir -p ./bin/
	CGO_ENABLED=0 GOARCH=amd64 go build -o ./bin/migration-manager.linux.amd64 ./cmd/migration-manager
	CGO_ENABLED=0 GOARCH=arm64 go build -o ./bin/migration-manager.linux.arm64 ./cmd/migration-manager
	GOOS=darwin GOARCH=amd64 go build -o ./bin/migration-manager.macos.amd64 ./cmd/migration-manager
	GOOS=darwin GOARCH=arm64 go build -o ./bin/migration-manager.macos.arm64 ./cmd/migration-manager
	GOOS=windows GOARCH=amd64 go build -o ./bin/migration-manager.windows.amd64.exe ./cmd/migration-manager
	GOOS=windows GOARCH=arm64 go build -o ./bin/migration-manager.windows.arm64.exe ./cmd/migration-manager

.PHONY: test
test: build-dependencies
	go test ./... -v -cover

.PHONY: static-analysis
static-analysis: build-dependencies
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
	go-licenses check --disallowed_types=forbidden,unknown,restricted --ignore libguestfs.org/libnbd ./...
	shellcheck --shell sh internal/worker/scripts/*.sh
	golangci-lint run ./...
	run-parts $(shell run-parts -V >/dev/null 2>&1 && echo -n "--verbose --exit-on-error --regex '.sh'") scripts/lint

.PHONY: clean
clean:
	rm -rf dist/ bin/

.PHONY: release-snapshot
release-snapshot:
ifeq ($(shell command -v goreleaser),)
	echo "Please install goreleaser"
	exit 1
endif
	goreleaser release --snapshot --clean
