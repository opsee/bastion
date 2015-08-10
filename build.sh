#!/bin/sh

proto_dir=proto/
proto=${proto_dir}/checker.proto
protoc -I/usr/local/include -I${proto_dir} --go_out=plugins=grpc:${proto_dir} ${proto}
