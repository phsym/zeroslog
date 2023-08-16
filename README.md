# zeroslog

[![Go Reference](https://pkg.go.dev/badge/github.com/phsym/zeroslog.svg)](https://pkg.go.dev/github.com/phsym/zeroslog) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/phsym/zeroslog/master/LICENSE) [![Build](https://github.com/phsym/zeroslog/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/phsym/zeroslog/actions/workflows/go.yml) [![codecov](https://codecov.io/gh/phsym/zeroslog/graph/badge.svg?token=VXOLS2AJOO)](https://codecov.io/gh/phsym/zeroslog) [![Go Report Card](https://goreportcard.com/badge/github.com/phsym/zeroslog)](https://goreportcard.com/report/github.com/phsym/zeroslog)

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
        zeroslog.NewJsonHandler(os.Stderr, &zeroslog.HandlerOptions{Level: slog.LevelDebug}),
    )
	slog.SetDefault(logger)
    slog.Info("Hello world!", "foo", "bar")
}

```