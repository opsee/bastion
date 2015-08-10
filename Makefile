all: protoc fmt test build

build:
	gb build

clean:
	rm -fr target bin

protoc:
	protoc -I/usr/local/include -Iproto proto/bastion.proto --go_out=plugins=grpc:proto

fmt:
	@gofmt -w ./

.PHONY: clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)

