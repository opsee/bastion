PREFIX=/usr/local
DESTDIR=
GOFLAGS=-v
BINDIR=${PREFIX}/bin

SRCS = $(wildcard *.go)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

CMDS = $(notdir $(wildcard cmd/*))
BLDDIR = target/${GOOS}

all: deps protoc fmt test $(CMDS)

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	@godep go build -ldflags '-w' -tags netgo ${GOFLAGS} -o $(abspath $@) ./$*

$(BINARIES): $: $(BLDDIR)/%
$(CMDS): %: $(BLDDIR)/cmd/% $(SRCS)

clean:
	rm -fr target

.PHONY: install clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)

install: $(BINARIES)
	install -m 755 -d ${DESTDIR}${BINDIR}
	install -m 755 $(BLDDIR)/cmd/connector ${DESTDIR}${BINDIR}/connector
	install -m 755 $(BLDDIR)/cmd/checker ${DESTDIR}${BINDIR}/checker

deps:
	@go get github.com/tools/godep

protoc:
	protoc -I/usr/local/include -Iproto proto/bastion.proto --go_out=plugins=grpc:proto

test: build
	@godep go test -v ./...

fmt:
	@gofmt -w ./

