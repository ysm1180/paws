package ui

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ysm1180/paws/internal/aws"
	"github.com/ysm1180/paws/internal/transfer"
)

type browserMode int

const (
	browserModeList browserMode = iota
	browserModePathBar
	browserModeDownloadPrompt
)

type fileBrowserState struct {
	instance    aws.Instance
	cwd         string
	entries     []aws.RemoteEntry
	cursor      int
	mode        browserMode
	pathInput   textinput.Model
	dlPathInput textinput.Model
	loadErr     error
	statusMsg   string
	pendingFile aws.RemoteEntry
}

func newFileBrowserState(inst aws.Instance, cwd string, entries []aws.RemoteEntry) *fileBrowserState {
	pi := textinput.New()
	pi.Placeholder = "/absolute/path"
	pi.Prompt = ""
	pi.CharLimit = 1024

	di := textinput.New()
	di.Prompt = ""
	di.CharLimit = 1024

	return &fileBrowserState{
		instance:    inst,
		cwd:         cwd,
		entries:     entries,
		mode:        browserModeList,
		pathInput:   pi,
		dlPathInput: di,
	}
}

func (s *fileBrowserState) applyListing(cwd string, entries []aws.RemoteEntry, err error) {
	s.cwd = cwd
	s.entries = entries
	s.cursor = 0
	s.loadErr = err
}

// handleBrowserKey processes keypresses while screen == screenBrowser.
func (m *Model) handleBrowserKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.browser
	if s == nil {
		return m, nil
	}

	switch s.mode {
	case browserModePathBar:
		if key.Matches(msg, Keys.Escape) {
			s.pathInput.Blur()
			s.mode = browserModeList
			return m, nil
		}
		if key.Matches(msg, Keys.Enter) {
			target := strings.TrimSpace(s.pathInput.Value())
			s.pathInput.Blur()
			s.mode = browserModeList
			if target != "" {
				return m, m.cdCmd(target)
			}
			return m, nil
		}
		var cmd tea.Cmd
		s.pathInput, cmd = s.pathInput.Update(msg)
		return m, cmd

	case browserModeDownloadPrompt:
		if key.Matches(msg, Keys.Escape) {
			s.dlPathInput.Blur()
			s.mode = browserModeList
			return m, nil
		}
		if key.Matches(msg, Keys.Enter) {
			localPath := strings.TrimSpace(s.dlPathInput.Value())
			s.dlPathInput.Blur()
			s.mode = browserModeList
			if localPath == "" {
				return m, nil
			}
			return m, m.startDownloadCmd(s.pendingFile, localPath)
		}
		var cmd tea.Cmd
		s.dlPathInput, cmd = s.dlPathInput.Update(msg)
		return m, cmd
	}

	// browserModeList
	switch {
	case key.Matches(msg, Keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, Keys.Escape):
		if j := m.tm.ActiveJob(s.instance.ID); j != nil {
			j.Cancel()
			s.statusMsg = "Transfer cancelled"
			// The cancel watchdog in StreamRemoteFileBase64 will close
			// the transfer shell's stdin to unblock its Peek; that
			// permanently poisons the session, so spin up a fresh one
			// in the background to keep subsequent downloads working.
			return m, m.reopenTransferShellCmd()
		}
		return m, func() tea.Msg { return browserClosedMsg{} }
	case key.Matches(msg, Keys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(msg, Keys.Down):
		if s.cursor < len(s.entries)-1 {
			s.cursor++
		}
	case key.Matches(msg, Keys.Back):
		parent := filepath.Dir(s.cwd)
		if parent == s.cwd {
			return m, nil
		}
		return m, m.cdCmd(parent)
	case key.Matches(msg, Keys.PathBar):
		s.pathInput.SetValue(s.cwd)
		s.pathInput.Focus()
		s.mode = browserModePathBar
		return m, textinput.Blink
	case key.Matches(msg, Keys.Enter):
		if s.cursor >= len(s.entries) {
			return m, nil
		}
		e := s.entries[s.cursor]
		if e.IsDir {
			return m, m.cdCmd(path.Join(s.cwd, e.Name))
		}
		s.pendingFile = e
		defaultDest := filepath.Join(m.config.GetDownloadDir(), s.instance.Name, e.Name)
		s.dlPathInput.SetValue(defaultDest)
		s.dlPathInput.Focus()
		s.mode = browserModeDownloadPrompt
		return m, textinput.Blink
	}
	return m, nil
}

// cdCmd issues `ls` for absPath.
func (m *Model) cdCmd(absPath string) tea.Cmd {
	target := absPath
	ctx := m.ctx
	browseSh := m.browseShell
	return func() tea.Msg {
		entries, err := aws.ListRemoteDir(ctx, browseSh, target)
		if err != nil {
			return browserListedMsg{cwd: target, err: err}
		}
		return browserListedMsg{cwd: target, entries: entries}
	}
}

// startDownloadCmd captures all shell pointers and IDs at Cmd creation time
// so the inner goroutine never reads Model fields concurrently with Update.
func (m *Model) startDownloadCmd(file aws.RemoteEntry, localPath string) tea.Cmd {
	if m.browser == nil {
		return nil
	}
	if m.transferShell == nil {
		return func() tea.Msg {
			return logMsg{"error", "transfer session reopening, retry in a moment"}
		}
	}
	remoteAbs := path.Join(m.browser.cwd, file.Name)
	instanceID := m.browser.instance.ID
	browseSh := m.browseShell
	transferSh := m.transferShell
	tm := m.tm
	ctx := m.ctx
	return func() tea.Msg {
		size, err := aws.StatRemoteFile(ctx, browseSh, remoteAbs)
		if err != nil {
			return logMsg{"error", fmt.Sprintf("stat: %v", err)}
		}
		_, err = tm.Start(ctx, transferSh, instanceID, remoteAbs, localPath, size,
			func(j *transfer.Job) {},
			func(j *transfer.Job) {},
		)
		if err != nil {
			return logMsg{"error", err.Error()}
		}
		return logMsg{"EC2", fmt.Sprintf("download started: %s (%d bytes)", remoteAbs, size)}
	}
}
