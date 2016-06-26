# lug [![Build Status](https://travis-ci.org/sjtug/lug.svg)](https://travis-ci.org/sjtug/lug)

Extensible backend of software mirror

## Development

Contributors should push to `dev` branch. Reviewed code will be merged to `master` branch.

1. set your `GOPATH` to a directory: `export GOPATH=/home`
2. `mkdir -p $GOPATH/src/github.com/sjtug/lug && cd $GOPATH/src/github.com/sjtug/lug`
3. `git clone {URL of this repo} . && git checkout dev`
4. `go get github.com/sjtug/lug`, and binary will be built under `$GOPATH/bin`

NOTICE: Please attach test files when contributing to your module

Used package:
 - **Logging**: `github.com/op/go-logging` (Singleton)
 - **Test**: Builtin `testing` package and `github.com/stretchr/testify/assert`
 - **Yaml**: `gopkg.in/yaml.v2`
