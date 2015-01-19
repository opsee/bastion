all: build-cloudformation

pack-ami: test build
	packer build -machine-readable -parallel=true packer.json > packer.log

build-cloudformation: pack-ami
	go packer_to_cloudformation.go -packer_log packer.log -cloudform cloudformation.json > mapped_cloudformation.json

deps:
	go get -v -t ./...

test: deps
	go test -v bastion/...

build: deps
	go build -o cookbooks/bastion/files/default/bastion cmd/bastion/main.go