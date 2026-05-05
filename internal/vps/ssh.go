// Package vps holds the SSH client and the per-tool implementations that
// shell out to the remote VPS.
package vps

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient executes commands on a remote VPS over SSH.
//
// It is safe for concurrent use; each Run dials a fresh connection. We
// deliberately don't pool connections — tool invocations are infrequent
// and short-lived, and per-call dials make timeouts and cancellation
// straightforward.
type SSHClient struct {
	user    string
	addr    string
	signer  ssh.Signer
	timeout time.Duration
	logger  *slog.Logger
}

// NewSSHClient parses host (in the form user@host[:port]) and loads the
// private key from keyPath.
func NewSSHClient(host, keyPath string, logger *slog.Logger) (*SSHClient, error) {
	user, addr, err := parseHost(host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key: %w", err)
	}

	return &SSHClient{
		user:    user,
		addr:    addr,
		signer:  signer,
		timeout: 15 * time.Second,
		logger:  logger,
	}, nil
}

// Run executes cmd on the remote host. It honors ctx cancellation.
//
// stderr is captured and included in the returned error if cmd exits non-zero.
func (c *SSHClient) Run(ctx context.Context, cmd string) (string, error) {
	c.logger.Debug("ssh run", "cmd", cmd)

	dialer := &net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", c.addr, err)
	}
	defer conn.Close()

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, c.addr, &ssh.ClientConfig{
		User:            c.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(c.signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: pin host key via known_hosts
		Timeout:         c.timeout,
	})
	if err != nil {
		return "", fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(clientConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	// Cancel the session if the caller context is canceled.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGTERM)
			_ = client.Close()
		case <-done:
		}
	}()
	defer close(done)

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("run %q: %w (stderr: %s)", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// parseHost splits user@host[:port] into user and host:port.
func parseHost(s string) (user, addr string, err error) {
	at := strings.Index(s, "@")
	if at < 0 {
		return "", "", fmt.Errorf("expected user@host, got %q", s)
	}
	user, host := s[:at], s[at+1:]
	if user == "" || host == "" {
		return "", "", fmt.Errorf("user and host required in %q", s)
	}
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	return user, host, nil
}
