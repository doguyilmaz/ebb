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

//go:embed dist
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

// Policy is the app's content security policy. The page talks to the origin
// it came from, or, hosted elsewhere, to ebb on the visitor's own loopback.
const Policy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; " +
	"connect-src 'self' http://127.0.0.1:*; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// Handler serves the single-page app: an embedded file when the path names one,
// index.html for every client route. Paths under a reserved prefix (an API the
// caller mounts itself) get 404 instead of the app shell.
func Handler(reserved ...string) http.Handler {
	assets := FS()
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", Policy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
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
		if _, err := fs.Stat(assets, name); err != nil {
			r.URL.Path = "/"
			name = "index.html"
		}
		// The build stamps a content hash into every name under assets/, so
		// those can be kept forever; a new build asks for new names. Files
		// copied as they are (the shell, icons, fonts) keep their names.
		if strings.HasPrefix(name, "assets/") {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
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
