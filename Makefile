PREFIX=/usr/local
DESTDIR=
GOFLAGS=-v
BINDIR=${PREFIX}/bin

CONNECTOR_SRCS = $(wildcard *.go)
BASTION_SRCS = $(wildcard *.go)
CHECKER_SRCS = $(wildcard *.go)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

CMDS = connector checker
BLDDIR = target/${GOOS}

all: deps fmt test $(CMDS)

build: deps fmt $(CMDS)
	@godep go build test.go

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	@godep go build ${GOFLAGS} -o $(abspath $@) ./$*

$(BINARIES): $: $(BLDDIR)/%
$(CMDS): %: $(BLDDIR)/cmd/%

$(BLDDIR)/cmd/connector: $(CONNECTOR_SRCS)
$(BLDDIR)/cmd/checker: $(CHECKER_SRCS)

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

test: build
	@godep go test -v ./...

fmt:
	@gofmt -w ./

