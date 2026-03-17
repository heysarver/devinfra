# Feature: Extra Domains, Custom Subdomains, and Mixed Host/Docker Services

## Problem

Projects like feedvalue need:

1. **Multiple domain patterns** — e.g., `*.feedvalue.test` (local) AND `*.feedvalue.sarvent.dev` (real domain, no dnsmasq)
2. **Custom subdomain names** — e.g., `support-site` service served at `docs.feedvalue.test` instead of `support-site.feedvalue.test`
3. **Root domain routing** — e.g., frontend app served at `feedvalue.test` (root) instead of `app.feedvalue.test`
4. **Mixed host/docker services** — some services run in Docker (wordpress, webhook-listener), others run on the host (frontend dev server, API dev servers) and need file-provider configs

Currently these require manual edits to `docker-compose.devinfra.yaml`, `host-*.yaml`, and `tls-*.yaml` — all of which `di regenerate` will overwrite or delete.

## Proposed Schema Changes

### `projects.yaml`

```yaml
- name: feedvalue
  dir: /path/to/feedvalue
  domain: '*.feedvalue.test'
  extra_domains:                        # NEW
    - '*.feedvalue.sarvent.dev'
  host_mode: false
  services:
    - name: support-site
      port: 5175
      subdomain: docs                   # NEW — custom subdomain (default: service name)
    - name: webhook-listener
      port: 8080
    - name: wordpress
      port: 80
    - name: frontend-web
      port: 5173
      subdomain: '@'                    # NEW — '@' means root domain
      host: true                        # NEW — file-provider, not docker labels
    - name: core-api
      port: 3001
      subdomain: api                    # NEW
      host: true                        # NEW
    - name: auth-api
      port: 3002
      subdomain: auth                   # NEW
      host: true                        # NEW
```

### Go Struct Changes

**`internal/config/registry.go`**

```go
type Service struct {
    Name      string `yaml:"name" json:"name"`
    Port      int    `yaml:"port" json:"port"`
    Subdomain string `yaml:"subdomain,omitempty" json:"subdomain,omitempty"` // custom subdomain; "@" = root domain
    Host      bool   `yaml:"host,omitempty" json:"host,omitempty"`           // runs on host, not in docker
}

type Project struct {
    Name         string    `yaml:"name" json:"name"`
    Dir          string    `yaml:"dir" json:"dir"`
    Domain       string    `yaml:"domain" json:"domain"`
    ExtraDomains []string  `yaml:"extra_domains,omitempty" json:"extra_domains,omitempty"`
    HostMode     bool      `yaml:"host_mode" json:"host_mode"`
    Services     []Service `yaml:"services" json:"services"`
    Flavors      []string  `yaml:"flavors,omitempty" json:"flavors,omitempty"`
    ComposeFile  string    `yaml:"compose_file,omitempty" json:"compose_file,omitempty"`
    Created      string    `yaml:"created_at" json:"created_at"`
}

// Helper: returns base domains stripped of "*." prefix
func (p Project) BaseDomains() []string {
    domains := []string{strings.TrimPrefix(p.Domain, "*.")}
    for _, d := range p.ExtraDomains {
        domains = append(domains, strings.TrimPrefix(d, "*."))
    }
    return domains
}

// Helper: returns effective subdomain (service name if not overridden)
func (s Service) ServiceSubdomain() string {
    if s.Subdomain != "" {
        return s.Subdomain
    }
    return s.Name
}
```

## Files That Need Changes

### 1. `internal/config/registry.go` — Struct + helpers

Add `ExtraDomains`, `Subdomain`, `Host` fields and `BaseDomains()`/`ServiceSubdomain()` helpers as shown above.

### 2. `internal/project/add.go` — `generateOverlay()`

Currently generates Host rules using a fixed pattern. Needs to:

- Filter to only `!svc.Host` services (docker services)
- Build Host rules across ALL base domains
- Use `ServiceSubdomain()` instead of `svc.Name` for subdomain
- Handle `@` subdomain (root domain, no subdomain prefix)

