# c2o-plugin

OpenShift Console dynamic plugin for deploying and managing c2o (Claude-to-OpenShift) coding agents. Provides a self-service UI where users deploy Claude Code agent instances into their own namespaces, manage their lifecycle, and get connection details for Claude Code MCP integration.

## Features

- **Deploy agents** — multi-step wizard: select agent type, namespace, count (1-10), credentials, review & deploy
- **Manage agents** — table view with status, delete, connection details for each instance
- **Credential management** — create and reference Kubernetes Secrets (API keys, GCP JSON, custom)
- **Connection details** — auto-generated MCP config, skill prompt, CLI install commands, Grafana URLs
- **Multi-tenant** — users operate in namespaces governed by their RBAC permissions

## Architecture

```
OpenShift Console
  ├── c2o-plugin (dynamic plugin, :9443)
  │     ├── React frontend (PatternFly)
  │     └── Go backend (gorilla/mux)
  │           └── K8s API (user's OAuth token)
  │
  └── Per-user namespaces
        ├── c2o-agent1 (Deployment + PVC + Services + Route)
        ├── c2o-agent2 (Deployment + PVC + Services + Route)
        └── c2o-env (Secret)
```

The plugin backend never uses a privileged service account for data operations. All Kubernetes API calls use the authenticated user's OAuth token — RBAC is enforced by the cluster.

## Prerequisites

- OpenShift 4.14+ with Console enabled
- `oc` CLI configured and logged in
- Node.js 18+ and Yarn (for frontend development)
- Go 1.23+ (for backend development)

## Quick start

### Deploy to OpenShift

```bash
# Build container image
make podman-build

# Push to registry
make podman-push

# Install via Helm (creates namespace c2o-plugin, registers ConsolePlugin)
make helm-install
```

The plugin appears in the OpenShift Console sidebar under "c2o Agents" after installation.

### Local development

```bash
# Install dependencies
yarn install
go mod download

# Terminal 1: frontend dev server (port 9001, proxies /api to :9443)
yarn start

# Terminal 2: backend in dev mode (plain HTTP, no TLS)
DEV_MODE=true PORT=9443 go run ./cmd/backend/main.go
```

Access at http://localhost:9001. The dev server hot-reloads frontend changes.

## Build

```bash
make compile        # Build frontend (yarn build)
make go-build       # Compile Go backend
make podman-build   # Build container image
make podman-push    # Push to registry
make helm-install   # Deploy to OpenShift
make helm-uninstall # Remove from OpenShift
```

Image: `quay.io/eformat/c2o-plugin:latest`

## What gets deployed per agent

When a user deploys agents through the wizard, the plugin creates:

| Resource | Name pattern | Purpose |
|----------|-------------|---------|
| Deployment | `c2o-{prefix}{n}` | Agent pod (c2o image with Claude Code, routing, observability) |
| PVC | `c2o-workspace-{instance}` | 20Gi persistent workspace |
| Service | `c2o-anthropic-{instance}` | Anthropic API (port 8819) |
| Service | `c2o-openai-{instance}` | OpenAI API (port 8899) |
| Service | `c2o-grafana-{instance}` | Grafana dashboard (port 3000) |
| Route | `c2o-grafana-{instance}` | HTTPS Grafana access |
| Secret | user-specified | Credentials (TOKEN, KIMI_HOST, GCP_PROJECT_ID) |

## Connecting to agents

After deployment, the "Connection Details" dialog provides three methods:

1. **MCP Config** — JSON for `.mcp.json` to register the c2o-agents MCP server
2. **Skill Prompt** — YAML skill definition for Claude Code
3. **CLI Install** — `curl` + `pip` + `claude mcp add` commands

## API endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/health` | Health check |
| GET | `/api/namespaces` | List accessible namespaces |
| POST | `/api/namespaces` | Create namespace |
| GET | `/api/agents?namespace=` | List agents in namespace |
| DELETE | `/api/agents/{name}?namespace=` | Delete agent + services + PVC |
| POST | `/api/deploy` | Deploy agent instances |
| GET | `/api/credentials?namespace=` | List credential secrets |
| POST | `/api/credentials` | Create credential secret |
| GET | `/api/connection?namespace=` | Get MCP config, skill prompt, install cmd, Grafana URLs |

All `/api/*` endpoints (except health) require a valid OpenShift OAuth token.

## Tech stack

- **Frontend**: React 17, PatternFly 6, TypeScript, Webpack 5
- **Backend**: Go 1.23, gorilla/mux, client-go
- **Deployment**: Helm chart, ConsolePlugin CRD, multi-stage Containerfile
