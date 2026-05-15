package transfer

import (
	"testing"
)

func TestJob_LifecycleHappyPath(t *testing.T) {
	j := NewJob("i-1", "/var/log/app.log", "/tmp/app.log", 1000)
	if j.Status != StatusPending {
		t.Fatalf("initial status: %v", j.Status)
	}
	j.MarkRunning()
	if j.Status != StatusRunning {
		t.Fatalf("after MarkRunning: %v", j.Status)
	}
	j.AddBytes(400)
	j.AddBytes(600)
	if j.Transferred != 1000 {
		t.Fatalf("transferred: %d", j.Transferred)
	}
	if j.PercentDone() != 100 {
		t.Fatalf("percent: %d", j.PercentDone())
	}
	j.MarkDone()
	if j.Status != StatusDone {
		t.Fatalf("final: %v", j.Status)
	}
}

func TestJob_CancelMarksCancelled(t *testing.T) {
	j := NewJob("i-1", "/a", "/b", 100)
	j.MarkRunning()
	j.Cancel()
	if j.Status != StatusCancelled {
		t.Fatalf("status: %v", j.Status)
	}
}

func TestJob_FailureSetsError(t *testing.T) {
	j := NewJob("i-1", "/a", "/b", 100)
	j.MarkRunning()
	j.Fail(testErr{"net down"})
	if j.Status != StatusFailed {
		t.Fatalf("status: %v", j.Status)
	}
	if j.Err == nil || j.Err.Error() != "net down" {
		t.Fatalf("err: %v", j.Err)
	}
}

type testErr struct{ s string }

func (e testErr) Error() string { return e.s }
