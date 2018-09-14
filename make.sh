#!/bin/bash
set -e

echo "Generating code..."
go generate ./dashboard && go generate ./types/subscriber.go

echo "Linting..."
gometalinter --exclude=gen- --exclude=bindata --deadline=300s --vendor ./...

echo "Building..."
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o updog .
