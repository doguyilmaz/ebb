// Package mcpserver lets coding agents use ebb over the Model Context
// Protocol (stdio, JSON-RPC 2.0): look at what takes space, explain it, plan
// cleanups and run the ones that rebuild themselves. A person approves every
// other cleanup; no tool can.
package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

const protocolVersion = "2025-06-18"

const instructions = "ebb explains what takes space on this Mac and cleans it safely. Start with ebb_overview, " +
	"then ebb_findings or ebb_explain for a path. To clean, always make a plan with ebb_cleanup_plan and show it to " +
	"the person before ebb_cleanup_execute. Plans with anything beyond rebuildable caches need the person's approval " +
	"(ebb's window shows them the plan, or they run ebb approve <id>); never try to work around that, never approve or " +
	"confirm on their behalf, and never remove the person's own files " +
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

const maxLine = 16 << 20

var errTooLong = errors.New("request too large")

// Serve speaks newline-delimited JSON-RPC on r/w until r closes, then lets
// calls in flight finish. Tool calls run side by side, each stopped by
// notifications/cancelled or ctx; everything else is answered in order.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	var mu sync.Mutex
	enc := json.NewEncoder(w)
	send := func(res *response) error {
		mu.Lock()
		defer mu.Unlock()
		return enc.Encode(res)
	}
	calls := map[string]context.CancelFunc{}
	var cmu sync.Mutex
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := readLine(br)
		if errors.Is(err, errTooLong) {
			if err := send(&response{JSONRPC: "2.0", ID: null, Error: &rpcError{-32600, err.Error()}}); err != nil {
				return err
			}
			continue
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var req request
			if jerr := json.Unmarshal(line, &req); jerr != nil {
				if err := send(&response{JSONRPC: "2.0", ID: null, Error: &rpcError{-32700, "parse error"}}); err != nil {
					return err
				}
			} else if req.Method == "notifications/cancelled" {
				var p struct {
					RequestID json.RawMessage `json:"requestId"`
				}
				_ = json.Unmarshal(req.Params, &p)
				cmu.Lock()
				if stop, ok := calls[string(p.RequestID)]; ok {
					stop()
				}
				cmu.Unlock()
			} else if req.Method == "tools/call" && req.ID != nil {
				cctx, stop := context.WithCancel(ctx)
				key := string(req.ID)
				cmu.Lock()
				calls[key] = stop
				cmu.Unlock()
				wg.Go(func() {
					res := s.handle(cctx, req)
					cmu.Lock()
					delete(calls, key)
					cmu.Unlock()
					// A cancelled request gets no answer.
					if cctx.Err() == nil {
						_ = send(res)
					}
					stop()
				})
			} else if res := s.handle(ctx, req); res != nil {
				if err := send(res); err != nil {
					return err
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

var null = json.RawMessage("null")

// readLine reads one line; a longer one than maxLine is skipped to its end.
func readLine(br *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := br.ReadSlice('\n')
		if len(line)+len(chunk) > maxLine {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = br.ReadSlice('\n')
			}
			if err != nil && err != io.EOF {
				return nil, err
			}
			return nil, errTooLong
		}
		line = append(line, chunk...)
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, err
		}
	}
}

// handle answers one request; nil for notifications.
func (s *Server) handle(ctx context.Context, req request) *response {
	res := &response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "ebb", "version": s.version}, "instructions": instructions}
	case "notifications/initialized":
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
