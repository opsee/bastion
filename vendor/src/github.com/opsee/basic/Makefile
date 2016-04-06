test:
	docker run --rm -it -v $$(pwd):/gopath/src/github.com/opsee/basic quay.io/opsee/build-go:proto16 /bin/bash -c 'cd /gopath/src/github.com/opsee/basic && ./build.sh'

generate:
	docker run --rm -it -v $$(pwd):/gopath/src/github.com/opsee/basic quay.io/opsee/build-go:proto16 /bin/bash -c 'cd /gopath/src/github.com/opsee/basic && ./generate.sh'

.PHONY:
	generate
	test