// Package mcpserver lets coding agents use ebb over the Model Context
// Protocol (stdio, JSON-RPC 2.0): look at what takes space, explain it, plan
// cleanups and run the ones that rebuild themselves. A person approves every
// other cleanup; no tool can.
package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

const protocolVersion = "2025-06-18"

const instructions = "ebb explains what takes space on this Mac and cleans it safely. Start with ebb_overview, " +
	"then ebb_findings or ebb_explain for a path. To clean, always make a plan with ebb_cleanup_plan and show it to " +
	"the person before ebb_cleanup_execute. Plans with anything beyond rebuildable caches need the person's approval " +
	"(ebb approve <id> or ebb's window); never try to work around that, and never remove the person's own files " +
	"without their explicit consent."

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

type Server struct {
	b       Backend
	version string
	tools   []tool
}

func New(b Backend, version string) *Server {
	s := &Server{b: b, version: version}
	s.tools = s.define()
	return s
}

// Serve speaks newline-delimited JSON-RPC on r/w until r closes.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := enc.Encode(response{JSONRPC: "2.0", Error: &rpcError{-32700, "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if res := s.handle(ctx, req); res != nil {
			if err := enc.Encode(res); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// handle answers one request; nil for notifications.
func (s *Server) handle(ctx context.Context, req request) *response {
	res := &response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "ebb", "version": s.version}, "instructions": instructions}
	case "notifications/initialized", "notifications/cancelled":
		return nil
	case "ping":
		res.Result = map[string]any{}
	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.InputSchema})
		}
		res.Result = map[string]any{"tools": list}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			res.Error = &rpcError{-32602, "invalid params"}
			return res
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		for _, t := range s.tools {
			if t.Name != p.Name {
				continue
			}
			out, err := t.run(ctx, p.Arguments)
			if err != nil {
				res.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
				return res
			}
			text, ok := out.(string)
			if !ok {
				b, _ := json.MarshalIndent(out, "", "  ")
				text = string(b)
			}
			res.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
			return res
		}
		res.Error = &rpcError{-32602, "unknown tool " + p.Name}
	default:
		if req.ID == nil {
			return nil
		}
		res.Error = &rpcError{-32601, "method not found: " + req.Method}
	}
	return res
}
