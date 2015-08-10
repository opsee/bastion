PROTO_DIR=./proto

all: protoc fmt build

build:
	gb build

clean:
	rm -fr target bin

protoc: $(PROTO_DIR)/bastion.proto
	protoc -I/usr/local/include -I$(PROTO_DIR) --go_out=plugins=grpc:$(PROTO_DIR) $(PROTO_DIR)/bastion.proto 

fmt:
	@gofmt -w ./

.PHONY: clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)

