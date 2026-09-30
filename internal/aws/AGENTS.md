# internal/aws — AWS SDK facade + SSM plugin I/O

Score 11 (distinct domain). Leaf package: imports only AWS SDK v2 + stdlib; everyone else depends on it.

## OVERVIEW
One `Client` facade over ec2/rds/elasticache/ssm service clients, plus `ShellSession` — an interactive shell over the `session-manager-plugin` subprocess used for the EC2 file browser and base64 file downloads.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| New service listing | `rds.go`, `elasticache.go`, `ec2.go`, `ec2_ssm.go` | one `List*(ctx) ([]Instance, error)` per file; `Instance` in `types.go` |
| Client construction | `client.go` | `NewClient(ctx)`: `config.LoadDefaultConfig` + `NewFromConfig` x4 |
| Port-forward session start | `ssm.go` | `AWS-StartPortForwardingSessionToRemoteHost` doc; subprocess mgmt lives in `internal/portforward` |
| Interactive shell | `ssm_shell.go` | sentinel protocol; see CONVENTIONS |
| Remote ls/stat/home | `fs.go` | parses `ls -lA` output (`parseLsOutput`), strips ANSI |
| File download | `download.go` | `StreamRemoteFileBase64`: `base64 -w0` stream, `boundedReader` |
| Bastion display name | `ec2.go` | `GetBastionDisplayName` |

## CONVENTIONS
- `ShellSession.Run` wraps every command as `echo __PAWS_BEGIN_<nonce>__; <cmd>; echo __PAWS_EOF_<nonce>__` and returns the payload between sentinels; first echoed line is skipped by scanning for EOF only after BEGIN.
- Run reads synchronously from the calling goroutine and holds `s.mu` — concurrency means opening a SECOND session (the UI opens separate browse/transfer shells for exactly this).
- ctx cancel watchdog: write `0x03` (Ctrl+C) to stdin, wait up to 5s for the EOF sentinel, then mark closed and shut stdin.
- New sessions get a prelude: `stty -echo -onlcr -opost -icanon rows 1000 cols 100000; export LC_ALL=C TERM=dumb PS1='' PROMPT_COMMAND=''; unset HISTFILE`.
- `exec.Command`, NOT `exec.CommandContext`, for the plugin subprocess — callers use short-lived timeout ctxs and a ctx-bound kill would break every later Run with "broken pipe". Termination is `ShellSession.Close()`.
- stderr drains into `lockedBuffer` (mutex-guarded; exec drains stderr from an internal goroutine) — `Stderr()` is the diagnostic for "broken pipe" (IAM denial, agent ping failure, session limit).
- `pluginBinaryPath()` hardcodes `/usr/local/sessionmanagerplugin/bin/session-manager-plugin` (mac/linux) and the `C:\Program Files\Amazon\...` exe — duplicated in `internal/portforward/manager.go`; change both.
- All shell output goes through `stripANSI` — PTY sessions inject escape sequences even with `stty -opost`.

## ANTI-PATTERNS
- NEVER create a second `bufio.Reader` over a session's stdout: `reader` is the single owner for the session's lifetime and buffers ahead — another reader silently drops bytes (struct doc invariant).
- NEVER touch a session from two goroutines without `mu`; callers needing concurrency open separate sessions.
- Don't wrap list-method errors inconsistently — List* methods return raw `err` while SSM methods wrap with `%w`; match the file you're in.
