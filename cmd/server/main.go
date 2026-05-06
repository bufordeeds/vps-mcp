// Command server runs the vps-mcp MCP tool server.
//
// By default it speaks JSON-RPC 2.0 over stdio per the Model Context Protocol
// specification. Tools shell out via SSH to a remote Linux VPS using a key
// configured via env vars.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bufordeeds/vps-mcp/internal/mcp"
	"github.com/bufordeeds/vps-mcp/internal/vps"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "vps-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	ssh, err := vps.NewSSHClient(cfg.VPSHost, cfg.SSHKeyPath, logger)
	if err != nil {
		return fmt.Errorf("ssh client: %w", err)
	}

	registry := mcp.NewRegistry()
	registry.Register(vps.NewHealthTool(ssh))
	registry.Register(vps.NewCaddyLogsTool(ssh))
	registry.Register(vps.NewContainerStatusTool(ssh))
	registry.Register(vps.NewDiskUsageTool(ssh))

	server := mcp.NewServer(mcp.ServerConfig{
		Name:     "vps-mcp",
		Version:  version,
		Registry: registry,
		Logger:   logger,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting vps-mcp",
		"transport", cfg.Transport,
		"host", cfg.VPSHost,
		"version", version,
	)

	switch cfg.Transport {
	case "stdio":
		return server.ServeStdio(ctx, os.Stdin, os.Stdout)
	case "http":
		logger.Info("listening", "addr", cfg.ListenAddr)
		return server.ServeHTTP(ctx, cfg.ListenAddr)
	default:
		return fmt.Errorf("unknown transport %q", cfg.Transport)
	}
}

// version is overridden via -ldflags at build time.
var version = "dev"
