# Paws


Terminal User Interface for AWS SSM Session Manager - port forwarding to RDS and ElastiCache.

## Requirements

- Go 1.21+
- AWS CLI configured
- AWS Session Manager Plugin

## Installation

```bash
# Install Go (macOS)
brew install go

# Build
cd paws
go mod tidy
go build -o paws .

# Run
./paws
```

## Key Bindings

| Key | Action |
|-----|--------|
| `↑/k` `↓/j` | Navigate instances |
| `Tab` | Switch between RDS/ElastiCache |
| `c` | Connect (start port forwarding) |
| `d` | Disconnect (stop port forwarding) |
| `/` | Filter instances |
| `p` | Edit local port |
| `n/N` | Next/Previous bastion host |
| `r` | Refresh instances |
| `?` | Toggle help |
| `q` | Quit |

## Features

- RDS and ElastiCache instance listing
- Instance filtering
- Port forwarding via SSM
- Bastion host selection
- Settings persistence (port history, bastion selection)
- Real-time log viewer

## EC2 SSM File Browser (`3` key)

Press `3` to switch to the EC2 tab — it lists Linux EC2 instances whose SSM
agent is currently `Online`. Select an instance and press `Enter` to open a
remote file browser.

### Browser key bindings

| Key | Action |
|---|---|
| `↑↓` (or `j` / `k`) | Move cursor |
| `Enter` | Enter directory or download file (prompts for local path) |
| `h` / `backspace` / `←` | Up one directory |
| `:` | Jump to absolute path (address bar) |
| `esc` | Cancel running download, or close browser |
| `q` / `ctrl+c` | Quit (SSM sessions are auto-terminated) |

### Download behavior

- One concurrent download per instance
- Progress bar and transfer speed displayed; UI stays responsive during the
  download (a separate SSM session is used so directory navigation still
  works while transferring)
- Integrity check after completion: local file size must equal `stat -c %s`
  on the remote
- Default destination: `~/Downloads/paws/<instance-name>/<file>` (override
  with the `download_dir` field in `~/.ssm_session_manager/config.json`)
- Last directory per instance is remembered across runs

### Required IAM permissions

In addition to the port-forwarding permissions paws already needs:

- `ssm:DescribeInstanceInformation`
- `ssm:StartSession` (target: EC2 instance ARN, default shell document)
- `ssm:TerminateSession`
