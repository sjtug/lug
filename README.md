# lug [![Build Status](https://travis-ci.org/sjtug/lug.svg)](https://travis-ci.org/sjtug/lug)

Extensible backend of software mirror. Read our [Wiki](https://github.com/sjtug/lug/wiki) for usage and guides for developmenet.

## Development

Contributors should push to `dev` branch. Reviewed code will be merged to `master` branch.

1. set your `GOPATH` to a directory: `export GOPATH=/home`
2. `go get github.com/sjtug/lug`
3. `cd $GOPATH/src/github.com/sjtug/lug && git checkout dev`
4. Modify code, then use native `go build`(>=1.6, or 1.5 with `GO15VENDOREXPERIMENT` env var set) or `godep go build`(<=1.4) after installing [Godep](https://github.com/tools/godep) to build it

NOTICE: Please attach test files when contributing to your module

Used package:
 - **Logging**: `github.com/op/go-logging` (Singleton)
 - **Test**: Builtin `testing` package and `github.com/stretchr/testify/assert`
 - **Yaml**: `gopkg.in/yaml.v2`
