package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServeWaitsForAnInFlightOperationAtShutdown(t *testing.T) {
	c := newClient(t)
	entered, release := make(chan struct{}), make(chan struct{})
	requestContext := make(chan context.Context, 1)
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	c.s.mux.HandleFunc("POST /finish-item", func(w http.ResponseWriter, r *http.Request) {
		requestContext <- r.Context()
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, served := make(chan string, 1), make(chan error, 1)
	go func() {
		served <- Serve(ctx, c.s, "127.0.0.1:0", func(url string) { ready <- strings.Split(url, "/#")[0] })
	}()
	url := <-ready
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		res, err := http.Post(url+"/finish-item", "application/json", nil)
		if err == nil {
			res.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the operation did not start")
	}
	cancel()
	activeContext := <-requestContext
	select {
	case <-activeContext.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("the in-flight request did not see cancellation")
	}
	select {
	case err := <-served:
		t.Fatalf("Serve returned before the operation finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	released = true
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not finish after the operation")
	}
	<-requestDone
}
