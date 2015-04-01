all: test

ami: test build
	@packer build -machine-readable -parallel=true packer.json | tee packer.log

cloudformation: pack-ami
	@godep run packer_to_cloudformation/packer_to_cloudformation.go -packer_log packer.log -cloudform cloudformation.json > bastion-cf.template

deps:
	@godep restore

test: deps
	godep go test -v ./...

build: deps
	@godep go build  -a -p=4 -v -x  -o cookbooks/bastion/files/default/bastion

clean: 
	@go clean -x -i -r ./...