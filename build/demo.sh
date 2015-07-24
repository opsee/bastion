#!/bin/sh

export GOPATH=`pwd`
go get ./...
go run cmd/bastion/main.go --access_key_id=AKIAJJA5SLKBYDIE23AQ --secret_key=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 --region=us-west-1 --opsee=localhost:5555 --hostname="cliff"
