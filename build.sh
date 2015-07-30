#!/bin/bash
set -e

export VERSION=$(git rev-parse HEAD)

docker build -t bastion-build -f Dockerfile.build .

mkdir -p export
docker run -v `pwd`/export:/export bastion-build

docker build -f Dockerfile.bastion -t quay.io/opsee/bastion:latest .
