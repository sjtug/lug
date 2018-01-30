// Package exporter provides definition of exporter
package exporter

import (
	"net/http"
	"github.com/sjtug/lug/helper"
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter exports lug metrics to Prometheus
type Exporter struct {
	successCounter *prometheus.CounterVec
	failCounter *prometheus.CounterVec
	diskUsage *prometheus.GaugeVec
}

var instance *Exporter

// newExporter creates a new exporter
func newExporter() *Exporter {
	newExporter := Exporter {
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
	log.Info("Metrics exposed")
	log.Fatal(http.ListenAndServe(addr, nil))
}

// SyncSuccess will report a successful synchronization
func (e *Exporter) SyncSuccess(worker string) {
	e.successCounter.With(prometheus.Labels{"worker": worker}).Inc()
}

// SyncFail will report a failed synchronization
func (e *Exporter) SyncFail(worker string) {
	e.failCounter.With(prometheus.Labels{"worker": worker}).Inc()
}

// UpdateDiskUsage will update the disk usage of a directory
func (e *Exporter) UpdateDiskUsage(worker string, path string) {
	size, err := helper.DiskUsage(path)
	if err == nil {
		e.diskUsage.With(prometheus.Labels{"worker": worker}).Set(float64(size))
	}
}