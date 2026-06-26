# lug 
[![release](https://img.shields.io/github/release/sjtug/lug.svg)](https://github.com/sjtug/lug/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/sjtug/lug)](https://goreportcard.com/report/github.com/sjtug/lug)
[![Build Status](https://travis-ci.org/sjtug/lug.svg)](https://travis-ci.org/sjtug/lug)
[![Docker pulls](https://img.shields.io/docker/pulls/htfy96/lug.svg)](https://hub.docker.com/r/htfy96/lug/)
[![Apache License](https://img.shields.io/github/license/sjtug/lug.svg)](https://github.com/sjtug/lug/blob/master/LICENSE)

Extensible backend of software mirror. Read our [Wiki](https://github.com/sjtug/lug/wiki) for usage and guides for developmenet.

## Run it in a container

The refactor branch provides a nix2container image for local/containerized testing. The image runs `lug` directly as PID 1 (no shell wrapper), defaults to `-c /config.yaml`, and also ships a slim `lugctl` admin client on `PATH`.

### Build/load with nix2container

There is a `docker-image` flake output (built with [nix2container](https://github.com/nlewo/nix2container)) for testing the daemon in a container. It layers the `lug` and `lugctl` binaries plus the runtime tools (`rsync`, `bash`, `coreutils`, ...) the `shell_script` workers exec, and sets `PATH`/`CHECKPOINT_DIR` accordingly.

```sh
# optional: build the image JSON closure
nix build .#docker-image

# load into podman (or use copyToDockerDaemon for docker)
nix run '.#docker-image.copyToPodman'

# run with a config mounted at /config.yaml (the image's default Cmd):
podman run --rm --net=host -v ./config.example.yaml:/config.yaml:ro lug:latest
```

Logs are emitted as a JSON event stream to stdout (see PLAN, Phase 0 / 12-factor XI), so `podman run ... | jq` works.

### Admin commands with `lugctl`

`lugctl` is a one-off control-plane client for a running Lug daemon. It defaults to `LUG_API=http://localhost:7001`, which means it works naturally inside the daemon container:

```sh
docker compose exec lug lugctl summary
docker compose exec lug lugctl status --repo ubuntu
docker compose exec lug lugctl sync --repo ubuntu
docker compose exec lug lugctl stop
docker compose exec lug lugctl exit
```

For non-`compose exec` use, point it at the daemon explicitly:

```sh
LUG_API=http://lug:7001 lugctl status
```

The admin endpoints are intended for the internal Compose network; do not expose `/lug/v1/admin/*` publicly.

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

This branch is developed with the flake dev shell. If `direnv` is enabled:

```sh
direnv allow
go build ./...
go test ./...
```

Or run commands through the dev shell explicitly:

```sh
direnv exec . go build ./...
direnv exec . go test ./...
```

The default Nix package builds both commands:

```sh
nix build .#default
ls result/bin   # lug  lugctl
```

NOTICE: Please attach test files when contributing to your module

