package admission

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadMemAvailable(t *testing.T) {
	asrt := assert.New(t)
	path := filepath.Join(t.TempDir(), "meminfo")
	asrt.NoError(os.WriteFile(path, []byte(
		"MemTotal:       32663396 kB\nMemFree:         1109108 kB\nMemAvailable:   17972072 kB\n"), 0o644))
	mem, ok := readMemAvailable(path)
	asrt.True(ok)
	asrt.Equal(uint64(17972072*1024), mem)

	_, ok = readMemAvailable(filepath.Join(t.TempDir(), "missing"))
	asrt.False(ok)
}

func TestReadPSISomeAvg10(t *testing.T) {
	asrt := assert.New(t)
	path := filepath.Join(t.TempDir(), "psi")
	asrt.NoError(os.WriteFile(path, []byte(
		"some avg10=75.83 avg60=85.71 avg300=91.47 total=13174650506\n"+
			"full avg10=72.90 avg60=83.80 avg300=90.37 total=12876717463\n"), 0o644))
	asrt.InDelta(75.83, readPSISomeAvg10(path), 0.001)
	asrt.Zero(readPSISomeAvg10(filepath.Join(t.TempDir(), "missing")))
}

func TestProbeOnRealHost(t *testing.T) {
	s := Probe()
	if !s.Valid {
		t.Skip("/proc/meminfo unavailable")
	}
	assert.Greater(t, s.MemAvailableBytes, uint64(0))
}
