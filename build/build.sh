#!/bin/bash
set -e

cd $GOPATH/src/github.com/opsee/bastion
./dist.sh
mv target/linux/cmd/* /export
