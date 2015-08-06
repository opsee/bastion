# bastion
the real bastion software

## Building

### Compiling locally

`CGO_ENABLED=0 gb build`

Binaries will be under bin/

### Building the container

```
docker pull quay.io/opsee/go-build
docker run -v `pwd`:/build quay.io/opsee/go-build
docker buil -t quay.io/opsee/bastion:latest .
```
