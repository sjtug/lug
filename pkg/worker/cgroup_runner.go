//go:build linux

package worker

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// cgroup v2 job isolation.
//
// Each sync job runs in an ephemeral cgroup `<lug cgroup>/jobs/<job>` created
// right before exec (the child enters it atomically via clone3+CgroupFD) and
// removed right after collection. This gives us:
//   - enforceable per-job memory.max (replaces the racy RLIMIT_AS utility),
//   - kill-the-whole-tree reclamation via cgroup.kill on ctx cancellation,
//   - free telemetry: memory.peak and memory.events oom_kill per run.
//
// Requirements: cgroup v2 unified hierarchy and a delegated subtree (systemd
// service with Delegate=yes, or a container started with a private, writable
// cgroup namespace). When unavailable, executors fall back to plain process
// execution with rusage-based telemetry; see fallback paths in the executor.

const cgroupMountpoint = "/sys/fs/cgroup"

var (
	cgroupRootOnce sync.Once
	cgroupJobsRoot string // "" means cgroup isolation unavailable
)

// jobsCgroupRoot lazily prepares `<own cgroup>/jobs` with the memory and pids
// controllers enabled. To satisfy the cgroup v2 "no internal processes" rule,
// all of lug's own processes are first moved into a `supervisor` leaf so that
// controllers can be enabled on the subtree.
// Returns "" if the environment does not support it.
func jobsCgroupRoot() string {
	cgroupRootOnce.Do(func() {
		root, err := setupJobsCgroup()
		if err != nil {
			log.WithField("event", "cgroup_unavailable").WithError(err).
				Warn("cgroup v2 job isolation unavailable; falling back to plain process execution " +
					"(no memory.max enforcement, rusage-only telemetry)")
			return
		}
		log.WithField("event", "cgroup_jobs_root").WithField("path", root).
			Info("cgroup v2 job isolation enabled")
		cgroupJobsRoot = root
	})
	return cgroupJobsRoot
}

func setupJobsCgroup() (string, error) {
	own, err := ownCgroupPath()
	if err != nil {
		return "", err
	}
	base := filepath.Join(cgroupMountpoint, own)

	// Ensure the unified hierarchy is really there and delegated to us.
	if err := unixAccessWritable(filepath.Join(base, "cgroup.subtree_control")); err != nil {
		return "", fmt.Errorf("cgroup subtree not delegated at %s: %w", base, err)
	}

	// Move ourselves (and any sibling procs) to a leaf so controllers can be
	// enabled on `base` (cgroup v2 forbids processes in non-leaf cgroups with
	// enabled subtree controllers).
	supervisor := filepath.Join(base, "supervisor")
	if err := os.MkdirAll(supervisor, 0o755); err != nil {
		return "", err
	}
	if err := moveAllProcs(base, supervisor); err != nil {
		return "", fmt.Errorf("failed to move lug into supervisor leaf: %w", err)
	}

	// memory and pids are required for enforcement and cleanup;
	// cpu/io may be unavailable in unprivileged setups, so enable best-effort.
	if err := os.WriteFile(filepath.Join(base, "cgroup.subtree_control"),
		[]byte("+memory +pids"), 0); err != nil {
		return "", fmt.Errorf("failed to enable memory controller: %w", err)
	}
	_ = os.WriteFile(filepath.Join(base, "cgroup.subtree_control"), []byte("+cpu +io"), 0)

	jobs := filepath.Join(base, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(jobs, "cgroup.subtree_control"),
		[]byte("+memory +pids"), 0); err != nil {
		return "", fmt.Errorf("failed to enable memory controller on jobs: %w", err)
	}
	return jobs, nil
}

// ownCgroupPath returns this process's cgroup2 path relative to the mountpoint.
func ownCgroupPath() (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// cgroup v2 entry: "0::/path"
		if rest, ok := strings.CutPrefix(scanner.Text(), "0::"); ok {
			return strings.TrimSuffix(rest, " (deleted)"), nil
		}
	}
	return "", errors.New("no cgroup v2 entry in /proc/self/cgroup")
}

func unixAccessWritable(path string) error {
	return syscall.Access(path, 0x2 /* W_OK */)
}

func moveAllProcs(from, to string) error {
	data, err := os.ReadFile(filepath.Join(from, "cgroup.procs"))
	if err != nil {
		return err
	}
	for _, pid := range strings.Fields(string(data)) {
		if err := os.WriteFile(filepath.Join(to, "cgroup.procs"), []byte(pid), 0); err != nil {
			return err
		}
	}
	return nil
}

// jobCgroup is one ephemeral per-run cgroup.
type jobCgroup struct {
	dir string
	fd  int
}

// newJobCgroup creates `<jobs root>/<name>` with memory.max set to memMax
// (0 = unlimited) and returns a handle whose fd can be passed to
// SysProcAttr.CgroupFD. Returns (nil, nil) when cgroup isolation is
// unavailable (caller should fall back).
func newJobCgroup(name string, memMax uint64) (*jobCgroup, error) {
	root := jobsCgroupRoot()
	if root == "" {
		return nil, nil
	}
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.Remove(dir) }
	if memMax > 0 {
		if err := os.WriteFile(filepath.Join(dir, "memory.max"),
			[]byte(strconv.FormatUint(memMax, 10)), 0); err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to set memory.max: %w", err)
		}
		// A budget that silently spills to swap is not a budget: without this
		// a job exceeding memory.max degrades the whole host via swap I/O
		// instead of failing loudly. Best-effort (swap controller may be absent).
		_ = os.WriteFile(filepath.Join(dir, "memory.swap.max"), []byte("0"), 0)
		// Kill the whole job atomically on OOM instead of leaving a
		// half-dead process tree behind.
		_ = os.WriteFile(filepath.Join(dir, "memory.oom.group"), []byte("1"), 0)
	}
	fd, err := syscall.Open(dir, syscall.O_DIRECTORY|syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		cleanup()
		return nil, err
	}
	return &jobCgroup{dir: dir, fd: fd}, nil
}

// Kill terminates every process in the job cgroup (cgroup.kill, Linux >= 5.14).
func (j *jobCgroup) Kill() error {
	return os.WriteFile(filepath.Join(j.dir, "cgroup.kill"), []byte("1"), 0)
}

// Collect reads telemetry after the job's main process has exited.
func (j *jobCgroup) Collect() (peakMem uint64, oomKilled bool) {
	if data, err := os.ReadFile(filepath.Join(j.dir, "memory.peak")); err == nil {
		peakMem, _ = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}
	if data, err := os.ReadFile(filepath.Join(j.dir, "memory.events")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if cnt, ok := strings.CutPrefix(line, "oom_kill "); ok {
				n, _ := strconv.ParseUint(strings.TrimSpace(cnt), 10, 64)
				oomKilled = n > 0
			}
		}
	}
	return
}

// Close releases the fd and removes the cgroup. Descendant processes may
// still be draining after cgroup.kill, so removal is retried briefly.
func (j *jobCgroup) Close() {
	_ = syscall.Close(j.fd)
	for i := 0; i < 10; i++ {
		if err := os.Remove(j.dir); err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.WithField("event", "cgroup_remove_failed").WithField("dir", j.dir).
		Warn("failed to remove job cgroup; it will leak until manual cleanup")
}

// attachToCgroup makes cmd's child enter the job cgroup atomically at
// clone3 time (CLONE_INTO_CGROUP), so not a single instruction runs outside
// the resource-limited scope.
func attachToCgroup(cmd *exec.Cmd, cg *jobCgroup) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = cg.fd
}
