package aws

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

type fakeStdin struct{ buf bytes.Buffer }

func (f *fakeStdin) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *fakeStdin) Close() error                { return nil }

func newFakeShell(stdout string) (*ShellSession, *fakeStdin) {
	in := &fakeStdin{}
	return &ShellSession{
		stdin:   in,
		reader:  bufio.NewReader(strings.NewReader(stdout)),
		nonceFn: func() string { return "test" },
	}, in
}

func TestRun_DropsEchoAndCapturesPayload(t *testing.T) {
	stdout := "echo __PAWS_BEGIN_test__; stat -c %s /tmp/x; echo __PAWS_EOF_test__\r\n" +
		"\x1b[?2004l__PAWS_BEGIN_test__\r\n" +
		"42\r\n" +
		"__PAWS_EOF_test__\r\n"
	sh, _ := newFakeShell(stdout)
	out, err := sh.Run(context.Background(), "stat -c %s /tmp/x")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.TrimSpace(out) != "42" {
		t.Fatalf("expected '42', got %q", out)
	}
}

func TestRun_SequentialCallsAreNotCorrupted(t *testing.T) {
	stdout := "__PAWS_BEGIN_test__\r\nfirst\r\n__PAWS_EOF_test__\r\n" +
		"__PAWS_BEGIN_test__\r\nsecond\r\n__PAWS_EOF_test__\r\n"
	sh, _ := newFakeShell(stdout)
	o1, err := sh.Run(context.Background(), "echo first")
	if err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	o2, err := sh.Run(context.Background(), "echo second")
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if strings.TrimSpace(o1) != "first" || strings.TrimSpace(o2) != "second" {
		t.Fatalf("o1=%q o2=%q", o1, o2)
	}
}

// TestRun_RecoversWhenPayloadHasNoTrailingNewline reproduces the original
// EchoHome hang: `printf %s "$HOME"` emits the home path without a
// terminating \n, so the EOF sentinel concatenates onto the same line.
// Run must detect the sentinel within the line and still recover the
// payload, instead of hanging waiting for a newline that never arrives.
func TestRun_RecoversWhenPayloadHasNoTrailingNewline(t *testing.T) {
	stdout := "__PAWS_BEGIN_test__\r\n" +
		"/home/ec2-user__PAWS_EOF_test__\r\n"
	sh, _ := newFakeShell(stdout)
	out, err := sh.Run(context.Background(), `printf %s "$HOME"`)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.TrimSpace(out) != "/home/ec2-user" {
		t.Fatalf("expected '/home/ec2-user', got %q", out)
	}
}

func TestRun_PropagatesContextCancel(t *testing.T) {
	// Empty reader → ReadString returns io.EOF immediately. The watchdog
	// races to store ctxErr first; either way Run must return a non-nil
	// error when ctx is cancelled before the EOF sentinel arrives.
	sh := &ShellSession{
		stdin:   &fakeStdin{},
		reader:  bufio.NewReader(strings.NewReader("")),
		nonceFn: func() string { return "test" },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := sh.Run(ctx, "true"); err == nil {
		t.Fatal("expected error (context cancelled or EOF)")
	}
}
