package admission

import (
	"os"
	"strconv"
	"strings"
)

// Signals is a snapshot of host resource pressure used for admission decisions.
type Signals struct {
	// MemAvailableBytes is MemAvailable from /proc/meminfo.
	MemAvailableBytes uint64
	// MemPSI is the "some avg10" percentage from /proc/pressure/memory (0-100).
	MemPSI float64
	// Valid is false when host signals could not be read; admission then
	// falls back to the static concurrency gate.
	Valid bool
}

// Probe reads current host signals. PSI files may be absent (kernel without
// CONFIG_PSI); missing PSI reads as zero pressure while memory info is required.
func Probe() Signals {
	s := Signals{}
	mem, ok := readMemAvailable("/proc/meminfo")
	if !ok {
		return s
	}
	s.MemAvailableBytes = mem
	s.MemPSI = readPSISomeAvg10("/proc/pressure/memory")
	s.Valid = true
	return s
}

func readMemAvailable(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, "MemAvailable:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

func readPSISomeAvg10(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if val, ok := strings.CutPrefix(field, "avg10="); ok {
				f, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return 0
				}
				return f
			}
		}
	}
	return 0
}
