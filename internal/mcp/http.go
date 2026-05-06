package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ServeHTTP starts an MCP-over-HTTP (Streamable HTTP) server on addr.
//
// Per the MCP spec, a single endpoint accepts JSON-RPC requests via POST
// and returns the response inline. Notifications (no ID) get a 202.
//
// We don't currently support the optional GET /mcp SSE stream for
// server-initiated messages — our tools are request/response only.
//
// Returns when ctx is canceled (via graceful shutdown) or the listener
// errors out.
func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTP)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen: %w", err)
	}
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// GET on the same path could carry an SSE stream for server-initiated
		// events, but we don't emit any — short-circuit with 405.
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		s.cfg.Logger.Warn("invalid json-rpc payload", "err", err, "remote", r.RemoteAddr)
		writeJSON(w, http.StatusOK, makeError(nil, errParse, "parse error"))
		return
	}

	resp := s.processRequest(r.Context(), req)
	if resp == nil {
		// Notification — no response body.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
