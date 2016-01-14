#!/bin/bash

eval "$(/usr/bin/env go env)"

proto_dir=proto
checker_proto=${proto_dir}/checker.proto
aws_proto=${proto_dir}/aws.proto

protoc --go_out=plugins=grpc,Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor:. ${checker_proto}
protoc --go_out=plugins=grpc,Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor:. ${aws_proto}

mv ./proto/checker.pb.go src/github.com/opsee/bastion/checker/checker.pb.go
mv ./proto/aws.pb.go src/github.com/opsee/bastion/aws_command/aws.pb.go
