# lug 
[![release](https://img.shields.io/github/release/sjtug/lug.svg)](https://github.com/sjtug/lug/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/sjtug/lug)](https://goreportcard.com/report/github.com/sjtug/lug)
[![Build Status](https://travis-ci.org/sjtug/lug.svg)](https://travis-ci.org/sjtug/lug)
[![Docker pulls](https://img.shields.io/docker/pulls/htfy96/lug.svg)](https://hub.docker.com/r/htfy96/lug/)
[![Apache License](https://img.shields.io/github/license/sjtug/lug.svg)](https://github.com/sjtug/lug/blob/master/LICENSE)

Extensible backend of software mirror. Read our [Wiki](https://github.com/sjtug/lug/wiki) for usage and guides for developmenet.

## Use it in docker
```
docker run -d -v {{host_path}}:{{docker_path}} -v {{absolute_path_of_config.yaml}}:/go/src/github.com/sjtug/lug/config.yaml htfy96/lug {other args...}
```

### Build a container image with nix2container

There is a `docker-image` flake output (built with [nix2container](https://github.com/nlewo/nix2container)) for testing the daemon in a container. It layers the `lug` binary plus the runtime tools (`rsync`, `coreutils`, ...) the `shell_script` workers exec, and sets `PATH`/`CHECKPOINT_DIR` accordingly.

```sh
# load into podman (or use copyToDockerDaemon for docker)
nix run '.#docker-image.copyToPodman'

# run with a config mounted at /config.yaml (the image's default Cmd):
podman run --rm -v $PWD/config.example.yaml:/config.yaml:ro lug:latest
```

Logs are emitted as a JSON event stream to stdout (see PLAN, Phase 0 / 12-factor XI), so `podman run ... | jq` works:

### config.yaml

The below configuration may be outdated. Refer to [config.example.yaml](https://github.com/sjtug/lug/blob/master/config.example.yaml)
and [Wiki](https://github.com/sjtug/lug/wiki/Configuration) for the latest version.

```
interval: 3 # Interval between pollings
loglevel: 5 # 0-5. 0 for ERROR and 5 for DEBUG
logstashaddr: "172.0.0.4:6000" # TCP Address of logstash. empty means no logstash support
dummy: # place your anchor here!
    common_interval: &common_interval
      interval: 3600
    common_retry: &commone_retry
      retry: 3
repos:
    - type: shell_script
      script: rsync -av rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/ /tmp/putty
      name: putty
      rlimit: 300M
      <<: *common_interval # interval: 3600 will be inserted here
    - type: external
      name: ubuntu
      proxy_to: http://ftp.sjtu.edu.cn/ubuntu/
      <<: [*common_interval, *common_retry] # use array for importing multiple anchors
# You can add more repos here, different repos may have different worker types,
# refer to Worker Types section for detailed explanation
```

## Development

Contributors should push to their own branch. Reviewed code will be merged to `master` branch.

Currently this project assumes Go >= 1.23.

1. set your `GOPATH` to a directory: `export GOPATH=/home/go`. Set `$GOPATH/bin` to your `$PATH`: `export PATH=$PATH:$GOPATH/bin`
2. `go get github.com/sjtug/lug`
3. Install dep by `curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh`
3. `cd $GOPATH/src/github.com/sjtug/lug && dep ensure`
4. Modify code, then use `go build .` to build binary, or test with `go test $(go list ./... | grep -v /vendor/)`
5. Run `scripts/gen_license.sh` before committing your code

NOTICE: Please attach test files when contributing to your module

