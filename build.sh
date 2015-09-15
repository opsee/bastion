#!/bin/bash

proto_dir=proto
proto=${proto_dir}/checker.proto
protoc -I/usr/local/include -I${proto_dir}/ --go_out=plugins=grpc:${proto_dir}/ ${proto}
rm -f src/github.com/opsee/bastion/checker/checker.pb.go
ln -s ../../../../../proto/checker.pb.go src/github.com/opsee/bastion/checker/checker.pb.go
sed -i 's/import google_protobuf "google\/protobuf"/import google_protobuf "github.com\/peter-edge\/go-google-protobuf"/' ${proto_dir}/checker.pb.go
