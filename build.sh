#!/bin/sh

proto_dir=src/github.com/opsee/bastion/proto
proto=${proto_dir}/bastion.proto
protoc -I/usr/local/include -I${proto_dir} --go_out=plugins=grpc:${proto_dir} ${proto}
