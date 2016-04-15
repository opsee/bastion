# docker tag is BASTION_VERSION unless BASTION_VERSION is set
BASTION_VERSION ?= $(shell git rev-parse HEAD)
 

all: clean fmt build

deps:
	docker-compose stop
	docker-compose rm -f
	docker-compose up -d
	docker run --link bastion_slate_1 aanand/wait

clean:
	$(MAKE) -i docker-clean
	rm -fr target bin pkg

docker-clean:
	docker rm -vf quay.io/opsee/bastion:${BASTION_VERSION}

build: deps
	docker run \
	--link bastion_slate_1:slate \
	--link bastion_nsqd_1:nsqd \
	-e "SLATE_HOST=slate:7000" \
	-e "TARGETS=linux/amd64"  \
	-e "NSQD_HOST=nsqd:4150" \
	-e CUSTOMER_ID \
	-e CUSTOMER_EMAIL \
	-e BARTNET_HOST \
	-e BASTION_AUTH_TYPE \
	-e CUSTOMER_PASSWORD \
	-e BASTION_AUTH_ENDPOINT \
	-e AWS_DEFAULT_REGION \
	-v `pwd`:/build quay.io/opsee/build-go:go15
	docker build -t quay.io/opsee/bastion:${BASTION_VERSION} .

docker-push:
	docker push quay.io/opsee/bastion:${BASTION_VERSION} 

fmt:
	@gofmt -w ./src

.PHONY: clean all
.PHONY: $(BINARIES)
.PHONY: $(CMDS)
