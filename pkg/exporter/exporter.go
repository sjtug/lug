// Package exporter provides definition of exporter
package exporter

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/sjtug/lug/pkg/helper"
)

// Exporter exports lug metrics to Prometheus. All operations are thread-safe
type Exporter struct {
	successCounter *prometheus.CounterVec
	failCounter    *prometheus.CounterVec
	diskUsage      *prometheus.GaugeVec
	peakMem        *prometheus.GaugeVec
	peakMemEWMA    *prometheus.GaugeVec
	duration       *prometheus.GaugeVec
	durationEWMA   *prometheus.GaugeVec
	oomKills       *prometheus.GaugeVec
	admissions     *prometheus.CounterVec
	// stores worker_name -> last time that updates its disk usage
	diskUsageLastUpdateTime map[string]time.Time
	// guard the exporter
	mutex sync.Mutex
}

var instance *Exporter

// newExporter creates a new exporter
func newExporter() *Exporter {
	newExporter := Exporter{
		successCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "success_sync",
				Help: "How many successful synchronizations processed, partitioned by workers.",
			},
			[]string{"worker"},
		),
		failCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fail_sync",
				Help: "How many failed synchronizations processed, partitioned by workers.",
			},
			[]string{"worker"},
		),
		diskUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug",
				Subsystem: "storage",
				Name:      "disk_usage",
				Help:      "Disk usage in bytes, partitioned by workers.",
			},
			[]string{"worker"},
		),
		peakMem: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug", Subsystem: "job", Name: "peak_mem_bytes",
				Help: "Peak memory (bytes) of the last sync run, partitioned by workers.",
			},
			[]string{"worker"},
		),
		peakMemEWMA: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug", Subsystem: "job", Name: "peak_mem_ewma_bytes",
				Help: "EWMA of peak memory (bytes) across sync runs, partitioned by workers.",
			},
			[]string{"worker"},
		),
		duration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug", Subsystem: "job", Name: "duration_seconds",
				Help: "Wall time (seconds) of the last sync run, partitioned by workers.",
			},
			[]string{"worker"},
		),
		durationEWMA: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug", Subsystem: "job", Name: "duration_ewma_seconds",
				Help: "EWMA of sync run wall time (seconds), partitioned by workers.",
			},
			[]string{"worker"},
		),
		oomKills: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lug", Subsystem: "job", Name: "oom_kills_total",
				Help: "Cumulative OOM kills observed for sync runs, partitioned by workers.",
			},
			[]string{"worker"},
		),
		admissions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lug", Subsystem: "admission", Name: "verdicts_total",
				Help: "Admission verdicts, partitioned by outcome (admitted/deferred).",
			},
			[]string{"outcome"},
		),
		diskUsageLastUpdateTime: map[string]time.Time{},
	}
	prometheus.MustRegister(newExporter.successCounter)
	prometheus.MustRegister(newExporter.failCounter)
	prometheus.MustRegister(newExporter.diskUsage)
	prometheus.MustRegister(newExporter.peakMem)
	prometheus.MustRegister(newExporter.peakMemEWMA)
	prometheus.MustRegister(newExporter.duration)
	prometheus.MustRegister(newExporter.durationEWMA)
	prometheus.MustRegister(newExporter.oomKills)
	prometheus.MustRegister(newExporter.admissions)
	log.Info("Exporter initialized")
	return &newExporter
}

// GetInstance gets the exporter
func GetInstance() *Exporter {
	if instance == nil {
		instance = newExporter()
	}
	return instance
}

// Expose the registered metrics via HTTP.
func Expose(addr string) {
	GetInstance() // ensure init
	http.Handle("/metrics", promhttp.Handler())
	log.Info("Metrics exposed at " + addr + "/metrics")
	log.Fatal(http.ListenAndServe(addr, nil))
}

// SyncSuccess will report a successful synchronization
func (e *Exporter) SyncSuccess(worker string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.successCounter.With(prometheus.Labels{"worker": worker}).Inc()
	e.failCounter.With(prometheus.Labels{"worker": worker}).Add(0)
}

// SyncFail will report a failed synchronization
func (e *Exporter) SyncFail(worker string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.failCounter.With(prometheus.Labels{"worker": worker}).Inc()
	e.successCounter.With(prometheus.Labels{"worker": worker}).Add(0)
}

// AdmissionVerdict reports one admission decision.
func (e *Exporter) AdmissionVerdict(admitted bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	outcome := "deferred"
	if admitted {
		outcome = "admitted"
	}
	e.admissions.With(prometheus.Labels{"outcome": outcome}).Inc()
}

// TelemetrySample carries per-run resource telemetry into the exporter.
// Defined here (not in pkg/worker) to avoid an import cycle.
type TelemetrySample struct {
	LastDuration     time.Duration
	DurationEWMA     time.Duration
	LastPeakMemBytes uint64
	PeakMemEWMABytes uint64
	OOMKills         uint64
}

// SyncTelemetry reports accumulated per-worker resource telemetry after a run.
func (e *Exporter) SyncTelemetry(workerName string, t TelemetrySample) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	labels := prometheus.Labels{"worker": workerName}
	if t.LastPeakMemBytes > 0 {
		e.peakMem.With(labels).Set(float64(t.LastPeakMemBytes))
		e.peakMemEWMA.With(labels).Set(float64(t.PeakMemEWMABytes))
	}
	if t.LastDuration > 0 {
		e.duration.With(labels).Set(t.LastDuration.Seconds())
		e.durationEWMA.With(labels).Set(t.DurationEWMA.Seconds())
	}
	e.oomKills.With(labels).Set(float64(t.OOMKills))
}

// need at least 1min to rescan disk
const updateDiskUsageThrottle time.Duration = time.Minute

// UpdateDiskUsage will update the disk usage of a directory.
// This call is asynchronous at rate-limited per worker
func (e *Exporter) UpdateDiskUsage(worker string, path string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	lastUpdateTime, found := e.diskUsageLastUpdateTime[worker]
	logger := log.WithFields(log.Fields{
		"worker": worker,
		"path":   path,
	})
	logger.WithField("event", "update_disk_usage").Info("Invoke UpdateDiskUsage")
	if !found || time.Since(lastUpdateTime) > updateDiskUsageThrottle {
		logger.Debug("background update_disk_usage launched")
		// first, we set it to infinity (2037-01-01)
		// Note that we cannot use a larger value due to Y2038 problem on *nix
		e.diskUsageLastUpdateTime[worker] = time.Date(
			2037, 1, 1, 0, 0, 0, 0, time.Local)
		// then, we perform the operation in background
		go func() {
			size, err := helper.DiskUsage(path)
			// the above step is time-consuming, so acquire the lock after it completes
			e.mutex.Lock()
			defer e.mutex.Unlock()
			if err == nil {
				e.diskUsage.With(prometheus.Labels{"worker": worker}).Set(float64(size))
			}
			// when it finishes, we set it to actual finishing time
			e.diskUsageLastUpdateTime[worker] = time.Now()
			logger.WithField(
				"event", "update_disk_usage_complete").WithField("size", size).Info("Disk usage updated")
		}()
	}
}
