default: build

build:
	go build ./cmd/import-vm-metadata
	go build ./cmd/import-disks
	go build ./cmd/inject-drivers

clean:
	rm -f import-disks import-vm-metadata inject-drivers