**Current rule generation (line ~132):**
```go
localRule = fmt.Sprintf("Host(`%s.%s`) || Host(`%s.%s.%s`)", name, tld, svc.Name, name, tld)
```

**Should become something like:**
```go
func buildHostRule(svc Service, baseDomains []string) string {
    sub := svc.ServiceSubdomain()
    var parts []string
    for _, base := range baseDomains {
        if sub == "@" {
            parts = append(parts, fmt.Sprintf("Host(`%s`)", base))
        } else {
            parts = append(parts, fmt.Sprintf("Host(`%s.%s`)", sub, base))
        }
    }
    return strings.Join(parts, " || ")
}
```

### 3. New function: `generateHostConfig()` in `internal/project/add.go`

Generate `host-{name}.yaml` file-provider config for `svc.Host == true` services:

```yaml
http:
  routers:
    feedvalue-frontend-web:
      rule: "Host(`feedvalue.test`) || Host(`feedvalue.sarvent.dev`)"
      entryPoints:
        - websecure
      tls: {}
      service: feedvalue-frontend-web
  services:
    feedvalue-frontend-web:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:5173"
```

Write to `~/.config/devinfra/dynamic/host-{name}.yaml`.

### 4. `internal/compose/certs.go`

**`GenerateCerts()`** — Also generate certs for extra domains:
```go
// For each extra domain pattern like "*.feedvalue.sarvent.dev":
// mkcert "feedvalue.sarvent.dev" "*.feedvalue.sarvent.dev"
```

**`WriteTLSConfig()`** — Include extra domain cert entries:
```yaml
tls:
  certificates:
    - certFile: /certs/feedvalue.test+1.pem
      keyFile: /certs/feedvalue.test+1-key.pem
    - certFile: /certs/sarvent.dev+1.pem
      keyFile: /certs/sarvent.dev+1-key.pem
```

These functions need access to the Project (currently they only take `name string`). Either pass the Project or load the registry.

**`RemoveCerts()`** — Currently deletes `host-{name}.yaml` (line 113). Should only delete it if the project has no host services, or better yet, let regenerate handle it (generate fresh host config instead of deleting).

### 5. `internal/project/regenerate.go`

Add call to `generateHostConfig()` alongside `generateOverlay()`:

```go
// Rewrite overlay for docker services
if !p.HostMode && len(dockerServices) > 0 {
    generateOverlay(...)
}

// Rewrite host config for host services
if len(hostServices) > 0 {
    generateHostConfig(...)
}
```

### 6. `cmd/add.go` — Interactive service setup

The `add` command's interactive flow should prompt for:
- Custom subdomain (default: service name)
- Whether it's a host service

## Current Manual Workaround (feedvalue)

Until this is implemented, feedvalue uses manually edited files:

- `feedvalue/docker-compose.devinfra.yaml` — custom Host rules with `*.feedvalue.sarvent.dev`
- `~/.config/devinfra/dynamic/host-feedvalue.yaml` — file-provider for host services
- `~/.config/devinfra/dynamic/tls-feedvalue.yaml` — includes both cert sets
- `~/.config/devinfra/certs/sarvent.dev+1*.pem` — mkcert cert for `*.feedvalue.sarvent.dev`

**WARNING: Do NOT run `di regenerate` until this feature is implemented — it will overwrite these manual configs.**

## Expected Routing (feedvalue)

| Service | Docker/Host | feedvalue.test | feedvalue.sarvent.dev |
|---------|-------------|----------------|----------------------|
| frontend-web | host | `feedvalue.test` | `feedvalue.sarvent.dev` |
| support-site | docker | `docs.feedvalue.test` | `docs.feedvalue.sarvent.dev` |
| wordpress | docker | `wordpress.feedvalue.test` | `wordpress.feedvalue.sarvent.dev` |
| webhook-listener | docker | `webhook-listener.feedvalue.test` | `webhook-listener.feedvalue.sarvent.dev` |
| core-api | host | `api.feedvalue.test` | `api.feedvalue.sarvent.dev` |
| auth-api | host | `auth.feedvalue.test` | `auth.feedvalue.sarvent.dev` |
