#!/bin/bash

echo "Linting..."
gometalinter --vendor ./...

echo "Generating code..."
go generate ./dashboard && go generate ./types/subscriber.go

echo "Building..."
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o updog .
