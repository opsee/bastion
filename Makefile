PREFIX=/usr/local
DESTDIR=
GOFLAGS=-v
BINDIR=${PREFIX}/bin

SRCS = $(wildcard *.go)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

#CMDS = connector checker
CMDS = $(notdir $(wildcard cmd/*))
BLDDIR = target/${GOOS}

all: deps fmt test $(CMDS)

build: deps fmt $(CMDS)
	@godep go build test.go

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	@CGO_ENABLED=0 godep go build -ldflags '-w' -tags netgo ${GOFLAGS} -o $(abspath $@) ./$*

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

test: build
	@godep go test -v ./...

fmt:
	@gofmt -w ./

