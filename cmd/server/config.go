package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	VPSHost    string
	SSHKeyPath string
	Transport  string
	ListenAddr string
	LogLevel   slog.Level
}

func loadConfig() (Config, error) {
	cfg := Config{
		VPSHost:    os.Getenv("VPS_HOST"),
		SSHKeyPath: getenvDefault("VPS_SSH_KEY_PATH", "/etc/vps-mcp/ssh_key"),
		Transport:  getenvDefault("MCP_TRANSPORT", "stdio"),
		ListenAddr: getenvDefault("MCP_LISTEN_ADDR", ":8080"),
	}

	if cfg.VPSHost == "" {
		return cfg, fmt.Errorf("VPS_HOST is required (e.g. user@host)")
	}

	level, err := parseLevel(getenvDefault("LOG_LEVEL", "info"))
	if err != nil {
		return cfg, err
	}
	cfg.LogLevel = level

	return cfg, nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q", s)
	}
}
