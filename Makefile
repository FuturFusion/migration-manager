default: build

build:
	go build ./cmd/migration-manager
	go build ./cmd/migration-manager-agent

clean:
	rm -f migration-manager migration-manager-agent
