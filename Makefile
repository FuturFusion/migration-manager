default: build

build:
	go build ./cmd/import-vm-metadata
	go build ./cmd/import-disks

clean:
	rm -f import-disks import-vm-metadata
