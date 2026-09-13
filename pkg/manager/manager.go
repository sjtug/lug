// Package manager provides definition of manager
package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/sirupsen/logrus"

	"github.com/sjtug/lug/pkg/admission"
	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/exporter"
	"github.com/sjtug/lug/pkg/worker"
)

const (
	// SigStart is a signal sent to control channel of manager which starts sync of all container
	SigStart = iota
	// SigStop is a signal sent to control channel of manager which stops sync of all container
	SigStop
	// SigExit is a signal sent to control channel of manager which exits manager run loop
	SigExit
	// ExitFinish is a signal from finish channel of manager indicating exit finished
	ExitFinish
	// StopFinish is a signal from finish channel of manager indicating stopping job finished
	StopFinish
	// StartFinish is a signal from finish channel of manager indicating starting job finished
	StartFinish
)

// syncRequest asks the Run() goroutine to enqueue a manual sync.
type syncRequest struct {
	name     string
	resultCh chan error
}

// queueRequest asks for a snapshot of the task queue.
type queueRequest struct {
	resultCh chan QueueStatus
}

// QueueStatus is the response from GetQueueStatus.
type QueueStatus struct {
	Running []RunningEntry `json:"running"`
	Pending []string       `json:"pending"`
}

// RunningEntry describes a currently syncing worker.
type RunningEntry struct {
	Name      string                `json:"name"`
	ActiveJob *worker.ActiveJobInfo `json:"active_job,omitempty"`
}

// Manager holds worker instances
type Manager struct {
	config                *config.Config
	workers               []worker.Worker
	workersLastInvokeTime map[string]time.Time
	controlChan           chan int
	finishChan            chan int
	running               bool
	// storing index of worker to launch
	pendingQueue []int
	// admission gates launches on host pressure; nil when disabled
	admission *admission.Controller
	// hasGrant tracks workers holding an admission grant, so the poll loop
	// can release budgets once a worker returns to idle
	hasGrant map[string]bool
	// syncReqChan carries manual-sync requests into the Run() goroutine.
	syncReqChan chan syncRequest
	// queueReqChan carries queue-inspection requests into the Run() goroutine.
	queueReqChan chan queueRequest
	logger       *logrus.Entry
}

// Status holds the status of a manager and its workers
// WorkerStatus: key = worker's name, value = worker's status
type Status struct {
	Running      bool
	WorkerStatus map[string]worker.Status
}

type WorkerCheckPoint struct {
	LastInvokeTime time.Time  `json:"last_invoke_time"`
	LastFinished   *time.Time `json:"last_finished,omitempty"`
	Result         *bool      `json:"result,omitempty"`
	// Telemetry persists learned resource usage (peak mem / duration EWMAs)
	// across restarts. Additive; absent in v1 checkpoints.
	Telemetry *worker.Telemetry `json:"telemetry,omitempty"`
}

type CheckPoint struct {
	WorkerInfo map[string]WorkerCheckPoint `json:"worker_info"`
}

// fromCheckpoint laods last invoke time from json
func fromCheckpoint(checkpointFile string) (*CheckPoint, error) {
	jsonFile, err := os.Open(checkpointFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = jsonFile.Close() }()

	data, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var checkpoint CheckPoint

	err = json.Unmarshal(data, &checkpoint)
	if err != nil {
		return nil, err
	}

	return &checkpoint, nil
}

func workerFromCheckpoint(repoConfig config.RepoConfig, checkpoint *CheckPoint, name string, lastInvokeTime time.Time) (worker.Worker, error) {
	if checkpoint == nil {
		return worker.NewWorker(repoConfig, lastInvokeTime, true)
	}
	info, ok := checkpoint.WorkerInfo[name]
	if !ok {
		return worker.NewWorker(repoConfig, lastInvokeTime, true)
	}

	result := true
	if info.Result != nil {
		result = *info.Result
	}
	lastFinished := lastInvokeTime
	if info.LastFinished != nil {
		lastFinished = *info.LastFinished
	}
	telemetry := worker.Telemetry{}
	if info.Telemetry != nil {
		telemetry = *info.Telemetry
	}
	return worker.NewWorkerWithTelemetry(repoConfig, lastFinished, result, telemetry)
}

