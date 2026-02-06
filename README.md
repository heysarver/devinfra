# devinfra

Local development infrastructure manager. Single binary that manages Traefik, DNSMasq, and Docker Compose projects with `.test` domains and HTTPS.

## Install

### From source

```bash
go install github.com/heysarver/devinfra@latest
```

Or clone and build:

```bash
git clone https://github.com/heysarver/devinfra.git
cd devinfra
make install
```

This puts both `devinfra` and `di` on your `$GOPATH/bin`. If `GOPATH/bin` is not in your `PATH`, it will be added to your shell config automatically.

### Homebrew

```bash
brew install heysarver/tap/devinfra
```

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (with Docker Compose v2)
- [mkcert](https://github.com/FiloSottile/mkcert)

## Quick Start

```bash
di init          # One-time setup: config dir, DNS, certs, Docker network
di up            # Start Traefik + DNSMasq + socket-proxy
di doctor        # Verify everything works
di new           # Interactive project wizard
```

## Commands

### Infrastructure

```bash
di init                        # First-time setup
di init --import-from ~/dev-infra  # Import from existing bash-based repo
di up                          # Start core infrastructure
di down                        # Stop core infrastructure
di logs                        # Tail infrastructure logs
```

### Project Lifecycle

```bash
di new                         # Interactive wizard
di new --name myapp \
  --dir ~/projects/myapp \
  --services web:3000,api:8080 \
  --flavors postgres           # Non-interactive

di up myapp                    # Start project (prompts to start infra if needed)
di down myapp                  # Stop project
di up --all                    # Start infra + all projects
di down --all                  # Stop all projects

di flavor add myapp redis      # Add flavor overlay
di remove myapp                # Unregister (keeps directory)
di remove myapp --no-directory-preserve  # Unregister and delete directory
```

### Certificates

```bash
di certs regen                 # Regenerate all certs
di certs regen myapp           # Regenerate one project's certs
```

### Inspection

```bash
di status                      # List all projects with status
di status --json               # Machine-readable output
di inspect myapp               # Full project detail
di inspect myapp --json        # JSON output
di list projects               # Project names
di list flavors                # Available flavors
di doctor                      # Health check
di doctor --json               # Structured health report
```

### Utilities

```bash
di clean                       # Remove certs + dynamic configs
di version                     # Print version info
di completion bash             # Shell completion script
```

## Global Flags

```
--json          Machine-readable JSON output (where applicable)
--yes / -y      Skip confirmation prompts
--verbose / -v  Debug output
--quiet / -q    Suppress non-error output
--no-color      Disable colored output
--config-dir    Override config directory
```

## Configuration

Runtime state lives in the XDG config directory. On macOS this is `~/Library/Application Support/devinfra/` with a convenience symlink at `~/.config/devinfra`. On Linux it defaults to `~/.config/devinfra/`. Override with `$DEVINFRA_HOME` or `$XDG_CONFIG_HOME`.

```
~/.config/devinfra/
├── .env                           # DNS_PORT=5354
├── projects.yaml                  # Project registry
├── compose/docker-compose.yaml    # Core infrastructure
├── certs/                         # mkcert certificates
└── dynamic/                       # Traefik file-provider configs
```

## Flavors

Flavors add infrastructure services to a project as Docker Compose overlay files.

| Flavor | What It Adds |
|--------|-------------|
| `postgres` | PostgreSQL 16 with random password |
| `redis` | Valkey 9 + Sentinel |
| `rabbitmq` | RabbitMQ 4 with management UI |
| `minio` | MinIO S3-compatible object storage with console UI |

## Shell Completion

```bash
# bash
echo 'source <(di completion bash)' >> ~/.bashrc

# zsh
echo 'source <(di completion zsh)' >> ~/.zshrc

# fish
di completion fish > ~/.config/fish/completions/di.fish
```

## Migrating from dev-infra (bash)

```bash
di init --import-from /path/to/dev-infra
```

This imports `projects.yaml`, certificates, and Traefik dynamic configs from an existing bash-based dev-infra repo.

## Development

```bash
make build     # Build binary to bin/devinfra
make install   # Install devinfra + di symlink, add GOPATH/bin to PATH
make test      # Run tests
make lint      # Run golangci-lint
```

## License

MIT
