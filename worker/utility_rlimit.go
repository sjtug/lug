package worker

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"syscall"
)

type rlimit struct {
	oldRlimit syscall.Rlimit
	w         Worker
}

func newRlimit(w Worker) *rlimit {
	return &rlimit{
		w: w,
	}
}

type rlimitError string

func (re rlimitError) Error() string {
	return string(re)
}

func (r *rlimit) preHook() error {
	cfg := r.w.GetConfig()
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &r.oldRlimit); err != nil {
		return rlimitError(fmt.Sprint("Failed to getrlimit:", err))
	}
	if rlimitMem, ok := cfg["rlimit_mem"]; ok {
		if bytes, err := humanize.ParseBytes(rlimitMem.(string)); err == nil {
			var rlimitNew syscall.Rlimit
			rlimitNew = r.oldRlimit
			rlimitNew.Cur = bytes
			err := syscall.Setrlimit(syscall.RLIMIT_AS, &rlimitNew)
			if err != nil {
				return rlimitError(fmt.Sprint("Failed to setrlimit:", err))
			}
		} else {
			return rlimitError(fmt.Sprint("Invalid rlimit_mem: must be size:", err))
		}
	}
	return nil
}

func (r *rlimit) postHook() error {
	err := syscall.Setrlimit(syscall.RLIMIT_AS, &r.oldRlimit)
	if err != nil {
		return rlimitError(fmt.Sprint("Failed to restore rlimit:", err))
	}
	return nil
}
