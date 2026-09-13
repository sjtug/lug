# lug

[![release](https://img.shields.io/github/release/sjtug/lug.svg)](https://github.com/sjtug/lug/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/sjtug/lug)](https://goreportcard.com/report/github.com/sjtug/lug)

Extensible backend of software mirror.

## Quick start

```sh
docker run -d \
  -v /srv/mirror:/data \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -p 8081:8081 -p 7001:7001 \
  sjtug/lug -c /app/config.yaml
```

Refer to [`config.example.yaml`](config.example.yaml) for available options.

### Ports

| Port | Purpose |
|------|---------|
| 8081 | Prometheus metrics (`/metrics`) |
| 7001 | JSON control API |

## Configuration

See [`config.example.yaml`](config.example.yaml) for a full annotated example.
Key settings:

| Key | Description |
|-----|-------------|
| `interval` | Seconds between scheduler poll ticks |
| `concurrent_limit` | Hard cap on simultaneously running sync jobs |
| `checkpoint` | Path to persist worker state across restarts |
| `exporter_address` | Bind address for Prometheus metrics |
| `json_api.address` | Bind address for the JSON control API |

### Per-repo options

Each entry under `repos` configures one mirror worker. Shell-script workers
accept these additional fields:

| Key | Description |
|-----|-------------|
| `rlimit_mem` | Memory budget per job (e.g. `1G`). Enforced via cgroup `memory.max`; the entire job tree is OOM-killed when exceeded. Ignored gracefully when cgroup isolation is unavailable. |
| `timeout` | Wall-clock budget in seconds. The job tree is killed on expiry. |
| `interval` | Seconds between syncs for this repo. If unset, the repo is synced only at startup. |

### Elastic admission control

When `admission.enabled` is `true`, lug gates queued job launches on:

- **Memory PSI** — defers when memory `some/avg10` exceeds the threshold.
- **Memory headroom** — reserves a fraction of available memory and checks
  that the sum of granted budgets plus the new estimate fits.

Estimates are derived from per-repo peak-memory EWMAs learned over past runs.
Unknown repos assume a conservative default (512 MiB) until measured. The
static `concurrent_limit` always applies as a hard cap on top.

IO pressure is deliberately **not** an admission signal: a mirror host is
IO-saturated by design, and gating on it would permanently starve the queue.

## Cgroup v2 setup

Lug uses cgroup v2 to enforce per-job memory limits, kill entire process trees
on cancellation or timeout, and collect resource telemetry (peak memory, OOM
kills, duration). **This is optional**: when cgroup isolation is unavailable,
lug falls back to plain subprocess execution with a logged warning, and
`rlimit_mem` / admission memory estimates are simply not enforced.

### Requirements

1. **Linux host with cgroup v2 unified hierarchy** (`/sys/fs/cgroup` mounted
   as `cgroup2`). This is the default on all modern distros (Debian 11+,
   Ubuntu 22.04+, Fedora 31+, RHEL 9+, NixOS 20.09+).
2. **Delegated cgroup subtree** — lug must be able to create child cgroups and
   write controller knobs (`memory.max`, `cgroup.kill`, etc.) under its own
   cgroup. How to achieve this depends on the runtime.

### Rootful Docker (Docker Engine with root daemon)

Docker ≥ 20.10 supports `--cgroupns=private` (the default since Docker 23),
which gives each container its own cgroup namespace and a writable cgroup root.
Run lug with:

```sh
docker run -d \
  --cgroupns=private \
  -v /srv/mirror:/data \
  -v $(pwd)/config.yaml:/app/config.yaml \
  sjtug/lug -c /app/config.yaml
```

This is sufficient — no `--privileged`, no extra capabilities, no bind-mounts
of `/sys/fs/cgroup`. The container sees its own cgroup subtree as `/` and can
write to it freely.

> **Verify**: inside the container, `cat /sys/fs/cgroup/cgroup.controllers`
> should list `memory` (and ideally `pids`). If it shows an empty file or
> the path does not exist, the host likely uses cgroup v1.

### Rootless Podman

Rootless Podman ≥ 4.0 delegates a user-owned cgroup subtree by default when
the host runs systemd ≥ 247 and cgroup v2. No extra flags are needed:

```sh
podman run -d \
  -v /srv/mirror:/data \
  -v $(pwd)/config.yaml:/app/config.yaml \
  docker.io/sjtug/lug -c /app/config.yaml
```

### systemd service (bare metal / VM)

If running lug as a systemd service without containers:

```ini
[Service]
ExecStart=/usr/bin/lug -c /etc/lug/config.yaml
Delegate=yes
```

`Delegate=yes` permits lug to manage cgroups under its service scope.

### Disabling cgroup isolation

If cgroup v2 is unavailable (cgroup v1 host, restricted container, etc.), lug
detects this at startup and logs a warning:

```
level=warning msg="cgroup v2 job isolation unavailable; falling back to plain process execution (no memory.max enforcement, rusage-only telemetry)"
```

No configuration change is needed — the fallback is automatic. `rlimit_mem`
and `timeout` settings in the config are silently ignored when enforcement is
unavailable; admission control will operate without memory estimates.

## Observability

### Prometheus metrics

Exposed at `http://<exporter_address>/metrics` (default `:8081`).

**Sync counters** (labels: `worker`):

| Metric | Description |
|--------|-------------|
| `success_sync` | Successful sync runs (counter) |
| `fail_sync` | Failed sync runs (counter) |

**Resource telemetry** (labels: `worker`) — populated when cgroup isolation is
active:

| Metric | Description |
|--------|-------------|
| `lug_job_peak_mem_bytes` | Peak memory (bytes) of the last sync run |
| `lug_job_peak_mem_ewma_bytes` | EWMA of peak memory across runs |
| `lug_job_duration_seconds` | Wall time (seconds) of the last sync run |
| `lug_job_duration_ewma_seconds` | EWMA of sync run wall time |
| `lug_job_oom_kills_total` | Cumulative OOM kills observed |

**Admission verdicts** (labels: `outcome = admitted | deferred`):

| Metric | Description |
|--------|-------------|
| `lug_admission_verdicts_total` | Admission decisions (counter) |

**Disk usage** (labels: `worker`):

| Metric | Description |
|--------|-------------|
| `lug_disk_usage` | Disk usage in bytes |

### Structured logs

All log lines are JSON with `repo` and `sync_id` fields. Key events:

| Event | Description |
|-------|-------------|
| `attempt_telemetry` | Emitted after every sync attempt with `duration_sec`, `peak_mem`, `oom_killed`, `cgroup_scoped` |
| `trigger_sync` | A queued worker is launched |
| `admission_deferred` | Admission controller deferred a launch (`reason` field explains why) |
| `job_cgroup_kill` | Cancellation is reaping the job's cgroup tree |
| `abort_worker` | An operator aborted a sync via the REST API |

## JSON control API

Served at `json_api.address` (default `:7001`). All responses are JSON.

### Endpoints

#### `GET /lug/v1/manager/summary`

Worker status overview (public, safe for dashboards).

```json
{
  "Running": true,
  "WorkerStatus": {
    "putty": { "Result": true, "LastFinished": "2025-01-01T00:00:00Z", "Idle": true }
  }
}
```

#### `GET /lug/v1/admin/manager/detail`

Full status including per-worker stdout/stderr history and telemetry.

#### `POST /lug/v1/admin/manager/start`

Resume the scheduler after a stop. Workers whose interval has elapsed are
queued on the next poll tick.

#### `POST /lug/v1/admin/manager/stop`

Pause the scheduler. Already-running syncs finish, but no new syncs are
launched.

#### `POST /lug/v1/admin/worker/:name/abort`

Abort the in-flight sync of the named worker. The cancellation propagates
through the worker's context and (with cgroup isolation) kills the entire
process tree.

- **202 Accepted** — cancellation initiated.
- **404 Not Found** — no worker by that name, or the worker is idle.

```sh
# Example: abort a stuck rsync job
curl -X POST http://localhost:7001/lug/v1/admin/worker/putty/abort
```

#### `POST /lug/v1/admin/worker/:name/sync`

Trigger an immediate sync for the named worker. The worker is inserted at the
head of the pending queue (or lifted to the head if already queued) and
launched immediately if capacity allows.

- **202 Accepted** — sync queued / launched.
- **404 Not Found** — no worker by that name.
- **409 Conflict** — the worker is already syncing. Abort first if you need
  to restart it.

```sh
# Trigger putty sync right now
curl -X POST http://localhost:7001/lug/v1/admin/worker/putty/sync

# Force a restart: abort then re-trigger
curl -X POST http://localhost:7001/lug/v1/admin/worker/putty/abort
sleep 2
curl -X POST http://localhost:7001/lug/v1/admin/worker/putty/sync
```

#### `GET /lug/v1/admin/queue`

Snapshot of the task queue: which workers are currently syncing (with live
cgroup/PID info) and which are pending launch.

```json
{
  "running": [
    {
      "name": "putty",
      "active_job": {
        "cgroup_path": "/sys/fs/cgroup/system.slice/.../jobs/putty-1234567890",
        "main_pid": 12345,
        "started_at": "2025-01-01T00:00:00Z"
      }
    }
  ],
  "pending": ["ubuntu", "debian"]
}
```

#### `GET /lug/v1/admin/worker/:name/job`

Live details of the named worker's active sync job: cgroup path, PID, start
time, and — when cgroup isolation is active — live memory and PID counters
read directly from the kernel.

- **200 OK** — job details (see below).
- **404 Not Found** — no such worker, or the worker is idle.

```json
{
  "name": "putty",
  "active_job": {
    "cgroup_path": "/sys/fs/cgroup/.../jobs/putty-1234567890",
    "main_pid": 12345,
    "started_at": "2025-01-01T00:00:00Z"
  },
  "cgroup_stats": {
    "memory_current_bytes": 104857600,
    "memory_peak_bytes": 209715200,
    "memory_limit_bytes": 1073741824,
    "pids": 5
  },
  "attach_hint": "nsenter --cgroup=/sys/fs/cgroup/.../jobs/putty-1234567890 --fork <command>"
}
```

The `attach_hint` field shows the `nsenter` invocation to enter the job's
cgroup for interactive debugging (run a shell, inspect `/proc`, etc.).

#### `DELETE /lug/v1/admin/manager`

Gracefully shut down the manager (stop scheduler, then exit the run loop).

## Development

This project requires **Go ≥ 1.23** and uses Nix for the development
environment.

```sh
# Enter the dev shell (includes Go toolchain, golangci-lint, treefmt)
nix develop

# Build
go build ./...

# Run tests (memory-capped to avoid host OOM from cgroup enforcement tests)
go test -c -o /tmp/worker.test ./pkg/worker/
systemd-run --user --scope -p MemoryMax=2G -p Delegate=yes /tmp/worker.test -test.v

# Or without cgroup delegation (enforcement tests auto-skip):
go test ./...
```

Tests that exercise cgroup enforcement (`TestCgroupEnforcement`,
`TestCgroupTelemetryHappy`) require a delegated cgroup subtree and
automatically skip otherwise.
