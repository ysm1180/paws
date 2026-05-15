package aws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
)

// ProgressFunc is called periodically with the number of raw (decoded) bytes
// written so far.
type ProgressFunc func(transferredBytes int64)

// StreamRemoteFileBase64 sends `base64 -w0 <path>` to the shell, waits for
// the BEGIN sentinel line, then stream-decodes the base64 payload that
// follows until the EOF sentinel substring is seen.
//
// It reads through sh.reader (the session's single shared *bufio.Reader);
// creating any other reader over sh.stdoutR here would silently drop bytes
// that sh.reader has already buffered ahead.
func StreamRemoteFileBase64(
	ctx context.Context,
	sh *ShellSession,
	remotePath string,
	w io.Writer,
	onProgress ProgressFunc,
) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if sh.closed.Load() {
		return fmt.Errorf("shell session closed")
	}
	if sh.nonceFn == nil {
		sh.nonceFn = defaultNonce
	}
	nonce := sh.nonceFn()
	begin := "__PAWS_BEGIN_" + nonce + "__"
	end := "__PAWS_EOF_" + nonce + "__"
	cmd := fmt.Sprintf("echo %s; base64 -w0 %s; echo; echo %s\n",
		begin, shellQuote(remotePath), end)
	if _, err := io.WriteString(sh.stdin, cmd); err != nil {
		return fmt.Errorf("write base64 cmd: %w", err)
	}

	var ctxErr atomic.Value
	watchdogDone := make(chan struct{})
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				ctxErr.Store(ctx.Err())
				_, _ = sh.stdin.Write([]byte{0x03})
				select {
				case <-watchdogDone:
				case <-time.After(5 * time.Second):
					sh.closed.Store(true)
					_ = sh.stdin.Close()
				}
			case <-watchdogDone:
			}
		}()
	}
	defer close(watchdogDone)

	if err := drainUntilLine(sh.reader, begin); err != nil {
		if v := ctxErr.Load(); v != nil {
			return v.(error)
		}
		return fmt.Errorf("await BEGIN: %w", err)
	}

	br := &boundedReader{r: sh.reader, end: []byte(end)}
	dec := base64.NewDecoder(base64.StdEncoding, br)
	buf := make([]byte, 48*1024)
	var total int64
	for {
		n, err := dec.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return fmt.Errorf("local write: %w", werr)
			}
			total += int64(n)
			if onProgress != nil {
				onProgress(total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			if v := ctxErr.Load(); v != nil {
				return v.(error)
			}
			return err
		}
	}
	if v := ctxErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// drainUntilLine reads whole lines (after ANSI strip + trim) and returns
// when one matches marker exactly.
func drainUntilLine(r interface {
	ReadString(byte) (string, error)
}, marker string) error {
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			if strings.TrimSpace(stripANSI(line)) == marker {
				return nil
			}
		}
		if err != nil {
			return err
		}
	}
}

// boundedReader streams bytes from r and returns io.EOF when end (a substring
// that cannot collide with base64 payload bytes — e.g. contains underscores)
// is first seen. It copies the prefix into a freshly-allocated buffer BEFORE
// Discard, which is the safe pattern for bufio.Reader.Peek+Discard.
type boundedReader struct {
	r         *bufio.Reader
	end       []byte
	done      bool
	truncated bool
}

// errTruncated signals that the remote stream ended before the EOF sentinel
// was seen.
var errTruncated = fmt.Errorf("remote stream ended before EOF sentinel")

func (b *boundedReader) Read(p []byte) (int, error) {
	if b.truncated {
		return 0, errTruncated
	}
	if b.done {
		return 0, io.EOF
	}
	// bufio.Reader.Peek(n) blocks until n bytes are buffered (or the
	// underlying reader errors). Asking for a large window — e.g. p's full
	// length — deadlocks near the end of a file: the remote shell stops
	// producing bytes once base64 + the sentinel + a trailing newline are
	// sent, so a Peek waiting for ~48K can never be satisfied. Peek only
	// enough bytes to make sentinel detection unambiguous, then use
	// Buffered() to pull whatever else is already sitting in the buffer.
	if _, err := b.r.Peek(len(b.end)); err != nil {
		if avail := b.r.Buffered(); avail > 0 {
			out := make([]byte, avail)
			peeked, _ := b.r.Peek(avail)
			copy(out, peeked)
			_, _ = b.r.Discard(avail)
			b.truncated = true
			n := copy(p, out)
			if n < len(out) {
				return n, io.ErrShortBuffer
			}
			return n, nil
		}
		return 0, errTruncated
	}
	chunk, _ := b.r.Peek(b.r.Buffered())
	if idx := bytes.Index(chunk, b.end); idx >= 0 {
		out := make([]byte, idx)
		copy(out, chunk[:idx])
		consume := idx + len(b.end)
		for consume < len(chunk) && (chunk[consume] == '\r' || chunk[consume] == '\n') {
			consume++
		}
		if _, derr := b.r.Discard(consume); derr != nil {
			return 0, derr
		}
		b.done = true
		n := copy(p, out)
		if n < len(out) {
			return n, io.ErrShortBuffer
		}
		return n, nil
	}
	// No sentinel in the buffered chunk. Hand back everything except the
	// trailing len(end)-1 bytes — those might be the prefix of a sentinel
	// that will complete on the next Peek.
	safe := len(chunk) - (len(b.end) - 1)
	if safe < 1 {
		bbyte, rerr := b.r.ReadByte()
		if rerr != nil {
			return 0, errTruncated
		}
		p[0] = bbyte
		return 1, nil
	}
	if safe > len(p) {
		safe = len(p)
	}
	return b.r.Read(p[:safe])
}
