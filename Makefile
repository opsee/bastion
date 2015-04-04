all: cookbook-files

cookbook-files: cloudformation
	@cp -v  out/* cookbooks/bastion/files/default/

pack-ami: test
	@packer build -debug -machine-readable -parallel=true build/packer.json | tee packer.log

cloudformation: pack-ami
	@godep go run build/packer_to_cloudformation.go -packer_log packer.log -cloudform build/cloudformation.json > bastion-cf.template

deps:
	@go get github.com/tools/godep

test: build
	@godep go test -v ./...

build: deps out
	@godep go build -p=4 -v -x  -o cookbooks/bastion/files/default/bastion  cmd/bastion/main.go

out:
	mkdir -v out

clean: 
	@godep go clean -x -i -r ./...
	@rm -v -f cookbooks/bastion/files/default/bastion
	@rm -v -f cookbooks/bastion/files/default/bastion-cf.template
	@rm -vrf out

