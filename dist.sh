#!/bin/bash
set -e

# Should be run inside of a container.

os=linux
arch=$(go env GOARCH)
goversion=$(go version | awk '{print $3}')

echo "... building v$VERSION for $os/$arch"
make clean
GOOS=$os GOARCH=$arch CGO_ENABLED=0 make
cp target/linux/cmd/* /export
