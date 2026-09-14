package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobAttachHintTargetsProcessAndJoinsItsCgroup(t *testing.T) {
	hint := jobAttachHint(12345)

	assert.Equal(
		t,
		"nsenter --target=12345 --cgroup --join-cgroup -- <command>",
		hint,
	)
	assert.NotContains(t, hint, "--fork")
	assert.NotContains(t, hint, "/sys/fs/cgroup")
}
