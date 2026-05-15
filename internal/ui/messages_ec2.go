package ui

import (
	"github.com/ysm1180/paws/internal/aws"
	"github.com/ysm1180/paws/internal/transfer"
)

// screen represents which top-level UI mode is active.
type screen int

const (
	screenList screen = iota
	screenBrowser
)

// browserOpenedMsg carries the two new SSM sessions to the Update goroutine
// so the Model never mutates pointer fields from a tea.Cmd goroutine.
type browserOpenedMsg struct {
	instance aws.Instance
	browseSh *aws.ShellSession
	transSh  *aws.ShellSession
	cwd      string
	entries  []aws.RemoteEntry
}

// browserOpenFailedMsg is returned when StartShellSession or the initial ls
// fails. Any sessions already opened are returned so Update can close them.
type browserOpenFailedMsg struct {
	browseSh *aws.ShellSession
	transSh  *aws.ShellSession
	err      string
}

type browserListedMsg struct {
	cwd     string
	entries []aws.RemoteEntry
	err     error
}

type transferProgressMsg struct {
	job *transfer.Job
}

type transferDoneMsg struct {
	job *transfer.Job
}

type browserClosedMsg struct{}

// transferShellReopenedMsg delivers a freshly opened transfer ShellSession
// back to Update after a cancel. The cancel watchdog in StreamRemoteFileBase64
// closes stdin to unblock a stuck Peek, which permanently poisons that
// session — without a reopen path, every download after the first cancel
// fails with "shell session closed".
type transferShellReopenedMsg struct {
	sh  *aws.ShellSession
	err error
}

type tickMsg struct{}
