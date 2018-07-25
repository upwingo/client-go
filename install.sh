#!/usr/bin/env bash

cd `dirname $0`

env GOPATH=`pwd` go get -v github.com/sacOO7/socketcluster-client-go
env GOPATH=`pwd` go build -v -o bin/example client/cmd/example
