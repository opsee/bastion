all: cloudformation

pack-ami: test
	@packer build -debug -machine-readable -parallel=true build/packer.json | tee packer.log

cloudformation: pack-ami
	@godep go run build/packer_to_cloudformation.go -packer_log packer.log -cloudform build/cloudformation.json > bastion-cf.template

deps:
	@go get github.com/tools/godep

test: build
	@godep go test -v ./...

build: deps
	@godep go build -p=4 -v -x  -o cookbooks/bastion/files/default/bastion  cmd/bastion/main.go

clean:
	@godep go clean -a -r -i -x ./...