// NewManager creates a new manager with attached workers from config
func NewManager(config *config.Config) (*Manager, error) {
	logger := logrus.WithField("manager", "")
	checkpoint, err := fromCheckpoint(config.Checkpoint)
	workersLastInvokeTime := make(map[string]time.Time)
	if err != nil {
		logger.Info("failed to parse checkpoint file")
	} else {
		for name, info := range checkpoint.WorkerInfo {
			workersLastInvokeTime[name] = info.LastInvokeTime
		}
	}
	var admissionCtl *admission.Controller
	if config.Admission.Enabled {
		admissionCtl = admission.NewController(admission.Config{
			MemReserveFraction: config.Admission.MemReserveFraction,
			MemPSIThreshold:    config.Admission.MemPSIThreshold,
		})
	}
	newManager := Manager{
		config:                config,
		workers:               []worker.Worker{},
		workersLastInvokeTime: workersLastInvokeTime,
		controlChan:           make(chan int),
		finishChan:            make(chan int),
		running:               true,
		admission:             admissionCtl,
		hasGrant:              make(map[string]bool),
		syncReqChan:           make(chan syncRequest),
		queueReqChan:          make(chan queueRequest),
		logger:                logger,
	}
	for _, repoConfig := range config.Repos {
		if disabled, ok := repoConfig["disabled"].(bool); ok && disabled {
			continue
		}
		name, _ := repoConfig["name"].(string)
		if _, ok := newManager.workersLastInvokeTime[name]; !ok {
			newManager.workersLastInvokeTime[name] = time.Now().AddDate(-1, 0, 0)
		}
		w, err := workerFromCheckpoint(repoConfig, checkpoint, name, newManager.workersLastInvokeTime[name])
		if err != nil {
			return nil, err
		}
		newManager.workers = append(newManager.workers, w)
	}
	return &newManager, nil
}

func (m *Manager) checkpoint() error {
	if m.config.Checkpoint == "" {
		// checkpointing disabled (e.g. tests); learned telemetry only lives in memory
		return nil
	}
	ckptObj := &CheckPoint{WorkerInfo: make(map[string]WorkerCheckPoint)}
	for _, w := range m.workers {
		name := w.GetConfig()["name"].(string)
		status := w.GetStatus()
		lastInvokeTime, ok := m.workersLastInvokeTime[name]
		if !ok {
			lastInvokeTime = time.Now().AddDate(-1, 0, 0)
		}

		telemetry := status.Telemetry
		ckptObj.WorkerInfo[name] = WorkerCheckPoint{
			LastInvokeTime: lastInvokeTime,
			Result:         &status.Result,
			LastFinished:   &status.LastFinished,
			Telemetry:      &telemetry,
		}
	}

	file, err := json.MarshalIndent(ckptObj, "", "  ")
	if err != nil {
		return err
	}
	ckpt := fmt.Sprintf("%s.tmp", m.config.Checkpoint)
	err = os.WriteFile(ckpt, file, 0644)
	if err != nil {
		return err
	}
	err = os.Rename(ckpt, m.config.Checkpoint)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) isAlreadyInPendingQueue(workerIdx int) bool {
	for _, wk := range m.pendingQueue {
		if wk == workerIdx {
			return true
		}
	}
	return false
}

// launchWorkerFromPendingQueue starts queued workers in FIFO order.
// max_allowed is the static concurrent_limit gate; when admission control is
// enabled, each launch must also pass the controller, which checks host
// pressure and learned memory budgets. On the first deferral the drain stops
// (FIFO, no overtaking), and the remaining workers retry next poll tick.
func (m *Manager) launchWorkerFromPendingQueue(max_allowed int) {
	if max_allowed <= 0 {
		return
	}
	m.logger.WithFields(logrus.Fields{
		"event":         "launch_worker_from_pending_queue",
		"max_allowed":   max_allowed,
		"pending_queue": spew.Sprint(m.pendingQueue),
	}).Debug("launch worker from pending queue")

	launched := 0
	for launched < max_allowed && len(m.pendingQueue) > 0 {
		w_idx := m.pendingQueue[0]
		w := m.workers[w_idx]
		wConfig := w.GetConfig()
		name := wConfig["name"].(string)

		if m.admission != nil {
			verdict := m.admission.TryAdmit(admission.Request{
				Name:        name,
				MemEstimate: admission.Estimate(w.GetStatus().Telemetry.PeakMemEWMA),
			})
			exporter.GetInstance().AdmissionVerdict(verdict.Admit)
			if !verdict.Admit {
				m.logger.WithFields(logrus.Fields{
					"event":              "admission_deferred",
					"target_worker_name": name,
					"reason":             verdict.Reason,
				}).Infof("admission deferred for worker %s: %s", name, verdict.Reason)
				break
			}
			m.hasGrant[name] = true
		}

		m.pendingQueue = m.pendingQueue[1:]
		m.logger.WithFields(logrus.Fields{
			"event":              "trigger_sync",
			"target_worker_name": name,
		}).Infof("trigger sync for worker %s from pendingQueue", name)
		m.workersLastInvokeTime[name] = time.Now()
		w.TriggerSync()
		launched++
	}
}

