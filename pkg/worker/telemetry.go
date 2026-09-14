package worker

import "time"

// ewmaAlpha is the smoothing factor for per-repo resource telemetry.
// 0.3 weights recent runs enough to track upstream growth while damping
// one-off spikes (e.g. a full re-sync after checkpoint loss).
const ewmaAlpha = 0.3

// Telemetry accumulates per-repo resource usage across sync runs.
// It is persisted in the manager checkpoint so learned budgets survive
// restarts, and feeds the admission controller's memory estimates.
type Telemetry struct {
	// LastDuration is the wall time of the last completed attempt chain.
	LastDuration time.Duration `json:"last_duration_ns,omitempty"`
	// DurationEWMA is the exponentially weighted moving average of durations.
	DurationEWMA time.Duration `json:"duration_ewma_ns,omitempty"`
	// LastPeakMem is the peak memory (bytes) of the last run, when measurable.
	LastPeakMem uint64 `json:"last_peak_mem,omitempty"`
	// PeakMemEWMA is the EWMA of peak memory in bytes.
	PeakMemEWMA uint64 `json:"peak_mem_ewma,omitempty"`
	// OOMKills counts runs terminated by the kernel OOM killer (cgroup runner only).
	OOMKills uint64 `json:"oom_kills,omitempty"`
	// Runs counts observed runs contributing to the EWMAs.
	Runs uint64 `json:"runs,omitempty"`
}

// Observe folds one run's measurements into the telemetry.
// Zero-valued measurements (unavailable) do not disturb the EWMAs.
func (t *Telemetry) Observe(duration time.Duration, peakMem uint64, oomKilled bool) {
	t.Runs++
	if duration > 0 {
		t.LastDuration = duration
		if t.DurationEWMA == 0 {
			t.DurationEWMA = duration
		} else {
			t.DurationEWMA = time.Duration(ewmaAlpha*float64(duration) + (1-ewmaAlpha)*float64(t.DurationEWMA))
		}
	}
	if peakMem > 0 {
		t.LastPeakMem = peakMem
		if t.PeakMemEWMA == 0 {
			t.PeakMemEWMA = peakMem
		} else {
			t.PeakMemEWMA = uint64(ewmaAlpha*float64(peakMem) + (1-ewmaAlpha)*float64(t.PeakMemEWMA))
		}
	}
	if oomKilled {
		t.OOMKills++
	}
}
