default: build

build:
	go build ./cmd/migration-manager
	go build ./cmd/migration-managerd
	go build ./cmd/migration-manager-worker

clean:
	rm -f migration-manager migration-managerd migration-manager-worker
