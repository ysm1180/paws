# PROJECT KNOWLEDGE BASE

**Generated:** 2026-09-30T07:28Z
**Commit:** 0927dea
**Branch:** main

## OVERVIEW
Paws (`github.com/ysm1180/paws`, Go 1.21): a Bubble Tea TUI for AWS SSM Session Manager — port forwarding to RDS/ElastiCache through EC2 bastions, plus an EC2 remote file browser with downloads over SSM shell sessions. AWS SDK for Go v2, no cobra/cli framework; the whole UI is one Bubble Tea program.

## STRUCTURE
```
.
├── main.go              # tea.NewProgram(WithAltScreen, WithMouseCellMotion) + ui.NewModel
├── internal/aws/        # AWS SDK v2 facade + session-manager-plugin subprocess I/O (has AGENTS.md)
├── internal/ui/         # entire TUI: Model, tabs, file browser, key/mouse (has AGENTS.md)
├── internal/config/     # JSON persistence at ~/.ssm_session_manager/config.json (stdlib only)
├── internal/portforward/ # long-lived port-forward sessions: plugin subprocess, 5s health ticker, auto-reconnect x3
├── internal/transfer/   # download Job lifecycle: one job per instance, .partial file + size integrity check
└── Makefile             # build/run/clean/install; binary name `paws`
```

Dependency direction: `ui -> {aws, config, portforward, transfer}`; `portforward -> aws`; `transfer -> aws`. `aws` and `config` are leaves.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Add/change key binding | `internal/ui/keys.go` | central `var Keys = KeyMap{}`; hints in `hints.go` |
| Change instance listing | `internal/aws/{rds,elasticache,ec2,ec2_ssm}.go` | one List* func per service |
| Port-forward behavior | `internal/portforward/manager.go` | subprocess + reconnect; UI wiring in `model.go` |
| Shell command execution on EC2 | `internal/aws/ssm_shell.go` | sentinel protocol, see its AGENTS.md |
| File download | `internal/transfer/` + `internal/aws/download.go` | base64 streaming over the shell session |
| File browser UI | `internal/ui/filebrowser{,_render}.go` | state machine + view |
| Persisted settings | `internal/config/config.go` | ports, bastions, cwd, download_dir |
| Mouse hit-testing | `internal/ui/mouse.go`, `regions.go` | regions written during render |

## CODE MAP
LSP (gopls) unavailable; refs counted via rg (qualified refs undercount same-package use).

| Symbol | Type | Location | Refs | Role |
|--------|------|----------|------|------|
| `Model` | struct | `internal/ui/model.go` | central (1289 LOC) | root Bubble Tea model: tabs, screens, Update/View |
| `Client` | struct | `internal/aws/client.go` | ~59 (`aws.`) | facade holding ec2/rds/elasticache/ssm clients + Region/Profile |
| `ShellSession` | struct | `internal/aws/ssm_shell.go` | ~33 | one interactive SSM shell over the plugin subprocess |
| `Instance` | struct | `internal/aws/types.go` | ~24 | unified EC2/RDS/ElastiCache row |
| `StreamRemoteFileBase64` | func | `internal/aws/download.go` | 12 | base64 stream download through a ShellSession |
| `portforward.Manager` | struct | `internal/portforward/manager.go` | via ui | port-forward lifecycle + auto-reconnect |
| `transfer.Manager` / `Job` | struct | `internal/transfer/` | via ui | download jobs, progress, cancel |
| `Config` | struct | `internal/config/config.go` | via ui | `Load`/`Save`, per-instance getters/setters |
| `Keys` | var | `internal/ui/keys.go` | ui-wide | all key bindings in one place |

## CONVENTIONS
- All `Model` mutations happen in `Update` on the main goroutine. `tea.Cmd` closures capture locals (`target := *inst; parentCtx := m.ctx`) and return results via `*Msg` types — resource-opening cmds return the resource inside the msg (e.g. `browserOpenedMsg` carries two `*aws.ShellSession`).
- No `log`/`slog`: diagnostics go to the in-app log panel via `m.addLog(instanceType, msg)`; `portforward`/`transfer` report through `onLog`/`onProgress` callbacks, not channels.
- Errors: `fmt.Errorf("...: %w", err)` at package boundaries; UI errors become `logMsg{"error", ...}`; ignored errors use explicit `_ =`; `os.Exit` only in `main.go`.
- Message types end in `Msg`; enums are iota-typed ints (`tabType`, `panel`, `inputMode`, `screen`, `Status`); tests are `Test<Type>_<Behavior>`, stdlib `testing` only, table-driven for multi-case logic.
- Shell plugin subprocess: `exec.Command`, NOT `CommandContext` (rationale in `internal/aws/AGENTS.md`); `portforward/manager.go` deliberately does the opposite (ctx-bound cancel tied to session stop).
- ctx: one `context.Background()` in main, stored on the Model; cmd goroutines derive `context.WithTimeout(ctx, 15*time.Second)`.

## ANTI-PATTERNS (THIS PROJECT)
- NEVER mutate `Model` state inside a `tea.Cmd` goroutine (`model.go:212`, `mouse.go:13`).
- NEVER create a second reader over a `ShellSession`'s stdout (invariant owned by `internal/aws`, details in its AGENTS.md).
- Do not bind the plugin subprocess to a caller's ctx — termination goes through `ShellSession.Close()`.
- Do not add an interface abstraction over `aws.Client` — the project deliberately uses the concrete type; testability is via `nonceFn`/`streamFn` injection points.

## UNIQUE STYLES
- Sentinel protocol: shell commands wrapped in `__PAWS_BEGIN_<nonce>__` / `__PAWS_EOF_<nonce>__`; payloads parsed between markers; ANSI stripped.
- File transfer = base64 stream over the same shell channel; `.partial` suffix + remote `stat -c %s` size check on completion.
- Mouse hit-testing via `Region` rects written as a render side effect (documented as "not application state").
- Local `min`/`max` helpers in `model.go` shadow Go 1.21 builtins (redundant but intentional).

## COMMANDS
```bash
make build        # go build -o paws .
make run          # go run .
go test ./...     # 8 test files across aws, transfer, ui
make install      # sudo cp paws /usr/local/bin/
```
Runtime requirements: AWS CLI configured, `session-manager-plugin` binary.

## NOTES
- Repo dir name (`aws-session-manager-main`) != module/binary name (`paws`).
- README is partially stale: `n/N` bastion keys and `?` help don't exist (`b` = bastion popup, hints are contextual); Tab covers 3 tabs (RDS/ElastiCache/EC2), not 2.
- `session-manager-plugin` path is hardcoded per-OS in BOTH `internal/aws/ssm_shell.go` and `internal/portforward/manager.go` (`/usr/local/sessionmanagerplugin/bin/...`, `C:\Program Files\Amazon\...`) — change both.
- Config file lives at `~/.ssm_session_manager/config.json` (not XDG).
- No CI, no lint config, no `cmd/` dir, no LICENSE. `model.go` (1289 LOC) is the complexity hotspot.
