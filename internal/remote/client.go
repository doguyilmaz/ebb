// Package remote talks to a running ebb server (ebb serve or the daemon)
// with this machine's token, for the CLI and the MCP server.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/sys"
)

type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

var ErrNoServer = errors.New("ebb is not running; start it with: ebb serve")

// Discover finds the running server through its status file.
func Discover(env sys.Env) (*Client, error) {
	st, ok := server.Live(env)
	if !ok {
		return nil, ErrNoServer
	}
	tok, err := server.LoadOrCreateToken(env)
	if err != nil {
		return nil, err
	}
	return &Client{Base: st.URL(), Token: tok, HTTP: &http.Client{Timeout: 15 * time.Minute}}, nil
}

// Do sends a JSON request to /api/v1<path> and decodes the response into out.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		var e struct{ Error string }
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("ebb server: %s", resp.Status)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
