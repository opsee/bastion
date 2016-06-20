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

VPN_REMOTE ?= "bastion.in.opsee.com"
DNS_SERVER ?= "169.254.169.253"
NSQD_HOST ?= "nsqd:4150"
BARTNET_HOST ?= "http://localhost:8080"
BASTION_AUTH_ENDPOINT ?= "none"
BASTION_AUTH_TYPE ?= "BASIC_TOKEN"
BASTION_ID ?= "stub-bastion-id"
CUSTOMER_ID ?= "stub-customer-id"
CUSTOMER_EMAIL ?= "email"
CUSTOMER_PASSWORD ?= "password"
DNS_SERVER ?= "169.254.169.253"
ENABLE_BASTION_INGRESS ?= "false"
LOG_LEVEL ?= "info"
SLATE_HOST ?= "slate:7000"
BEZOS_HOST ?= "bezosphere.in.opsee.com:8443"
TARGETS ?= linux/amd64

build: deps
	docker run \
	--link bastion_slate_1:slate \
	--link bastion_nsqd_1:nsqd \
	-e SLATE_HOST=$(SLATE_HOST) \
	-e TARGETS=$(TARGETS) \
	-e NSQD_HOST=$(NSQD_HOST) \
	-e BEZOS_HOST=$(BEZOS_HOST) \
	-e CUSTOMER_ID=$(CUSTOMER_ID) \
	-e CUSTOMER_EMAIL=$(CUSTOMER_EMAIL) \
	-e BARTNET_HOST=$(BARTNET_HOST) \
	-e BASTION_AUTH_TYPE=$(BASTION_AUTH_TYPE) \
	-e CUSTOMER_PASSWORD=$(CUSTOMER_PASSWORD) \
	-e BASTION_AUTH_ENDPOINT=$(BASTION_AUTH_ENDPOINT) \
	-e AWS_DEFAULT_REGION \
	-e PROJECT=$(PROJECT) \
	-e LOG_LEVEL=$(LOG_LEVEL) \
	-v `pwd`:/gopath/src/$(PROJECT) \
	quay.io/opsee/build-go:16
	docker build -t quay.io/opsee/bastion:$(BASTION_VERSION) .

docker-push:
	docker push quay.io/opsee/bastion:${BASTION_VERSION} 

fmt:
	@gofmt -w .

.PHONY: clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)