// Run will block current routine
func (m *Manager) Run() {
	m.logger.Debugf("%p", m)
	c := time.Tick(time.Duration(m.config.Interval) * time.Second)
	for _, w := range m.workers {
		m.logger.WithFields(logrus.Fields{
			"event":         "call_runsync",
			"target_worker": w.GetConfig()["name"],
		}).Debugf("Calling RunSync() to w %s", w.GetConfig()["name"])
		go w.RunSync()
	}
	err := m.checkpoint()
	if err != nil {
		m.logger.WithFields(logrus.Fields{
			"event": "checkpoint_failed",
			"error": err,
		}).Error("Failed to checkpoint")
	}
	for {
		// wait until config.Interval seconds has elapsed
		select {
		case <-c:
			if m.running {
				m.logger.WithField("event", "poll_start").Info("Start polling workers")
				running_worker_cnt := 0
				shouldCheckpoint := false
				for i, w := range m.workers {
					wStatus := w.GetStatus()
					m.logger.WithFields(logrus.Fields{
						"event":                       "worker_status",
						"target_worker_idx":           i,
						"target_worker_idle":          wStatus.Idle,
						"target_worker_result":        wStatus.Result,
						"target_worker_last_finished": wStatus.LastFinished,
					})
					if !wStatus.Idle {
						running_worker_cnt++
						continue
					}
					wConfig := w.GetConfig()
					if name, ok := wConfig["name"].(string); ok && m.hasGrant[name] {
						if m.admission != nil {
							m.admission.Release(name)
						}
						delete(m.hasGrant, name)
					}
					elapsed := time.Since(m.workersLastInvokeTime[wConfig["name"].(string)])
					sec2sync, ok := wConfig["interval"].(int)
					if !ok {
						sec2sync = 31536000 // if "interval" is not specified, then worker will launch once a year
					}
					if !m.isAlreadyInPendingQueue(i) && elapsed > time.Duration(sec2sync)*time.Second {
						m.logger.WithFields(logrus.Fields{
							"event":                  "trigger_pending",
							"target_worker_name":     wConfig["name"],
							"target_worker_interval": sec2sync,
						}).Infof("Interval of w %s (%d sec) elapsed, send it to pendingQueue", wConfig["name"], sec2sync)
						m.pendingQueue = append(m.pendingQueue, i)
						shouldCheckpoint = true
					}
				}
				m.launchWorkerFromPendingQueue(m.config.ConcurrentLimit - running_worker_cnt)
				m.logger.WithField("event", "poll_end").Info("Stop polling workers")

				// Here we do not checkpoint very concisely (e.g. every time after a successful sync).
				// We just want to minimize re-sync after restarting lug.
				if shouldCheckpoint {
					err := m.checkpoint()
					if err != nil {
						m.logger.WithFields(logrus.Fields{
							"event": "checkpoint_failed",
							"error": err,
						}).Error("Failed to checkpoint")
					}
				}
			}
		case req := <-m.syncReqChan:
			req.resultCh <- m.handleSyncRequest(req.name)
		case req := <-m.queueReqChan:
			req.resultCh <- m.buildQueueStatus()
		case sig, ok := <-m.controlChan:
			if ok {
				switch sig {
				default:
					m.logger.WithField("event", "unrecognized_control_signal").
						Warningf("Unrecognized Control Signal: %d", sig)
				case SigStart:
					m.running = true
					m.finishChan <- StartFinish
				case SigStop:
					m.running = false
					m.finishChan <- StopFinish
				case SigExit:
					m.logger.WithField("event", "exit_control_signal").Info("Exiting...")
					goto END_OF_FINISH
				}
			} else {
				m.logger.WithField("event", "control_channel_closed").Fatal("Control channel is closed!")
			}
		}
	}
END_OF_FINISH:
	m.logger.WithField("event", "send_exit_finish").Debug("Sending ExitFinish...")
	m.finishChan <- ExitFinish
	m.logger.WithField("event", "senf_exit_finish_end").Debug("Finished sending ExitFinish...")
}

func (m *Manager) expectChanVal(ch chan int, expected int) {
	exitMsg, ok := <-ch
	if ok {
		switch exitMsg {
		default:
			m.logger.WithFields(logrus.Fields{
				"event":        "unexpected_control_message",
				"expected_msg": expected,
				"received_msg": exitMsg,
			}).Fatalf("Unrecognized Msg: %d, expected %d", exitMsg, expected)
		case expected:
			m.logger.WithFields(logrus.Fields{
				"event":        "finish_receive_control_message",
				"expected_msg": expected,
				"received_msg": expected,
			}).Infof("Finished reading %d", expected)
		}
	} else {
		m.logger.WithField("event", "control_channel_closed").Fatalf("Channel has been closed, expected %d", expected)
	}
}

