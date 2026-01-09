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
