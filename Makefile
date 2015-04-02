all: cloudformation

pack-ami: test
	@packer build -machine-readable -parallel=true build/packer.json | tee build/packer.log

cloudformation: pack-ami
	@godep go run build/packer_to_cloudformation/packer_to_cloudformation.go -packer_log build/packer.log -cloudform build/cloudformation.json > build/bastion-cf.template

dl-godep: 
	@go get github.com/tools/godep

deps: dl-godep
	@godep restore

test: build
	@godep go test -v ./...

build: deps
	@godep go build -p=4 -v -x  -o cookbooks/bastion/files/default/bastion

clean: 
	@go clean -x -i -r ./...