PREFIX=/usr/local
DESTDIR=
GOFLAGS=-v
BINDIR=${PREFIX}/bin

CONNECTOR_SRCS = $(wildcard *.go)
BASTION_SRCS = $(wildcard *.go)
WORKER_SRCS = $(wildcard *.go)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

CMDS = connector bastion worker
BLDDIR = target/${GOOS}

all: deps fmt test $(CMDS)

build: deps fmt $(CMDS)
	go build test.go

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	@godep go build ${GOFLAGS} -o $(abspath $@) ./$*

$(BINARIES): $: $(BLDDIR)/%
$(CMDS): %: $(BLDDIR)/cmd/%

$(BLDDIR)/cmd/connector: $(CONNECTOR_SRCS)
$(BLDDIR)/cmd/bastion: $(BASTION_SRCS)
$(BLDDIR)/cmd/worker: $(WORKER_SRCS)

clean:
	rm -fr target

.PHONY: install clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)

install: $(BINARIES)
	install -m 755 -d ${DESTDIR}${BINDIR}
	install -m 755 $(BLDDIR)/cmd/connector ${DESTDIR}${BINDIR}/connector
	install -m 755 $(BLDDIR)/cmd/bastion ${DESTDIR}${BINDIR}/bastion
	install -m 755 $(BLDDIR)/cmd/worker ${DESTDIR}${BINDIR}/worker

deps:
	@go get github.com/tools/godep

test: build
	@godep go test -v ./...

fmt:
	@gofmt -w ./

