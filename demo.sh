#!/bin/sh

export GOPATH=`pwd`
go get ./...
go run cmd/bastion/main.go --access_key_id=a --secret_key=s --region=us-west-1 --opsee=localhost:5555 --hostname="cliff" --data=cookbooks/bastion/files/demo_data.json