package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// Protocol-level constants. We pin to the version we've tested against.
const protocolVersion = "2024-11-05"

// JSON-RPC 2.0 standard error codes.
const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// ServerConfig configures the MCP server.
type ServerConfig struct {
	Name     string
	Version  string
	Registry *Registry
	Logger   *slog.Logger
}

// Server is a minimal MCP server.
type Server struct {
	cfg ServerConfig
}

// NewServer constructs a Server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Server{cfg: cfg}
}

// ServeStdio reads JSON-RPC messages from r (one per line) and writes
// responses to w. It returns when ctx is canceled or r returns EOF.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// Allow large messages — tools/call results can be sizeable (logs, etc.).
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	enc := json.NewEncoder(w)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.cfg.Logger.Warn("invalid json-rpc payload", "err", err)
			if encErr := enc.Encode(makeError(nil, errParse, "parse error")); encErr != nil && !errors.Is(encErr, io.ErrClosedPipe) {
				s.cfg.Logger.Error("write parse error", "err", encErr)
			}
			continue
		}

		s.handle(ctx, enc, req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	return nil
}

// processRequest dispatches a single JSON-RPC request and returns the
// response to write. Notifications (requests with no ID) return nil so
// transports can omit a response entirely.
func (s *Server) processRequest(ctx context.Context, req request) *response {
	if req.JSONRPC != "2.0" {
		return makeError(req.ID, errInvalidRequest, "jsonrpc must be \"2.0\"")
	}

	switch req.Method {
	case "initialize":
		return makeResult(req.ID, initializeResult{
			ProtocolVersion: protocolVersion,
			ServerInfo: serverInfo{
				Name:    s.cfg.Name,
				Version: s.cfg.Version,
			},
			Capabilities: capabilities{Tools: &toolsCapability{}},
		})

	case "tools/list":
		tools := s.cfg.Registry.List()
		descriptors := make([]toolDescriptor, len(tools))
		for i, t := range tools {
			descriptors[i] = toolDescriptor{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: t.InputSchema(),
			}
		}
		return makeResult(req.ID, toolsListResult{Tools: descriptors})

	case "tools/call":
		var p toolsCallParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return makeError(req.ID, errInvalidParams, "invalid params")
		}
		tool := s.cfg.Registry.Get(p.Name)
		if tool == nil {
			return makeError(req.ID, errMethodNotFound, "tool not found: "+p.Name)
		}

		text, err := tool.Call(ctx, p.Arguments)
		if err != nil {
			// Tool errors are returned as a successful response with isError=true
			// per the MCP spec — that lets the LLM see the error and try again.
			return makeResult(req.ID, toolsCallResult{
				Content: []content{{Type: "text", Text: err.Error()}},
				IsError: true,
			})
		}
		return makeResult(req.ID, toolsCallResult{
			Content: []content{{Type: "text", Text: text}},
		})

	case "notifications/initialized", "notifications/cancelled":
		// Notifications never carry an ID and never get a response.
		return nil

	default:
		if req.ID == nil {
			return nil
		}
		return makeError(req.ID, errMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) handle(ctx context.Context, enc *json.Encoder, req request) {
	resp := s.processRequest(ctx, req)
	if resp == nil {
		return
	}
	if err := enc.Encode(resp); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		s.cfg.Logger.Error("write response", "err", err)
	}
}

func makeResult(id json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

func makeError(id json.RawMessage, code int, message string) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
}

// ── wire types ──────────────────────────────────────────────────────────────

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Capabilities    capabilities `json:"capabilities"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type toolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDescriptor `json:"tools"`
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolsCallResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
