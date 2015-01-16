all: pack-ami

pack-ami: test build
	packer build packer.json

deps:
	go get -v ./...

test: deps
	go test -v bastion/...

build: deps
	go build cmd/bastion/main.go -o bastion