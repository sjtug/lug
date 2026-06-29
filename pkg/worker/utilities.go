package worker

import "os/exec"

type utility interface {
	apply(*exec.Cmd) error
}
