package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ContainerStatusTool reports `docker ps` for one or all containers.
type ContainerStatusTool struct {
	ssh *SSHClient
}

// NewContainerStatusTool constructs a ContainerStatusTool.
func NewContainerStatusTool(ssh *SSHClient) *ContainerStatusTool {
	return &ContainerStatusTool{ssh: ssh}
}

func (t *ContainerStatusTool) Name() string { return "vps_container_status" }

func (t *ContainerStatusTool) Description() string {
	return "Returns Docker container status (name, state, image, ports). " +
		"Args: name (optional, filters to containers matching this name fragment); " +
		"all (optional, also include stopped containers)."
}

func (t *ContainerStatusTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"name":{"type":"string","description":"Optional container name filter (substring match)"},
			"all":{"type":"boolean","default":false,"description":"Include stopped containers"}
		},
		"additionalProperties":false
	}`)
}

type containerStatusArgs struct {
	Name string `json:"name,omitempty"`
	All  bool   `json:"all,omitempty"`
}

func (t *ContainerStatusTool) Call(ctx context.Context, raw json.RawMessage) (string, error) {
	var p containerStatusArgs
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	cmd := `docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}\t{{.Ports}}'`
	if p.All {
		cmd += " --all"
	}
	if p.Name != "" {
		if err := checkContainerName(p.Name); err != nil {
			return "", err
		}
		quoted, err := quoteForShell(p.Name)
		if err != nil {
			return "", err
		}
		cmd += " --filter name=" + quoted
	}

	out, err := t.ssh.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("docker ps: %w", err)
	}
	out = strings.TrimSpace(out)
	if out == "" || strings.Count(out, "\n") == 0 {
		// Only the header row, or nothing.
		filt := ""
		if p.Name != "" {
			filt = fmt.Sprintf(" matching %q", p.Name)
		}
		return fmt.Sprintf("No running containers%s.", filt), nil
	}
	return out, nil
}
