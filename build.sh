#!/bin/bash

eval "$(/usr/bin/env go env)"

proto_dir=proto
proto=${proto_dir}/checker.proto
protoc -I/usr/local/include -I${proto_dir}/ --go_out=plugins=grpc:${proto_dir}/ ${proto}
rm -f src/github.com/opsee/bastion/checker/checker.pb.go
ln -s ../../../../../proto/checker.pb.go src/github.com/opsee/bastion/checker/checker.pb.go

if [ "$GOOS" = "darwin" ]; then
  sed -i'' -e 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/checker.pb.go
else
  sed -i 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/checker.pb.go
fi
