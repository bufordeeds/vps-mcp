package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// HealthTool reports a snapshot of system health: uptime, load, disk and
// memory pressure, and the top running containers by CPU.
type HealthTool struct {
	ssh *SSHClient
}

// NewHealthTool constructs a HealthTool.
func NewHealthTool(ssh *SSHClient) *HealthTool {
	return &HealthTool{ssh: ssh}
}

func (t *HealthTool) Name() string { return "vps_health" }

func (t *HealthTool) Description() string {
	return "Returns a snapshot of VPS health: uptime, load average, disk usage, " +
		"memory usage, and the running Docker containers. Use this when the user " +
		"asks anything like \"is the VPS healthy?\" or \"how's the server doing?\"."
}

func (t *HealthTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
}

// healthCmd combines several lightweight commands into one SSH round trip.
const healthCmd = `printf '=== uptime ===\n'; uptime; ` +
	`printf '\n=== disk ===\n'; df -h --output=source,size,used,avail,pcent,target -x tmpfs -x devtmpfs; ` +
	`printf '\n=== memory ===\n'; free -h; ` +
	`printf '\n=== containers ===\n'; docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' 2>/dev/null || echo '(docker not available)'`

func (t *HealthTool) Call(ctx context.Context, _ json.RawMessage) (string, error) {
	out, err := t.ssh.Run(ctx, healthCmd)
	if err != nil {
		return "", fmt.Errorf("collect health: %w", err)
	}
	return strings.TrimSpace(out), nil
}
