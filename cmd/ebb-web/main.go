// Command ebb-web serves the web app on its own, to host it at a public
// address. The page holds nothing: it runs in the visitor's browser and talks
// to ebb on that visitor's own machine.
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/doguyilmaz/ebb/internal/server/assets"
)

func main() {
	listen := flag.String("listen", env("EBB_WEB_LISTEN", "127.0.0.1:8080"), "address to listen on; put a TLS proxy in front")
	check := flag.Bool("health-check", false, "fetch the page from --listen and exit; the container health check")
	flag.Parse()
	if *check {
		os.Exit(healthCheck(*listen))
	}
	if !assets.Built() {
		log.Fatal("ebb-web: this binary carries no web app; build the UI before the binary")
	}
	srv := &http.Server{
		Addr:              *listen,
		Handler:           assets.Handler("/api/"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("ebb-web listening on %s", *listen)
	log.Fatal(srv.ListenAndServe())
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func healthCheck(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	c := http.Client{Timeout: 3 * time.Second}
	res, err := c.Get("http://" + net.JoinHostPort(host, port) + "/")
	if err != nil {
		return 1
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
