package aws

import (
	"testing"
	"time"
)

func TestParseLsOutput_BasicEntries(t *testing.T) {
	out := `total 16
drwxr-xr-x 2 root root 4096 2026-05-10 12:34 logs
-rw-r--r-- 1 ec2-user ec2-user 524 2026-05-13 09:00 app.conf
lrwxrwxrwx 1 root root 11 2026-05-01 00:00 current -> /opt/v1
`
	entries, err := parseLsOutput(out)
	if err != nil {
		t.Fatalf("parseLsOutput: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "logs" || !entries[0].IsDir {
		t.Errorf("entry 0: %+v", entries[0])
	}
	if entries[1].Name != "app.conf" || entries[1].Size != 524 {
		t.Errorf("entry 1: %+v", entries[1])
	}
	if entries[2].Name != "current" || !entries[2].IsLink {
		t.Errorf("entry 2: %+v", entries[2])
	}
	want, _ := time.Parse("2006-01-02 15:04", "2026-05-13 09:00")
	if !entries[1].MTime.Equal(want) {
		t.Errorf("mtime parse: got %v want %v", entries[1].MTime, want)
	}
}

func TestParseLsOutput_IgnoresEmpty(t *testing.T) {
	entries, err := parseLsOutput("total 0\n\n")
	if err != nil {
		t.Fatalf("parseLsOutput: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0, got %d", len(entries))
	}
}

func TestParseLsOutput_StripsANSI(t *testing.T) {
	out := "\x1b[0mdrwxr-xr-x\x1b[0m 2 root root 4096 2026-05-10 12:34 \x1b[01;34mlogs\x1b[0m\n"
	entries, err := parseLsOutput(out)
	if err != nil {
		t.Fatalf("parseLsOutput: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "logs" || !entries[0].IsDir {
		t.Fatalf("expected [logs/dir], got %+v", entries)
	}
}
