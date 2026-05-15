package transfer

import (
	"context"
	"sync"
	"time"
)

type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusDone
	StatusCancelled
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusCancelled:
		return "cancelled"
	case StatusFailed:
		return "failed"
	}
	return "unknown"
}

type Job struct {
	ID          string
	InstanceID  string
	RemotePath  string
	LocalPath   string
	Expected    int64
	Transferred int64
	StartedAt   time.Time
	EndedAt     time.Time
	Status      Status
	Err         error
	cancel      context.CancelFunc
	mu          sync.Mutex
}

func NewJob(instanceID, remotePath, localPath string, expected int64) *Job {
	return &Job{
		ID:         instanceID + ":" + remotePath,
		InstanceID: instanceID,
		RemotePath: remotePath,
		LocalPath:  localPath,
		Expected:   expected,
		Status:     StatusPending,
	}
}

func (j *Job) MarkRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusRunning
	j.StartedAt = time.Now()
}

func (j *Job) AddBytes(n int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Transferred += n
}

func (j *Job) SetTransferred(n int64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Transferred = n
}

func (j *Job) MarkDone() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusDone
	j.EndedAt = time.Now()
}

func (j *Job) Fail(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusFailed
	j.Err = err
	j.EndedAt = time.Now()
}

func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		j.cancel()
	}
	j.Status = StatusCancelled
	j.EndedAt = time.Now()
}

func (j *Job) SetCancel(c context.CancelFunc) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cancel = c
}

func (j *Job) PercentDone() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Expected <= 0 {
		return 0
	}
	p := int(j.Transferred * 100 / j.Expected)
	if p > 100 {
		p = 100
	}
	return p
}

func (j *Job) Speed() float64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	elapsed := time.Since(j.StartedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(j.Transferred) / elapsed
}
