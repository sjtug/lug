// Package exporter provides definition of exporter
package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/helper"
	"net/http"
	"sync"
	"time"
)

// Exporter exports lug metrics to Prometheus. All operations are thread-safe
type Exporter struct {
	successCounter *prometheus.CounterVec
	failCounter    *prometheus.CounterVec
	diskUsage      *prometheus.GaugeVec
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
		diskUsageLastUpdateTime: map[string]time.Time{},
	}
	prometheus.MustRegister(newExporter.successCounter)
	prometheus.MustRegister(newExporter.failCounter)
	prometheus.MustRegister(newExporter.diskUsage)
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
}

// SyncFail will report a failed synchronization
func (e *Exporter) SyncFail(worker string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.failCounter.With(prometheus.Labels{"worker": worker}).Inc()
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
	if !found || time.Now().Sub(lastUpdateTime) > updateDiskUsageThrottle {
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
