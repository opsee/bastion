PREFIX=/usr/local
DESTDIR=
GOFLAGS=
BINDIR=${PREFIX}/bin

CONNECTOR_SRCS = $(wildcard cmd/connector/*.go)
BASTION_SRCS = $(wildcard cmd/bastion/*.go)

CMDS = connector bastion
BLDDIR = target

all: $(CMDS)

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	@godep go build ${GOFLAGS} -o $(abspath $@) ./$*

$(BINARIES): $: $(BLDDIR)/%
$(CMDS): %: $(BLDDIR)/cmd/%

$(BLDDIR)/cmd/connector: $(CONNECTOR_SRCS)
$(BLDDIR)/cmd/bastion: $(BASTION_SRCS)

clean:
	rm -fr $(BLDDIR)

.PHONY: install clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)

install: $(BINARIES)
	install -m 755 -d ${DESTDIR}${BINDIR}
	install -m 755 $(BLDDIR)/cmd/connector ${DESTDIR}${BINDIR}/connector
	install -m 755 $(BLDDIR)/cmd/bastion ${DESTDIR}${BINDIR}/bastion

docker: test
	docker build -t opsee/bastion .

deps:
	@go get github.com/tools/godep

test: build
	@godep go test -v ./...

fmt:
	gofmt -w ./

