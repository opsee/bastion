PROJECT := github.com/opsee/bastion

# docker tag is BASTION_VERSION unless BASTION_VERSION is set
BASTION_VERSION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	GITCOMMIT := $(BASTION_VERSION)-dirty
endif

all: clean fmt build

deps:
	docker-compose stop
	docker-compose rm -f
	docker-compose up -d
	docker run --link bastion_slate_1 aanand/wait

clean:
	rm -fr target bin pkg

build: deps
	docker run \
	--link bastion_slate_1:slate \
	--link bastion_nsqd_1:nsqd \
	-e AWS_DEFAULT_REGION \
	-e "TARGETS=linux/amd64"  \
	-e PROJECT=$(PROJECT) \
	-v `pwd`:/gopath/src/$(PROJECT) \
	quay.io/opsee/build-go:16
	docker build -t quay.io/opsee/bastion:${BASTION_VERSION} .

docker-push:
	docker push quay.io/opsee/bastion:${BASTION_VERSION} 

fmt:
	@gofmt -w .

.PHONY: clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)
