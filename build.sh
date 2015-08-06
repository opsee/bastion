#!/bin/sh

protoc -I/usr/local/include -I./proto proto/bastion.proto --go_out=plugins=grpc:proto
