// Package assets carries the built web frontend embedded in the binary.
package assets

import (
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: keeps _headers, which embed leaves out by default.
//
//go:embed all:dist
var distFS embed.FS

// FS is the bundle's root (index.html, assets/…).
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return distFS
	}
	return sub
}

// Built reports whether a bundle was embedded into this binary. Source builds
// without scripts/build-ui.sh only carry dist/README.
func Built() bool {
	_, err := fs.Stat(FS(), "index.html")
	return err == nil
}

// UnbuiltPage explains a binary compiled without the frontend.
const UnbuiltPage = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>ebb</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;font:15px/1.5 system-ui,sans-serif;background:#0a0d12;color:#e7ecf2}
main{max-width:34rem;padding:2rem;border:1px solid #1e2833;border-radius:10px;background:#10161e}h1{font-size:1.1rem;margin:0 0 .6rem}
code{background:#1a2330;padding:2px 6px;border-radius:4px;font-size:.9em}</style></head><body><main>
<h1>This build has no web UI</h1>
<p>The binary was built without the bundled interface. Release builds (<code>brew install doguyilmaz/tap/ebb</code>,
GitHub Releases) include it. From source: <code>make build</code>.</p>
<p>The CLI and the JSON API under <code>/api/v1</code> work as usual.</p></main></body></html>
`

// Handler serves the single-page app: an embedded file when the path names one,
// index.html for every client route. Paths under a reserved prefix (an API the
// caller mounts itself) get 404 instead of the app shell.
func Handler(reserved ...string) http.Handler {
	assets := FS()
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for _, rl := range headerRules {
			if rl.matches(r.URL.Path) {
				for _, kv := range rl.headers {
					h.Set(kv[0], kv[1])
				}
			}
		}
		for _, p := range reserved {
			if strings.HasPrefix(r.URL.Path, p) {
				http.NotFound(w, r)
				return
			}
		}
		if !Built() {
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, UnbuiltPage)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(assets, name); err != nil || name == "_headers" {
			r.URL.Path = "/"
			name = "index.html"
		}
		// Files without a rule of their own (the shell, icons, fonts) keep
		// their names across builds, so they are checked every time.
		if h.Get("Cache-Control") == "" {
			h.Set("Cache-Control", "no-cache")
		}
		if gz := gzipFor(w, r, name); gz != nil {
			defer gz.Close()
			w = gz
		}
		files.ServeHTTP(w, r)
	})
}

// compressible names the types worth gzipping. Fonts and images arrive already
// compressed, and running them through gzip only costs time on both ends.
func compressible(name string) bool {
	switch path.Ext(name) {
	case ".js", ".css", ".html", ".json", ".svg", ".map", ".txt":
		return true
	}
	return false
}

// gzipWriter compresses the body and keeps Content-Length off the response,
// which would otherwise describe the uncompressed size.
type gzipWriter struct {
	http.ResponseWriter
	z *gzip.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) { return g.z.Write(b) }
func (g *gzipWriter) Close() error                { return g.z.Close() }

// gzipFor returns a compressing writer when the client accepts one and the
// content is worth compressing, else nil.
func gzipFor(w http.ResponseWriter, r *http.Request, name string) *gzipWriter {
	if !compressible(name) || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return nil
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Del("Content-Length")
	return &gzipWriter{ResponseWriter: w, z: gzip.NewWriter(w)}
}

// headerRule is one block of _headers: a path (a trailing * matches
// everything below it) and the headers responses to it carry.
type headerRule struct {
	path    string
	headers [][2]string
}

func (r headerRule) matches(path string) bool {
	if prefix, ok := strings.CutSuffix(r.path, "*"); ok {
		return strings.HasPrefix(path, prefix)
	}
	return path == r.path
}

var headerRules = func() []headerRule {
	b, err := fs.ReadFile(FS(), "_headers")
	if err != nil {
		return nil
	}
	return parseHeaders(string(b))
}()

// parseHeaders reads the _headers format Cloudflare uses: a path on its own
// line, then indented "Name: value" lines; # starts a comment.
func parseHeaders(text string) []headerRule {
	var rules []headerRule
	for line := range strings.Lines(text) {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case line[0] != ' ' && line[0] != '\t':
			rules = append(rules, headerRule{path: trimmed})
		case len(rules) > 0:
			if k, v, ok := strings.Cut(trimmed, ":"); ok {
				last := &rules[len(rules)-1]
				last.headers = append(last.headers, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
			}
		}
	}
	return rules
}
