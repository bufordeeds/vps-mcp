package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DiskUsageTool reports `du` for a path, sorted descending. Useful for
// answering "why is /var so full?" — `du -h --max-depth=N | sort -rh`
// surfaces the heaviest directories first.
type DiskUsageTool struct {
	ssh *SSHClient
}

// NewDiskUsageTool constructs a DiskUsageTool.
func NewDiskUsageTool(ssh *SSHClient) *DiskUsageTool {
	return &DiskUsageTool{ssh: ssh}
}

func (t *DiskUsageTool) Name() string { return "vps_disk_usage" }

func (t *DiskUsageTool) Description() string {
	return "Returns disk usage by directory under the given absolute path, " +
		"sorted largest-first. Args: path (required, e.g. \"/var\"), " +
		"depth (default 1, max 5)."
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

type diskUsageArgs struct {
	Path  string `json:"path"`
	Depth int    `json:"depth,omitempty"`
}

func (t *DiskUsageTool) Call(ctx context.Context, raw json.RawMessage) (string, error) {
	var p diskUsageArgs
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	if err := checkAbsPath(p.Path); err != nil {
		return "", err
	}
	if p.Depth == 0 {
		p.Depth = 1
	}
	if p.Depth < 1 || p.Depth > 5 {
		return "", fmt.Errorf("depth must be between 1 and 5")
	}
	quoted, err := quoteForShell(p.Path)
	if err != nil {
		return "", err
	}

	// sudo: many interesting paths (/var/log, container volumes) are
	// root-owned. The 2>/dev/null swallows "Permission denied" on
	// individual subdirs so a partial result still surfaces.
	cmd := fmt.Sprintf("sudo du -h --max-depth=%d %s 2>/dev/null | sort -rh | head -30",
		p.Depth, quoted)
	out, err := t.ssh.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("du: %w", err)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return fmt.Sprintf("No output from du %s. Path may be empty or unreadable.", p.Path), nil
	}
	return out, nil
}
