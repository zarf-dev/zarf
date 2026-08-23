# go-sync

[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/go-sync)](https://goreportcard.com/report/github.com/anchore/go-sync)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/anchore/go-sync.svg)](https://github.com/anchore/go-sync)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/go-sync/blob/main/LICENSE)
[![Slack Invite](https://img.shields.io/badge/Slack-Join-blue?logo=slack)](https://anchore.com/slack)

A collection of synchronization utilities.

## Status

***Consider this project to be in alpha. The API is not stable and may change at any time.***

## Overview

`sync.Executor` - a simple executor interface, with a bounded executor implementation available by using `sync.NewExecutor`

`sync.Collector` - a simple interface to concurrently execute tasks and get the results

`sync.List` - a concurrent list, queue, and stack implementation
