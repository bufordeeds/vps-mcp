# vps-mcp

A Go [Model Context Protocol](https://spec.modelcontextprotocol.io/) tool server for managing a remote Linux VPS, designed to be consumed by [kagent](https://kagent.dev/) or any other MCP client.

Built as a learning project to get fluent in Go using the same primitives (Kubernetes, MCP, kagent) that [Solo.io](https://www.solo.io/) is shipping production tooling around.

## What it does

Exposes four tools over MCP. An LLM-driven agent can call them to answer questions about a remote VPS without you SSHing in:

| Tool | Purpose |
|---|---|
| `vps_health` | Uptime, load avg, disk %, mem %, top containers |
| `vps_caddy_logs` | Recent Caddy access logs for a domain |
| `vps_container_status` | `docker ps` for a named or all containers |
| `vps_disk_usage` | `du` for a path, sorted descending |

All tools shell out via SSH using a private key mounted as a Kubernetes Secret. The server has no persistent state.

## Architecture

```
 ┌──────────┐    MCP (stdio)    ┌────────────┐    SSH    ┌─────────┐
 │  kagent  │ ───────────────→  │  vps-mcp   │ ────────→ │   VPS   │
 │  (LLM)   │ ←───────────────  │  (Go bin)  │ ←──────── │ (Linux) │
 └──────────┘   tool results    └────────────┘  stdout   └─────────┘
       │                              │
       │  Kubernetes (kind / cluster) │
       └──────────────────────────────┘
```

## Quickstart

Prerequisites: Go 1.22+, Docker, [kind](https://kind.sigs.k8s.io/), `kubectl`, an SSH key with access to your target VPS.

```bash
# 1. Build and run locally over stdio
export VPS_HOST=user@your.vps.ip
export VPS_SSH_KEY_PATH=$HOME/.ssh/id_ed25519
go run ./cmd/server

# 2. Or run the full kind + kagent demo
./examples/kind-quickstart.sh
```

Once kagent is running, ask the agent things like:

- *"Is the VPS healthy?"* → `vps_health`
- *"Did anyone visit example.com in the last hour?"* → `vps_caddy_logs`
- *"Why is `/` 80% full?"* → `vps_disk_usage`

## Configuration

| Env var | Default | Description |
|---|---|---|
| `VPS_HOST` | _required_ | `user@host` to SSH into |
| `VPS_SSH_KEY_PATH` | `/etc/vps-mcp/ssh_key` | Path to private key |
| `MCP_TRANSPORT` | `stdio` | `stdio` or `http` (HTTP/SSE not yet implemented) |
| `MCP_LISTEN_ADDR` | `:8080` | Bind address when transport is `http` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Repo layout

```
.
├── cmd/server/             # MCP server entry point
├── internal/
│   ├── mcp/                # JSON-RPC + tool registry
│   └── vps/                # SSH client + per-tool implementations
├── deploy/
│   ├── Dockerfile
│   ├── kubernetes/         # Deployment, Service, Secret example
│   └── kagent/             # kagent CRD example
├── examples/               # End-to-end demo scripts
└── .github/workflows/      # CI: test, lint, build
```

## Roadmap

- [x] Repo scaffold + project structure
- [x] Minimal MCP stdio server (initialize, tools/list, tools/call)
- [x] `vps_health` tool — fully working
- [ ] `vps_caddy_logs` — stub, needs Caddy JSON log parser
- [ ] `vps_container_status` — stub, needs `docker ps` parser
- [ ] `vps_disk_usage` — stub, needs `du` parser
- [ ] HTTP/SSE transport
- [ ] Dockerfile + K8s manifests verified end-to-end
- [ ] kagent CRD example pinned to a real kagent release
- [ ] Per-tool tests (table-driven)
- [ ] CI green

## Why this project

I'm an experienced React/TypeScript engineer ramping on Go. This is small enough to finish, large enough to exercise the parts of Go that matter for backend infra work (interfaces, context, goroutines, errors), and lands directly on the protocol stack Solo.io is building around. See [`docs/learning-notes.md`](docs/learning-notes.md) (TBD) for what I'm picking up along the way.

## License

MIT. See [LICENSE](LICENSE).
