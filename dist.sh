#!/bin/bash
set -e


os=linux
arch=$(go env GOARCH)
version=$(git rev-parse HEAD)
goversion=$(go version | awk '{print $3}')

echo "... building v$version for $os/$arch"
GOOS=$os GOARCH=$arch CGO_ENABLED=0 make

docker build -t quay.io/opsee/bastion:latest .
