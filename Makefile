all: cookbook-files

cookbook-files: cloudformation
	@cp  out/bastion out/bastion-cf.template  cookbooks/bastion/files/default/

pack-ami: test
	@packer build -machine-readable -parallel=true build/packer.json | tee out/packer.log

cloudformation: pack-ami
	@godep go run build/packer_to_cloudformation.go -packer_log out/packer.log -cloudform build/cloudformation.json > out/bastion-cf.template

deps:
	@godep get github.com/tools/godep

test: build
	@godep go test -v ./...

build: deps out
	@godep go build -p=4 -v -x  -o out/bastion  cmd/bastion/main.go

out:
	mkdir out

clean: 
	@godep go clean -x -i -r ./...
	@rm -f cookbooks/bastion/files.default/bastion
	@rm -f cookbooks/bastion/files.default/bastion-cf.template
	@rm -rf out

