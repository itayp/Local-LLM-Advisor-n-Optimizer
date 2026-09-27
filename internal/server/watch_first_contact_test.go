package server

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"advisor/internal/catalog/hf"
)

// countingTransport counts the requests that would have left the machine.
type countingTransport struct {
	n    atomic.Int32
	next http.RoundTripper
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return c.next.RoundTrip(r)
}

// Product rule 5, D-54, D-68: the daily check never makes the first
// contact. On an install whose model list the person has never fetched, a
// scheduled check sends nothing anywhere; once they have fetched it with
// the button, the watch keeps it current as Settings says it will.
func TestTheScheduledWatchNeverMakesFirstContact(t *testing.T) {
	srv, _ := newCatalogTestServer(t, &fakeBackend{name: "ollama"})
	if err := srv.SyncCatalogue(context.Background()); err != nil {
		t.Fatal(err)
	}
	// This test is about the network, not the machine: no hardware to wait for.
	srv.publishHardware(HardwareResponse{}, errors.New("not read in this test"))
	counter := &countingTransport{next: http.DefaultTransport}
	build := srv.cat.newClient
	srv.cat.newClient = func() *hf.Client {
		c := build()
		counter.next = c.HTTP.Transport
		c.HTTP.Transport = counter
		return c
	}

	_, err := srv.RunWatch(context.Background(), "scheduler")
	if !errors.Is(err, ErrNotFetchedYet) {
		t.Fatalf("a scheduled check on a fresh install: %v, want ErrNotFetchedYet", err)
	}
	if n := counter.n.Load(); n != 0 {
		t.Fatalf("a scheduled check on a fresh install sent %d requests", n)
	}

	// The person fetches the list (the button, trigger "api").
	if _, err := srv.RefreshCatalog(context.Background(), "api"); err != nil {
		t.Fatal(err)
	}
	before := counter.n.Load()
	if before == 0 {
		t.Fatal("the fetch itself sent nothing; the test is not counting")
	}
	if _, err := srv.RunWatch(context.Background(), "scheduler"); errors.Is(err, ErrNotFetchedYet) {
		t.Fatalf("after the first fetch the scheduled check still waits: %v", err)
	}
}
