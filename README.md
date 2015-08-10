# bastion
the real bastion software

## Building

Bastion uses [gb](https://getgb.io) and [go-build](https://github.com/opsee/go-build).

### Compiling locally

`CGO_ENABLED=0 gb build`

Binaries will be under bin/

The bastion, of course, requires you to compile the bastion protobuf spec.
It's recommended to use the build container for that, as you will not have
to deal with dependencies locally.

### Building the container

```
docker pull quay.io/opsee/build-go
docker run -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:latest .
```
