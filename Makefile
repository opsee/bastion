all: pack-ami

pack-ami: test build
	packer build packer.json

deps:
	go get -v -t ./...

test: deps
	go test -v bastion/...

build: deps
	go build -o cookbooks/bastion/files/default/bastion cmd/bastion/main.go