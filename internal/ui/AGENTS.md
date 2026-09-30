# internal/ui — Bubble Tea front end

Score 9 (distinct domain: entire TUI layer). Only package importing all others; `model.go` (1289 LOC) is the repo's complexity hotspot.

## OVERVIEW
Single root `Model` (Init/Update/View) with three tabs (RDS/ElastiCache/EC2), two screens (`screenList`/`screenBrowser`), input modes (filter/port/bastion/address-bar), and a plain-struct file-browser sub-state. Rendering splits into compact vs full layouts; mouse works via regions recorded at render time.

## STRUCTURE
- `model.go` — Model, Update/View dispatch, port-forward cmds, browser open/close, log panel
- `filebrowser.go` / `filebrowser_render.go` — browser state machine + view, download prompt
- `messages_ec2.go` — all `tea.Msg` types and the `screen` enum
- `keys.go` — `var Keys = KeyMap{}`: every binding in one place
- `hints.go` — per-panel contextual key hints (the "help" UI; README's `?` is stale)
- `mouse.go` / `regions.go` — hit-testing against `Region` rects
- `styles.go` — lipgloss styles, icons, layout row constants
- `format_test.go` covers `formatInstanceRow` (defined in `model.go`)

## WHERE TO LOOK
| Task | Location |
|------|----------|
| Key handling flow | `Update` in `model.go` — routes by `m.screen` then `m.inputMode` |
| Browser behavior | `filebrowser.go` (`handleBrowserKey`, cd/download cmds) |
| New async operation | follow `openBrowserCmd`: capture locals, return a `*Msg` carrying results/resources |
| Layout/responsiveness | `renderCompactView`/`renderFullView` (width<100 or height<12 -> compact) |
| Tab/row click targets | `renderTabsWithRegions`, `instancesListRect`, `logsRect` |

## CONVENTIONS
- Mutations ONLY in `Update` (root rule — see root AGENTS.md). Render funcs may write `m.tabRegions`/list/logs rects as a documented side channel for hit-testing ("not application state"); nothing else may be mutated during render.
- Cmd goroutines run under `context.WithTimeout(m.ctx, 15*time.Second)` and never touch `m` directly.
- The file browser is deliberately NOT a `tea.Model`: plain struct + `handleBrowserKey` + `renderFileBrowser`, gated by `m.screen`.
- `tickEvery()` re-arms `tea.Tick(200ms)` on each `tickMsg` — periodic re-render for progress/speed display.
- Two `*aws.ShellSession` per browser session (browse + transfer) so downloads don't block navigation; reopen transfer shell after a cancelled download (`transferShellReopenedMsg`).
- On browser-open failure, close any partially opened shell AND call `awsClient.TerminateSession` (see `browserOpenFailedMsg`).
- Local `min`/`max` helpers shadow the Go 1.21 builtins — existing code calls the local ones.

## ANTI-PATTERNS
- `handleMouseMsg` must stay synchronous on the main goroutine — never spawn a goroutine that touches the Model from it (`mouse.go:13`); the root NEVER-mutate-outside-Update rule covers everything else.
- Don't add key bindings inline in Update handlers — extend `keys.go` KeyMap and add the hint in `hints.go` (hint tests are table-driven and catch missing entries).
