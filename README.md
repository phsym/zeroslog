# zeroslog

[![godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/phsym/zeroslog) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/phsym/zeroslog/master/LICENSE) [![Build](https://github.com/phsym/zeroslog/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/phsym/zeroslog/actions/workflows/go.yml)

A zerolog handler for slog

## Example
```go
package main

import (
	"log/slog"
	"github.com/phsym/zeroslog"
)

func main() {
	logger := slog.New(
        zeroslog.NewJsonHandler(os.Stderr, &zeroslog.HandlerOptions{Level: slog.LevelDebug})
    )
	slog.SetDefault(logger)
    slog.Info("Hello world!", "foo", "bar")
}

```