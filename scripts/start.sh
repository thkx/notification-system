#!/bin/bash

cd "$(dirname "$0")/.."

echo "Starting notification system..."
go run cmd/main.go
