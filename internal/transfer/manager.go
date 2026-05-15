package transfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	pawsaws "github.com/ysm1180/paws/internal/aws"
)

// Manager owns one active Job per EC2 instance.
type Manager struct {
	mu   sync.Mutex
	jobs map[string]*Job

	streamFn func(ctx context.Context, sh *pawsaws.ShellSession, remotePath string, w io.Writer, onProgress pawsaws.ProgressFunc) error
}

func NewManager() *Manager {
	return &Manager{
		jobs:     make(map[string]*Job),
		streamFn: pawsaws.StreamRemoteFileBase64,
	}
}

func (m *Manager) ActiveJob(instanceID string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[instanceID]
	if !ok {
		return nil
	}
	if j.Status == StatusRunning || j.Status == StatusPending {
		return j
	}
	return nil
}

func (m *Manager) LastJob(instanceID string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[instanceID]
}

// Start begins a download. transferSh is a dedicated SSM ShellSession the
// caller has prepared; Manager does not own it.
func (m *Manager) Start(
	ctx context.Context,
	transferSh *pawsaws.ShellSession,
	instanceID, remotePath, localPath string,
	expected int64,
	onProgress func(*Job),
	onDone func(*Job),
) (*Job, error) {
	m.mu.Lock()
	if existing, ok := m.jobs[instanceID]; ok &&
		(existing.Status == StatusRunning || existing.Status == StatusPending) {
		m.mu.Unlock()
		return nil, fmt.Errorf("transfer already in progress for %s", instanceID)
	}
	job := NewJob(instanceID, remotePath, localPath, expected)
	m.jobs[instanceID] = job
	m.mu.Unlock()

	jobCtx, cancel := context.WithCancel(ctx)
	job.SetCancel(cancel)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		job.Fail(err)
		if onDone != nil {
			onDone(job)
		}
		return job, nil
	}
	partial := localPath + ".partial"
	f, err := os.Create(partial)
	if err != nil {
		job.Fail(err)
		if onDone != nil {
			onDone(job)
		}
		return job, nil
	}

	go func() {
		defer f.Close()
		job.MarkRunning()
		err := m.streamFn(jobCtx, transferSh, remotePath, f, func(n int64) {
			job.SetTransferred(n)
			if onProgress != nil {
				onProgress(job)
			}
		})
		f.Close()
		if err != nil {
			if jobCtx.Err() != nil {
				_ = os.Remove(partial)
				job.Cancel()
			} else {
				_ = os.Remove(partial)
				job.Fail(err)
			}
			if onDone != nil {
				onDone(job)
			}
			return
		}
		st, statErr := os.Stat(partial)
		if statErr != nil {
			_ = os.Remove(partial)
			job.Fail(statErr)
		} else if st.Size() != expected {
			_ = os.Remove(partial)
			job.Fail(fmt.Errorf("integrity: got %d bytes, expected %d", st.Size(), expected))
		} else if err := os.Rename(partial, localPath); err != nil {
			job.Fail(err)
		} else {
			job.MarkDone()
		}
		if onDone != nil {
			onDone(job)
		}
	}()

	return job, nil
}
