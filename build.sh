#!/bin/bash

eval "$(/usr/bin/env go env)"

proto_dir=proto
checker_proto=${proto_dir}/checker.proto
aws_proto=${proto_dir}/aws.proto

protoc --go_out=plugins=grpc,Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor:. ${checker_proto}
protoc --go_out=plugins=grpc,Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor:. ${aws_proto}

#protoc -I/usr/local/include -I${proto_dir}/ --go_out=plugins=grpc:${proto_dir}/ ${checker_proto}
#protoc -I/usr/local/include -I${proto_dir}/ --go_out=plugins=grpc:${proto_dir}/ ${aws_proto}

rm -f src/github.com/opsee/bastion/checker/checker.pb.go
rm -f src/github.com/opsee/bastion/aws_command/aws.pb.go
ln -s ../../../../../proto/checker.pb.go src/github.com/opsee/bastion/checker/checker.pb.go
ln -s ../../../../../proto/aws.pb.go src/github.com/opsee/bastion/aws_command/aws.pb.go

#if [ "$GOOS" = "darwin" ]; then
#  sed -i'' -e 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/checker.pb.go
#  sed -i'' -e 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/aws.pb.go
#else
#  sed -i 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/checker.pb.go
#  sed -i 's/import google_protobuf "google\/protobuf"/import google_protobuf "go.pedge.io\/google-protobuf"/' ${proto_dir}/aws.pb.go
#fi
