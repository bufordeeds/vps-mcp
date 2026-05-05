package vps

import (
	"context"
	"encoding/json"
	"errors"
)

// errNotImplemented is returned by every stub tool until its real
// implementation lands. The error is surfaced to the LLM via isError=true,
// so the agent sees a clear message and won't hang waiting on a result.
var errNotImplemented = errors.New("not implemented yet — see roadmap in README")

// ── vps_caddy_logs ──────────────────────────────────────────────────────────

// CaddyLogsTool will fetch and parse Caddy access logs for a domain.
type CaddyLogsTool struct{ ssh *SSHClient }

// NewCaddyLogsTool constructs a CaddyLogsTool stub.
func NewCaddyLogsTool(ssh *SSHClient) *CaddyLogsTool { return &CaddyLogsTool{ssh: ssh} }

func (t *CaddyLogsTool) Name() string { return "vps_caddy_logs" }

func (t *CaddyLogsTool) Description() string {
	return "Fetches recent Caddy access logs for a given domain. Args: domain " +
		"(required), since (e.g. \"1h\", default 1h), limit (default 50)."
}

func (t *CaddyLogsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"domain":{"type":"string","description":"Domain to filter by, e.g. example.com"},
			"since":{"type":"string","description":"Time window like 1h, 24h, 7d","default":"1h"},
			"limit":{"type":"integer","minimum":1,"maximum":1000,"default":50}
		},
		"required":["domain"],
		"additionalProperties":false
	}`)
}

func (t *CaddyLogsTool) Call(_ context.Context, _ json.RawMessage) (string, error) {
	return "", errNotImplemented
}

// ── vps_container_status ────────────────────────────────────────────────────

// ContainerStatusTool will report `docker ps` for a named container or all.
type ContainerStatusTool struct{ ssh *SSHClient }

// NewContainerStatusTool constructs a ContainerStatusTool stub.
func NewContainerStatusTool(ssh *SSHClient) *ContainerStatusTool {
	return &ContainerStatusTool{ssh: ssh}
}

func (t *ContainerStatusTool) Name() string { return "vps_container_status" }

func (t *ContainerStatusTool) Description() string {
	return "Returns Docker container status. Args: name (optional, filters to one container)."
}

func (t *ContainerStatusTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"name":{"type":"string","description":"Optional container name filter"}
		},
		"additionalProperties":false
	}`)
}

func (t *ContainerStatusTool) Call(_ context.Context, _ json.RawMessage) (string, error) {
	return "", errNotImplemented
}

// ── vps_disk_usage ──────────────────────────────────────────────────────────

// DiskUsageTool will report `du` for a path, sorted descending.
type DiskUsageTool struct{ ssh *SSHClient }

// NewDiskUsageTool constructs a DiskUsageTool stub.
func NewDiskUsageTool(ssh *SSHClient) *DiskUsageTool { return &DiskUsageTool{ssh: ssh} }

func (t *DiskUsageTool) Name() string { return "vps_disk_usage" }

func (t *DiskUsageTool) Description() string {
	return "Returns disk usage by directory under the given path, sorted largest-first. " +
		"Args: path (required, e.g. \"/var\"), depth (default 1)."
}

func (t *DiskUsageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"Absolute path to inspect"},
			"depth":{"type":"integer","minimum":1,"maximum":5,"default":1}
		},
		"required":["path"],
		"additionalProperties":false
	}`)
}

func (t *DiskUsageTool) Call(_ context.Context, _ json.RawMessage) (string, error) {
	return "", errNotImplemented
}
