package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type RemoteEntry struct {
	Name   string
	Size   int64
	MTime  time.Time
	IsDir  bool
	IsLink bool
	Mode   string
}

// parseLsOutput parses `ls -lA --time-style=long-iso` output.
// PTY-driven SSM sessions may interleave ANSI color/control sequences
// even with stty -opost; strip them before field-splitting so
// strings.Fields doesn't split on escape characters.
func parseLsOutput(out string) ([]RemoteEntry, error) {
	var entries []RemoteEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(stripANSI(line))
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		mode := fields[0]
		size, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			continue
		}
		mtime, err := time.Parse("2006-01-02 15:04", fields[5]+" "+fields[6])
		if err != nil {
			continue
		}
		nameParts := fields[7:]
		isLink := mode[0] == 'l'
		if isLink {
			for i, p := range nameParts {
				if p == "->" {
					nameParts = nameParts[:i]
					break
				}
			}
		}
		entries = append(entries, RemoteEntry{
			Name:   strings.Join(nameParts, " "),
			Size:   size,
			MTime:  mtime,
			IsDir:  mode[0] == 'd',
			IsLink: isLink,
			Mode:   mode,
		})
	}
	return entries, nil
}

func ListRemoteDir(ctx context.Context, sh *ShellSession, absPath string) ([]RemoteEntry, error) {
	cmd := fmt.Sprintf("ls -lA --time-style=long-iso %s 2>/dev/null", shellQuote(absPath))
	out, err := sh.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("ls %s: %w", absPath, err)
	}
	return parseLsOutput(out)
}

func StatRemoteFile(ctx context.Context, sh *ShellSession, absPath string) (int64, error) {
	out, err := sh.Run(ctx, fmt.Sprintf("stat -c %%s %s", shellQuote(absPath)))
	if err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, fmt.Errorf("stat empty (file missing?)")
	}
	return strconv.ParseInt(out, 10, 64)
}

func EchoHome(ctx context.Context, sh *ShellSession) (string, error) {
	// '%s\n' (not '%s'): Run wraps this as "echo BEGIN; <cmd>; echo EOF",
	// so without a trailing newline the EOF sentinel would concatenate
	// onto the $HOME line and Run would hang waiting for the next \n.
	out, err := sh.Run(ctx, `printf '%s\n' "$HOME"`)
	if err != nil {
		return "", err
	}
	h := strings.TrimSpace(out)
	if h == "" {
		return "/", nil
	}
	return h, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
