# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build     # Build binary to bin/devinfra
make install   # Install devinfra + di symlink to GOPATH/bin
make test      # go test ./...
make lint      # golangci-lint run
```

Run a single test package:
```bash
go test ./internal/compose/...
```

The binary embeds version info from git tags via ldflags (see `Makefile`). Tags use bare semver format (e.g., `0.0.3`), no `v` prefix.

## Architecture

**Entry point:** `main.go` → `cmd.Execute()` (Cobra CLI). All subcommands are in `cmd/`.

**Config directory** (`~/.config/devinfra/` or `$DEVINFRA_HOME`):
- `.env` — runtime settings (`TLD=`, `DNS_PORT=`, `REMOTE_ENABLED=`, etc.)
- `projects.yaml` — project registry (loaded/saved via `internal/config.Registry`)
- `compose/docker-compose.yaml` — core infra (rendered from embedded template at `di init`)
- `certs/` — mkcert certificates (world-readable, mounted `:ro` into Traefik)
- `dynamic/` — Traefik file-provider configs (`tls-<name>.yaml` per project)
- `acme/` — ACME JSON storage when remote domain is enabled (mounted `:rw`)

**Packages:**

- `internal/config` — all config access. `paths.go` defines all directory/file path helpers. `registry.go` defines `Project`/`Registry` YAML types and atomic save. `remote.go` reads remote domain config from env + `.env`. `validate.go` has validators for name, port, TLD, domain, email.

- `internal/compose` — wraps Docker and mkcert. `compose.go` runs docker compose commands and renders embedded templates. `certs.go` calls mkcert and writes Traefik TLS dynamic configs. `parse.go` parses compose files to detect services/ports. Embedded files live in `embed/` (compose template, dnsmasq config, Traefik dynamic TLS, setup scripts).

- `internal/project` — higher-level project operations. `add.go` contains `Add()` (registers project, generates overlay + certs) and `generateOverlay()` (writes `docker-compose.devinfra.yaml` with Traefik labels). `regenerate.go` rebuilds all overlays, certs, and dynamic configs for TLD/remote changes.

- `internal/ui` — colored output helpers (`ui.Info`, `ui.Ok`, `ui.Warn`, `ui.Error`).

- `internal/doctor` — health checks for `di doctor`.

- `cmd/` — one file per subcommand. `wizard_helpers.go` has shared Charmbracelet `huh` form helpers.

**Key data flow for a project:**

1. `di new` / `di add` → `project.Add()` → writes `docker-compose.devinfra.yaml` into the project directory (Traefik labels), generates mkcert certs, writes `dynamic/tls-<name>.yaml`, registers in `projects.yaml`.
2. `di up <name>` → `compose.ProjectUp()` → `docker compose -p <name> -f <base> -f docker-compose.devinfra.yaml up -d`
3. Core infra (`di up` with no args) → `compose.Up()` → `docker compose -p devinfra` in the config compose dir. Uses `infraEnv()` which injects `DNS_PORT` and optionally `CF_DNS_API_TOKEN`.

**Traefik routing model:**
- Local TLD routes: labels in `docker-compose.devinfra.yaml` (Docker provider)
- Remote domain routes: additional `-remote` router labels in the same overlay file when `REMOTE_ENABLED=true`
- Infrastructure TLS: Traefik file provider reads `dynamic/` directory
- ACME wildcard certs: Cloudflare DNS challenge, configured via `certificatesResolvers` in the rendered `docker-compose.yaml` template

**Embedded template rendering:**
`compose.ExtractEmbedded(tld)` renders `docker-compose.yaml` and `dnsmasq.conf` using `embedData{TLD, RemoteEnabled, ACMEEmail}` as Go template data. Called at `di init` and `di regenerate`. Conditional remote blocks use `{{- if .RemoteEnabled}}...{{- end}}`.

**Rollback pattern:** `internal/project/add.go` uses a `rollback` struct that collects cleanup functions and executes them on error; `rb.disarm()` cancels rollback on success.
