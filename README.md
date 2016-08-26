# lug [![release](https://img.shields.io/github/release/sjtug/lug.svg])(https://github.com/sjtug/lug/releases)[![Build Status](https://travis-ci.org/sjtug/lug.svg)](https://travis-ci.org/sjtug/lug)[![Docker pulls](https://img.shields.io/docker/pulls/sjtug/lug.svg)](https://hub.docker.com/r/sjtug/lug/)[![Apache License](https://img.shields.io/github/license/sjtug/lug.svg)](https://github.com/sjtug/lug/blob/master/LICENSE)

Extensible backend of software mirror. Read our [Wiki](https://github.com/sjtug/lug/wiki) for usage and guides for developmenet.

## Use it in docker
```
docker run -d -v {{host_path}}:{{docker_path}} -v {{absolute_path_of_config.yaml}}:/config.yaml sjtug/lug {other args...}
```

### config.yaml
```
interval: 3 # Interval between pollings
loglevel: 5 # 0-5. 0 for ERROR and 5 for DEBUG
logstashaddr: "172.0.0.4:6000" # TCP Address of logstash. empty means no logstash support
repos:
    - type: rsync # Config for repo1
      source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/
      interval: 6 # Interval between sync
      path: /tmp/putty # Output directory
      name: putty # Required
      rlimit_mem: 200M # Optional, maximum memory can be used
# You can add more repos here, different repos may have different worker types,
# refer to Worker Types section for detailed explanation
```

## Development

Contributors should push to `dev` branch. Reviewed code will be merged to `master` branch.

1. set your `GOPATH` to a directory: `export GOPATH=/home`
2. `go get github.com/sjtug/lug`
3. `cd $GOPATH/src/github.com/sjtug/lug && git checkout dev`
4. Modify code, then use native `go build`(>=1.6, or 1.5 with `GO15VENDOREXPERIMENT` env var set) or `godep go build`(<=1.4) after installing [Godep](https://github.com/tools/godep) to build it
5. Run `scripts/gen_license.sh` and `scripts/savedep.sh`(with Godep installed) before committing your code

NOTICE: Please attach test files when contributing to your module

Used package:
 - **Logging**: `github.com/op/go-logging` (Singleton)
 - **Test**: Builtin `testing` package and `github.com/stretchr/testify/assert`
 - **Yaml**: `gopkg.in/yaml.v2`

