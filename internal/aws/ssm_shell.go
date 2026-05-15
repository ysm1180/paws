package aws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// lockedBuffer guards bytes.Buffer for concurrent writes. exec.Cmd drains
// stderr from an internal goroutine, so a bare bytes.Buffer would race
// with reads from the UI thread.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ShellSession represents one interactive SSM shell session running
// session-manager-plugin as a subprocess. All access is serialized by mu;
// callers requiring concurrency must use separate sessions.
//
// Invariant: reader is the SINGLE owner of stdout for the lifetime of the
// session. Both Run and StreamRemoteFileBase64 read through reader. Creating
// any other bufio reader over the same stdout will silently drop bytes that
// the existing reader has buffered ahead.
type ShellSession struct {
	target    string
	sessID    string
	region    string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdoutR   io.ReadCloser
	reader    *bufio.Reader
	stderrBuf *lockedBuffer
	mu        sync.Mutex
	closed    atomic.Bool
	nonceFn   func() string
}

// Stderr returns what session-manager-plugin has written to stderr so far.
// The plugin logs a descriptive reason there right before its stdin pipe
// closes (session-limit, agent ping failure, IAM denial, etc.), making it
// the most useful diagnostic for "broken pipe" failures.
func (s *ShellSession) Stderr() string {
	if s.stderrBuf == nil {
		return ""
	}
	return s.stderrBuf.String()
}

func defaultNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var ansiSeqRE = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

func stripANSI(s string) string { return ansiSeqRE.ReplaceAllString(s, "") }

// Run sends a single command bracketed by BEGIN/EOF sentinels and returns
// the payload between them. It reads synchronously from the calling goroutine
// to avoid the classic "cancelled goroutine drains the next caller's output"
// bug. On ctx cancellation Run sends Ctrl+C and waits up to 5s for the EOF
// sentinel; on timeout the session is marked closed and stdin is shut.
func (s *ShellSession) Run(ctx context.Context, cmdLine string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return "", fmt.Errorf("shell session closed")
	}
	if s.nonceFn == nil {
		s.nonceFn = defaultNonce
	}
	nonce := s.nonceFn()
	begin := "__PAWS_BEGIN_" + nonce + "__"
	end := "__PAWS_EOF_" + nonce + "__"
	full := fmt.Sprintf("echo %s; %s; echo %s\n", begin, cmdLine, end)
	if _, err := io.WriteString(s.stdin, full); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	var ctxErr atomic.Value
	watchdogDone := make(chan struct{})
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				ctxErr.Store(ctx.Err())
				_, _ = s.stdin.Write([]byte{0x03})
				select {
				case <-watchdogDone:
				case <-time.After(5 * time.Second):
					s.closed.Store(true)
					_ = s.stdin.Close()
				}
			case <-watchdogDone:
			}
		}()
	}
	defer close(watchdogDone)

	var out strings.Builder
	inPayload := false
	for {
		line, err := s.reader.ReadString('\n')
		if line != "" {
			clean := strings.TrimSpace(stripANSI(line))
			// Only scan for the end sentinel AFTER inPayload: the very first
			// line is the shell echoing the wrapped command itself and
			// contains both sentinels as substrings.
			endIdx := -1
			if inPayload {
				endIdx = strings.Index(clean, end)
			}
			switch {
			case !inPayload && clean == begin:
				inPayload = true
			case endIdx >= 0:
				// endIdx > 0 means a command emitted payload without a
				// trailing newline (e.g. `printf %s`) so EOF landed on the
				// same line; recover the payload prefix.
				if endIdx > 0 {
					out.WriteString(clean[:endIdx])
					out.WriteByte('\n')
				}
				if v := ctxErr.Load(); v != nil {
					return out.String(), v.(error)
				}
				return out.String(), nil
			case inPayload:
				out.WriteString(clean)
				out.WriteByte('\n')
			}
		}
		if err != nil {
			if v := ctxErr.Load(); v != nil {
				return out.String(), v.(error)
			}
			return out.String(), err
		}
	}
}

func (s *ShellSession) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.stdoutR != nil {
		_ = s.stdoutR.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

func (s *ShellSession) SessionID() string { return s.sessID }

func (c *Client) StartShellSession(ctx context.Context, instanceID string) (*ShellSession, error) {
	resp, err := c.SSM.StartSession(ctx, &ssm.StartSessionInput{
		Target: awsv2.String(instanceID),
	})
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	sessionConfig := map[string]interface{}{
		"SessionId":        resp.SessionId,
		"StreamUrl":        resp.StreamUrl,
		"TokenValue":       resp.TokenValue,
		"ResponseMetadata": map[string]interface{}{},
	}
	configJSON, _ := json.Marshal(sessionConfig)
	targetJSON, _ := json.Marshal(map[string]string{"Target": instanceID})

	pluginPath := pluginBinaryPath()
	// exec.Command (NOT CommandContext): the subprocess must outlive the
	// caller's ctx. Callers pass short-lived timeout ctxs for the prelude
	// Run; binding the subprocess to one would kill the plugin the moment
	// the ctx's defer cancel() fires, breaking every subsequent Run with
	// "broken pipe". Termination is now driven by ShellSession.Close().
	cmd := exec.Command(pluginPath,
		string(configJSON),
		c.Region,
		"StartSession",
		"",
		string(targetJSON),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", c.Region),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrBuf := &lockedBuffer{}
	cmd.Stderr = stderrBuf
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin start: %w", err)
	}

	sh := &ShellSession{
		target:    instanceID,
		sessID:    *resp.SessionId,
		region:    c.Region,
		cmd:       cmd,
		stdin:     stdin,
		stdoutR:   stdout,
		reader:    bufio.NewReaderSize(stdout, 64*1024),
		stderrBuf: stderrBuf,
		nonceFn:   defaultNonce,
	}

	prelude := "stty -echo -onlcr -opost -icanon rows 1000 cols 100000 2>/dev/null; " +
		"export LC_ALL=C TERM=dumb PS1='' PROMPT_COMMAND=''; " +
		"unset HISTFILE; true"
	if _, err := sh.Run(ctx, prelude); err != nil {
		_ = sh.Close()
		return nil, fmt.Errorf("shell prelude: %w", err)
	}
	return sh, nil
}

func pluginBinaryPath() string {
	switch runtime.GOOS {
	case "windows":
		return "C:\\Program Files\\Amazon\\SessionManagerPlugin\\bin\\session-manager-plugin.exe"
	default:
		return "/usr/local/sessionmanagerplugin/bin/session-manager-plugin"
	}
}
