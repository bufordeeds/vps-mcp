package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// CaddyLogsTool fetches recent access logs for a domain from Caddy's
// JSON-formatted log files on the VPS.
//
// Assumes Caddy logs to ~/caddy-logs/*-access.log on the host (one file
// per virtual host) and that the SSH user has passwordless sudo to read
// them. Both hold on the target VPS; adjust constructor params if not.
type CaddyLogsTool struct {
	ssh    *SSHClient
	logDir string
}

// NewCaddyLogsTool constructs a CaddyLogsTool. logDir defaults to
// ~/caddy-logs.
func NewCaddyLogsTool(ssh *SSHClient) *CaddyLogsTool {
	return &CaddyLogsTool{ssh: ssh, logDir: "~/caddy-logs"}
}

func (t *CaddyLogsTool) Name() string { return "vps_caddy_logs" }

func (t *CaddyLogsTool) Description() string {
	return "Fetches recent Caddy access logs for a given domain. Returns " +
		"timestamped requests filtered to that host. Args: domain (required), " +
		"since (e.g. \"1h\", default 1h), limit (default 50)."
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

type caddyLogsArgs struct {
	Domain string `json:"domain"`
	Since  string `json:"since,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// caddyEntry mirrors the subset of Caddy's JSON access-log schema we need.
type caddyEntry struct {
	Ts      float64 `json:"ts"`
	Status  int     `json:"status"`
	Request struct {
		RemoteIP string              `json:"remote_ip"`
		Method   string              `json:"method"`
		Host     string              `json:"host"`
		URI      string              `json:"uri"`
		Headers  map[string][]string `json:"headers"`
	} `json:"request"`
}

func (t *CaddyLogsTool) Call(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := parseCaddyArgs(raw)
	if err != nil {
		return "", err
	}
	cutoff := time.Now().Add(-args.window).Unix()

	out, err := t.ssh.Run(ctx, fmt.Sprintf("sudo cat %s/*-access.log", t.logDir))
	if err != nil {
		return "", fmt.Errorf("read caddy logs: %w", err)
	}

	matches := filterCaddyEntries(out, args.Domain, cutoff)
	return formatCaddyEntries(matches, args), nil
}

// ── helpers (unit-testable, no SSH) ────────────────────────────────────────

type parsedCaddyArgs struct {
	caddyLogsArgs
	window time.Duration
}

func parseCaddyArgs(raw json.RawMessage) (parsedCaddyArgs, error) {
	var p caddyLogsArgs
	if err := json.Unmarshal(raw, &p); err != nil {
		return parsedCaddyArgs{}, fmt.Errorf("parse args: %w", err)
	}
	if err := checkDomain(p.Domain); err != nil {
		return parsedCaddyArgs{}, err
	}
	if p.Since == "" {
		p.Since = "1h"
	}
	if p.Limit == 0 {
		p.Limit = 50
	}
	window, err := time.ParseDuration(p.Since)
	if err != nil {
		return parsedCaddyArgs{}, fmt.Errorf("invalid since %q: %w", p.Since, err)
	}
	if window <= 0 {
		return parsedCaddyArgs{}, fmt.Errorf("since must be positive")
	}
	return parsedCaddyArgs{caddyLogsArgs: p, window: window}, nil
}

// filterCaddyEntries parses the concatenated log files and returns
// entries matching domain that fall after cutoff (unix seconds).
func filterCaddyEntries(raw, domain string, cutoff int64) []caddyEntry {
	var matches []caddyEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e caddyEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if int64(e.Ts) < cutoff {
			continue
		}
		if !strings.EqualFold(e.Request.Host, domain) {
			continue
		}
		matches = append(matches, e)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Ts < matches[j].Ts })
	return matches
}

func formatCaddyEntries(entries []caddyEntry, args parsedCaddyArgs) string {
	if len(entries) == 0 {
		return fmt.Sprintf("No requests to %s in the last %s.", args.Domain, args.Since)
	}
	if len(entries) > args.Limit {
		entries = entries[len(entries)-args.Limit:]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d requests to %s in the last %s (showing %d):\n",
		len(entries), args.Domain, args.Since, len(entries))
	for _, e := range entries {
		ts := time.Unix(int64(e.Ts), 0).UTC().Format("01-02 15:04:05")
		ua := firstHeader(e.Request.Headers, "User-Agent")
		ua = truncate(ua, 60)
		fmt.Fprintf(&b, "  %s  %d  %-6s  %-40s  ip=%s  ua=%q\n",
			ts, e.Status, e.Request.Method,
			truncate(e.Request.URI, 40),
			e.Request.RemoteIP, ua)
	}
	return strings.TrimRight(b.String(), "\n")
}

func firstHeader(h map[string][]string, key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
