#!/usr/bin/env sh
cd cli && golangci-lint run "$@"
