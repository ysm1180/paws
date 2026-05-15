package aws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"testing"
	"time"
)

// TestBoundedReader_DoesNotStallNearEOF reproduces the "last few percent"
// download stall: when the remaining bytes after BEGIN are smaller than the
// old Peek window, boundedReader.Peek blocks forever waiting for bytes a
// persistent shell never sends. The fix peeks only len(end) at minimum and
// then drains whatever is already buffered.
func TestBoundedReader_DoesNotStallNearEOF(t *testing.T) {
	end := []byte("__PAWS_EOF_test__")
	payload := []byte("hello world, this is the file body")

	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write(payload)
		_, _ = pw.Write([]byte("\n"))
		_, _ = pw.Write(end)
		_, _ = pw.Write([]byte("\n"))
		// Deliberately do NOT close — a real SSM shell stays open after
		// the command output.
	}()

	br := &boundedReader{
		r:   bufio.NewReaderSize(pr, 64*1024),
		end: end,
	}

	done := make(chan struct{})
	var got bytes.Buffer
	var readErr error
	go func() {
		defer close(done)
		buf := make([]byte, 8*1024)
		for {
			n, err := br.Read(buf)
			if n > 0 {
				got.Write(buf[:n])
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				readErr = err
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("boundedReader stalled near EOF (regression of last-few-percent bug)")
	}

	if readErr != nil {
		t.Fatalf("unexpected error: %v", readErr)
	}
	// Output includes the newline before the sentinel; the base64 decoder
	// strips that in the real path.
	want := append(payload, '\n')
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("got %q, want %q", got.Bytes(), want)
	}
}

// TestStreamRemoteFileBase64_SmallFile checks the full path end-to-end: a
// tiny file streams through StreamRemoteFileBase64 without stalling and the
// decoded output matches the source bytes.
func TestStreamRemoteFileBase64_SmallFile(t *testing.T) {
	src := []byte("the quick brown fox jumps over the lazy dog")
	encoded := base64.StdEncoding.EncodeToString(src)

	pr, pw := io.Pipe()
	go func() {
		// Mimic the actual shell command output: echoed command line
		// (drained by drainUntilLine), then BEGIN, then base64 payload,
		// then a blank echo, then EOF.
		_, _ = pw.Write([]byte("echo __PAWS_BEGIN_test__; base64 -w0 /file; echo; echo __PAWS_EOF_test__\n"))
		_, _ = pw.Write([]byte("__PAWS_BEGIN_test__\n"))
		_, _ = pw.Write([]byte(encoded))
		_, _ = pw.Write([]byte("\n"))
		_, _ = pw.Write([]byte("__PAWS_EOF_test__\n"))
	}()

	sh := &ShellSession{
		stdin:   &fakeStdin{},
		reader:  bufio.NewReaderSize(pr, 64*1024),
		nonceFn: func() string { return "test" },
	}

	var got bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- StreamRemoteFileBase64(context.Background(), sh, "/file", &got, nil)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StreamRemoteFileBase64: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("StreamRemoteFileBase64 stalled")
	}

	if !bytes.Equal(got.Bytes(), src) {
		t.Errorf("got %q, want %q", got.Bytes(), src)
	}
}
