# CLAUDE.md

## Project overview

c2o-plugin is an OpenShift Console dynamic plugin for self-service deployment and management of c2o coding agents. It has a React/TypeScript frontend (PatternFly) and a Go backend that proxies Kubernetes API calls using the authenticated user's OAuth token.

The plugin does not dispatch work to agents — it manages their lifecycle (deploy, delete, status). Work dispatch happens through the c2o-agents MCP server in the sibling repo `vllm-sr-claude`.

## Key files

| File | Purpose |
|------|---------|
| `cmd/backend/main.go` | Go HTTP server, TLS, route registration |
| `pkg/handlers/deploy.go` | Core logic: creates Deployment, PVC, Services, Route per agent |
| `pkg/handlers/agents.go` | List/delete agents |
| `pkg/handlers/connection.go` | Generates MCP config, skill prompt, CLI install commands |
| `pkg/handlers/credentials.go` | Create/list Kubernetes Secrets |
| `pkg/handlers/namespaces.go` | List/create namespaces |
| `pkg/handlers/auth.go` | Auth middleware — extracts user OAuth token |
| `pkg/k8s/client.go` | Creates K8s clientset from user token |
| `src/components/DeployPage.tsx` | 5-step deployment wizard |
| `src/components/ManagePage.tsx` | Agent management table |
| `src/components/ConnectionDialog.tsx` | MCP config / skill / CLI modal |
| `src/components/HelpPage.tsx` | In-app documentation |
| `src/utils/api.ts` | Frontend API client functions |
| `src/utils/types.ts` | TypeScript interfaces |
| `console-extensions.json` | OpenShift Console plugin manifest (routes, nav items) |
| `chart/c2o-plugin/` | Helm chart for deploying the plugin itself |

## Build and run

### Local development

```bash
yarn install && go mod download

# Terminal 1: frontend (hot reload on :9001)
yarn start

# Terminal 2: backend (HTTP, no TLS)
DEV_MODE=true PORT=9443 go run ./cmd/backend/main.go
```

### Build and deploy

```bash
make podman-build     # Build container image
make podman-push      # Push to registry
make helm-install     # Deploy to OpenShift (namespace: c2o-plugin)
make helm-uninstall   # Remove
```

### Other useful commands

```bash
yarn build            # Production frontend build
yarn lint             # ESLint
make go-build         # Compile Go backend (CGO_ENABLED=0)
make compile          # Frontend build only
```

## Architecture

```
OpenShift Console → c2o-plugin (:9443)
                      ├── Frontend: React + PatternFly
                      └── Backend: Go + gorilla/mux
                            └── K8s API (user's OAuth token)
```

All K8s operations use the user's token passed through the Console proxy — no privileged service account. RBAC enforcement is by the cluster.

The Console proxies requests: `/api/proxy/plugin/c2o-plugin/backend/api/*` → plugin service.

## API endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/health` | No | Health check |
| GET | `/api/namespaces` | Yes | List namespaces |
| POST | `/api/namespaces` | Yes | Create namespace |
| GET | `/api/agents?namespace=` | Yes | List agents (label: `app=c2o,app.kubernetes.io/managed-by=c2o-plugin`) |
| DELETE | `/api/agents/{name}?namespace=` | Yes | Delete agent + services + PVC |
| POST | `/api/deploy` | Yes | Deploy agents (body: agentType, namespace, count, prefix, credentialName, image) |
| GET | `/api/credentials?namespace=` | Yes | List credential secrets |
| POST | `/api/credentials` | Yes | Create secret (types: apikey, gcpjson, custom) |
| GET | `/api/connection?namespace=` | Yes | MCP config, skill prompt, install cmd, Grafana URLs |

## Frontend pages

| Route | Component | Purpose |
|-------|-----------|---------|
| `/c2o/deploy` | DeployPage | 5-step wizard: type → namespace → config → credentials → review |
| `/c2o/manage` | ManagePage | Agent table with status, delete, connection details |
| `/c2o/help/:topic?` | HelpPage | Markdown documentation |

## K8s resources created per agent

Deploy creates these resources per instance (e.g., "agent1"):

- **Deployment** `c2o-agent1` — 1 replica, c2o image, ports 8819/8899/3000/9090/9901
- **PVC** `c2o-workspace-agent1` — 20Gi RWO
- **Services** `c2o-anthropic-agent1` (8819), `c2o-openai-agent1` (8899), `c2o-grafana-agent1` (3000)
- **Route** `c2o-grafana-agent1` — TLS edge termination

Pod resources: 500m-4000m CPU, 2Gi-12Gi memory. Health checks on `:8819/health`.

## Labeling scheme

All resources labeled:
- `app: c2o`
- `c2o.instance: {instance-name}`
- `c2o.agent-type: {claude|codex|opencode}`
- `app.kubernetes.io/managed-by: c2o-plugin`

Agent listing filters by these labels. Delete cascades by instance label.

## Environment variables

### Plugin backend

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | 9443 | Listen port |
| `DEV_MODE` | "" | "true" disables TLS |
| `PLUGIN_DIST_DIR` | dist | Frontend static files path |
| `TLS_CERT_FILE` | /var/serving-cert/tls.crt | TLS cert |
| `TLS_KEY_FILE` | /var/serving-cert/tls.key | TLS key |

### Agent pods (set by deploy.go)

| Variable | Value |
|----------|-------|
| `UPSTREAM_HOST` | localhost |
| `ANTHROPIC_BASE_URL` | http://localhost:8819 |
| `ANTHROPIC_API_KEY` | sk-placeholder (overridden by secret) |
| From `c2o-env` secret | TOKEN, KIMI_HOST, GCP_PROJECT_ID, HF_TOKEN |

## Tech stack

- **Frontend**: React 17, PatternFly 6.4, TypeScript 5, Webpack 5, react-router 5
- **Backend**: Go 1.23, gorilla/mux 1.8, client-go 0.31
- **Deploy**: Helm chart, ConsolePlugin CRD, multi-stage Containerfile
- **Image**: `quay.io/eformat/c2o-plugin:latest`

## Development conventions

- Frontend styling uses PatternFly design tokens (CSS variables), not custom colors
- Backend handlers each get their own file in `pkg/handlers/`
- All handlers receive the user token via `r.Header.Get("X-User-Token")` (set by auth middleware)
- K8s clients are created per-request from user token — never cached
- Console plugin SDK handles module federation and route registration via `console-extensions.json`
