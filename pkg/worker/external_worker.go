package worker

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/sjtug/lug/pkg/config"
)

// ExternalWorker is a stub worker which always returns
// {Idle: false, Result: true}.
type ExternalWorker struct {
	name   string
	logger *log.Entry
	cfg    config.RepoConfig
}

func NewExternalWorker(cfg config.RepoConfig) (*ExternalWorker, error) {
	rawName, ok := cfg["name"]
	if !ok {
		return nil, errors.New("Name is required for external worker")
	}
	name := rawName.(string)
	return &ExternalWorker{
		name:   name,
		logger: log.WithField("worker", name),
		cfg:    cfg,
	}, nil
}

func (ew *ExternalWorker) GetStatus() Status {
	return Status{
		Result:       true,
		LastFinished: time.Now(),
		Idle:         true,
		Stdout:       []string{},
		Stderr:       []string{},
	}
}

func (ew *ExternalWorker) RunSync() {
	// a for {} should not be used here since it occupies 100% CPU
	select {}
}

func (ew *ExternalWorker) TriggerSync() {
}

func (ew *ExternalWorker) GetConfig() config.RepoConfig {
	return ew.cfg
}