// Start polling, block until finish(may take several seconds)
func (m *Manager) Start() {
	m.controlChan <- SigStart
	m.expectChanVal(m.finishChan, StartFinish)
}

// Stop polling, block until finish(may take several seconds)
func (m *Manager) Stop() {
	m.controlChan <- SigStop
	m.expectChanVal(m.finishChan, StopFinish)
}

// Exit polling, block until finish(may take several seconds)
func (m *Manager) Exit() {
	m.Stop()
	m.controlChan <- SigExit
	m.expectChanVal(m.finishChan, ExitFinish)
}

// AbortWorker cancels the in-flight sync of the named worker, if any.
// The cancellation propagates through the executor context and kills the
// whole job process tree when cgroup isolation is active.
func (m *Manager) AbortWorker(name string) error {
	for _, w := range m.workers {
		if w.GetConfig()["name"] != name {
			continue
		}
		aborter, ok := w.(worker.Aborter)
		if !ok {
			return fmt.Errorf("worker %s does not support aborting", name)
		}
		m.logger.WithFields(logrus.Fields{
			"event":              "abort_worker",
			"target_worker_name": name,
		}).Infof("aborting sync of worker %s", name)
		aborter.AbortSync()
		return nil
	}
	return fmt.Errorf("no worker named %s", name)
}

// TriggerWorkerSync inserts the named worker at the head of the pending
// queue (or lifts it there if already queued). The scheduler launches it
// immediately if capacity allows.
func (m *Manager) TriggerWorkerSync(name string) error {
	req := syncRequest{name: name, resultCh: make(chan error, 1)}
	m.syncReqChan <- req
	return <-req.resultCh
}

// GetQueueStatus returns a snapshot of running and pending workers.
func (m *Manager) GetQueueStatus() QueueStatus {
	req := queueRequest{resultCh: make(chan QueueStatus, 1)}
	m.queueReqChan <- req
	return <-req.resultCh
}

// handleSyncRequest runs inside the Run() goroutine with exclusive queue
// access. It validates, enqueues, and immediately tries to launch.
func (m *Manager) handleSyncRequest(name string) error {
	idx := m.findWorkerIdx(name)
	if idx < 0 {
		return fmt.Errorf("no worker named %s", name)
	}
	w := m.workers[idx]
	if !w.GetStatus().Idle {
		return fmt.Errorf("worker %s is currently syncing", name)
	}
	m.removeFromPendingQueue(idx)
	m.pendingQueue = append([]int{idx}, m.pendingQueue...)
	m.logger.WithFields(logrus.Fields{
		"event":              "manual_sync",
		"target_worker_name": name,
	}).Infof("manual sync requested for worker %s", name)
	// Try to launch immediately instead of waiting for the next tick.
	running := m.countRunning()
	m.launchWorkerFromPendingQueue(m.config.ConcurrentLimit - running)
	return nil
}

// buildQueueStatus runs inside the Run() goroutine.
func (m *Manager) buildQueueStatus() QueueStatus {
	qs := QueueStatus{Running: []RunningEntry{}, Pending: []string{}}
	for _, w := range m.workers {
		status := w.GetStatus()
		name := w.GetConfig()["name"].(string)
		if !status.Idle {
			qs.Running = append(qs.Running, RunningEntry{
				Name:      name,
				ActiveJob: status.ActiveJob,
			})
		}
	}
	for _, idx := range m.pendingQueue {
		name := m.workers[idx].GetConfig()["name"].(string)
		qs.Pending = append(qs.Pending, name)
	}
	return qs
}

func (m *Manager) findWorkerIdx(name string) int {
	for i, w := range m.workers {
		if w.GetConfig()["name"] == name {
			return i
		}
	}
	return -1
}

func (m *Manager) removeFromPendingQueue(idx int) {
	for i, qIdx := range m.pendingQueue {
		if qIdx == idx {
			m.pendingQueue = append(m.pendingQueue[:i], m.pendingQueue[i+1:]...)
			return
		}
	}
}

func (m *Manager) countRunning() int {
	count := 0
	for _, w := range m.workers {
		if !w.GetStatus().Idle {
			count++
		}
	}
	return count
}

// GetStatus gets status of Manager
func (m *Manager) GetStatus() *Status {
	status := Status{
		Running:      m.running,
		WorkerStatus: make(map[string]worker.Status),
	}
	for _, w := range m.workers {
		wConfig := w.GetConfig()
		wStatus := w.GetStatus()
		if hidden, ok := wConfig["hidden"].(bool); !ok || !hidden {
			status.WorkerStatus[wConfig["name"].(string)] = wStatus
		}
	}
	return &status
}
