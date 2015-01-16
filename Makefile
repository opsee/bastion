all: pack-ami

pack-ami: test build
	packer build packer.json

deps:
	go get ./...

test: deps
	go test ./...

build: deps
	go build cmd/bastion/main.go -o bastion