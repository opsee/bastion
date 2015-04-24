all: install

pack-ami: test
	@packer build -debug -machine-readable -parallel=true  build/packer.json | tee packer.log

cloudformation: pack-ami
	@godep go run build/packer_to_cloudformation.go -packer_log packer.log -cloudform build/cloudformation.json > bastion-cf.template

deps:
	@go get github.com/tools/godep

test: build
	@godep go test -v ./...

build: build-bastion build-protocheck

build-bastion: deps
	@godep go build -v -x  -o cookbooks/bastion/files/default/bastion  ./cmd/bastion

build-protocheck: deps
	@godep go build -v -x  cmd/protocheck/main.go

install: install-bastion install-protocheck

install-bastion: build-bastion
	@godep go install -v -x ./cmd/bastion

install-protocheck: build-protocheck
	@godep go install -v -x ./cmd/protocheck

clean:	clean-protocheck clean-bastion

clean-protocheck:
	@godep go clean -r -x -i -x ./cmd/protocheck

clean-bastion:
	@godep go clean -r -x -i -x ./cmd/protocheck




