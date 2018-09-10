#!/bin/bash

echo "Generating code..."
go generate ./dashboard && go generate ./types/subscriber.go

echo "Linting..."
gometalinter --exclude=gen- --exclude=bindata --vendor ./...

echo "Building..."
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o updog .
