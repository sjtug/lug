package admission

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

// DefaultMemEstimate is assumed for jobs with no learned telemetry yet.
// Deliberately conservative: unknown jobs should not be admitted in bulk
// on a loaded host, and one successful run replaces the guess.
const DefaultMemEstimate = 512 << 20

// EstimateHeadroom scales the learned peak-memory EWMA into a budget,
// absorbing run-to-run variance and upstream growth.
const EstimateHeadroom = 1.5

// Config tunes the admission controller. Zero values select defaults.
type Config struct {
	// MemReserveFraction of MemAvailable is kept free (default 0.2).
	MemReserveFraction float64
	// MemPSIThreshold defers admission when memory "some avg10" exceeds it (default 15).
	MemPSIThreshold float64
}

func (c Config) withDefaults() Config {
	if c.MemReserveFraction <= 0 {
		c.MemReserveFraction = 0.2
	}
	if c.MemPSIThreshold <= 0 {
		c.MemPSIThreshold = 15
	}
	return c
}

// Request describes one job asking to start.
type Request struct {
	Name string
	// MemEstimate is the expected peak memory in bytes (0 = unknown).
	MemEstimate uint64
}

// Verdict is the outcome of an admission attempt.
type Verdict struct {
	Admit bool
	// Reason explains a deferral (empty when admitted).
	Reason string
}

// Controller gates job launches on host pressure and learned memory budgets.
// It complements (does not replace) the manager's static concurrency cap:
// when host signals are unavailable, everything is admitted and only the
// static cap applies. Callers must pair every admitted TryAdmit with a Release.
type Controller struct {
	cfg   Config
	probe func() Signals

	mu      sync.Mutex
	granted map[string]uint64
	logger  *log.Entry
}

func NewController(cfg Config) *Controller {
	return &Controller{
		cfg:     cfg.withDefaults(),
		probe:   Probe,
		granted: make(map[string]uint64),
		logger:  log.WithField("component", "admission"),
	}
}

// Estimate derives a job's memory budget from learned telemetry.
func Estimate(peakMemEWMA uint64) uint64 {
	if peakMemEWMA == 0 {
		return DefaultMemEstimate
	}
	return uint64(float64(peakMemEWMA) * EstimateHeadroom)
}

// TryAdmit decides whether the job may start now. An admitted job holds its
// budget until Release(name) is called.
func (c *Controller) TryAdmit(req Request) Verdict {
	c.mu.Lock()
	defer c.mu.Unlock()

	if req.MemEstimate == 0 {
		req.MemEstimate = DefaultMemEstimate
	}
	verdict := c.decide(req)
	if verdict.Admit {
		c.granted[req.Name] = req.MemEstimate
	}
	c.logger.WithFields(log.Fields{
		"event":    "admission_verdict",
		"job":      req.Name,
		"estimate": req.MemEstimate,
		"admit":    verdict.Admit,
		"reason":   verdict.Reason,
		"running":  len(c.granted),
	}).Info("admission verdict")
	return verdict
}

func (c *Controller) decide(req Request) Verdict {
	signals := c.probe()
	if !signals.Valid {
		return Verdict{Admit: true}
	}
	if signals.MemPSI > c.cfg.MemPSIThreshold {
		return Verdict{Reason: "memory pressure too high"}
	}
	var grantedSum uint64
	for _, g := range c.granted {
		grantedSum += g
	}
	reserve := uint64(float64(signals.MemAvailableBytes) * c.cfg.MemReserveFraction)
	if grantedSum+req.MemEstimate+reserve > signals.MemAvailableBytes {
		return Verdict{Reason: "insufficient memory headroom"}
	}
	return Verdict{Admit: true}
}

// Release returns a job's granted budget to the pool.
func (c *Controller) Release(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.granted, name)
}
