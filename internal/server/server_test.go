package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewUnstartedServer(New(nil, nil).Handler())
	// httptest listens on 127.0.0.1, so the Host header is allowed.
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	var h Health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Version == "" || h.OS != runtime.GOOS || h.Arch != runtime.GOARCH {
		t.Fatalf("health = %+v", h)
	}
}

func TestHealthWrongMethodIs405(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/health", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405", resp.StatusCode)
	}
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var e APIError
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil || e.Error.Code != "not_found" {
		t.Fatalf("body was not a JSON API error: %v %+v", err, e)
	}
}

func TestSPAFallbackServesIndex(t *testing.T) {
	ts := newTestServer(t)
	for _, p := range []string{"/", "/models", "/settings/advanced"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", p, resp.StatusCode)
		}
		if !strings.Contains(strings.ToLower(string(body)), "<!doctype html>") {
			t.Fatalf("%s: did not get index.html, got %.80q", p, body)
		}
	}
}

func TestForeignHostIsRejected(t *testing.T) {
	ts := newTestServer(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	req.Host = "evil.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("status %d, want 421 for a foreign Host header", resp.StatusCode)
	}

	for _, host := range []string{"127.0.0.1:1234", "localhost:1234", "localhost", "127.0.0.1"} {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Host %q: status %d, want 200", host, resp.StatusCode)
		}
	}
}

// TestOnlyTheDaemonsOwnOriginIsAnswered is D-67: a browser-sent Origin
// must be exactly the daemon's own — the Host the request was addressed to
// — not merely some page on this computer. Another program's page on
// another port (a dev server, a local web app, anything a browser can be
// pointed at) is another origin.
func TestOnlyTheDaemonsOwnOriginIsAnswered(t *testing.T) {
	ts := newTestServer(t)
	own := ts.URL // http://127.0.0.1:<port>
	host := strings.TrimPrefix(own, "http://")
	_, port, _ := net.SplitHostPort(host)
	for _, c := range []struct {
		method, path, origin, host string
		want                       int
	}{
		{"GET", "/api/health", "http://evil.example.com", "", http.StatusForbidden},
		{"GET", "/api/health", "http://localhost:5173", "", http.StatusForbidden}, // another port on this computer
		{"GET", "/api/health", "http://127.0.0.1:1", "", http.StatusForbidden},
		{"GET", "/api/health", "https://" + host, "", http.StatusForbidden}, // not the scheme the daemon serves
		{"GET", "/api/health", "null", "", http.StatusForbidden},
		{"GET", "/api/health", "", "", http.StatusOK},  // no Origin: a program, not a page
		{"GET", "/api/health", own, "", http.StatusOK}, // the daemon's own page
		{"GET", "/api/health", "http://localhost:" + port, "localhost:" + port, http.StatusOK},
		{"GET", "/api/health", "http://127.0.0.1:" + port, "localhost:" + port, http.StatusForbidden}, // same machine, another origin
		// A write from elsewhere never reaches its handler.
		{"POST", "/api/models/pull", "http://evil.example.com", "", http.StatusForbidden},
		{"PUT", "/api/settings", "http://localhost:5173", "", http.StatusForbidden},
		{"POST", "/api/data/delete", "http://127.0.0.1:8080", "", http.StatusForbidden},
	} {
		req, _ := http.NewRequest(c.method, ts.URL+c.path, strings.NewReader("{}"))
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		if c.host != "" {
			req.Host = c.host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s %s Origin %q Host %q: status %d, want %d", c.method, c.path, c.origin, c.host, resp.StatusCode, c.want)
		}
	}
}

// A page elsewhere cannot reach the API even with a request that carries
// no Origin (an <img>, a <script>, a link): the browser says where the
// request came from in Sec-Fetch-Site, and only same-origin (the app
// itself) and none (the person typed the address) are answered.
func TestCrossSiteRequestsNeverReachTheAPI(t *testing.T) {
	ts := newTestServer(t)
	for site, want := range map[string]int{
		"cross-site":  http.StatusForbidden,
		"same-site":   http.StatusForbidden, // another port on 127.0.0.1 is the same site
		"same-origin": http.StatusOK,
		"none":        http.StatusOK,
	} {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
		req.Header.Set("Sec-Fetch-Site", site)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Sec-Fetch-Site %s: status %d, want %d", site, resp.StatusCode, want)
		}
	}
	// The app's pages themselves may be opened from anywhere — a
	// notification, a bookmark, a link — they are only its own UI.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/models", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("a cross-site navigation to the UI: status %d, want 200", resp.StatusCode)
	}
}

// No response ever grants another origin access: no CORS header on any
// answer, a preflight included.
func TestNoCORSHeaders(t *testing.T) {
	ts := newTestServer(t)
	for _, c := range []struct{ method, path, origin string }{
		{"GET", "/api/health", ts.URL},
		{"OPTIONS", "/api/health", "http://evil.example.com"},
		{"OPTIONS", "/api/models/pull", ts.URL},
		{"GET", "/", ""},
	} {
		req, _ := http.NewRequest(c.method, ts.URL+c.path, nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		for k := range resp.Header {
			if strings.HasPrefix(strings.ToLower(k), "access-control-") {
				t.Errorf("%s %s answered with %s: %q", c.method, c.path, k, resp.Header.Get(k))
			}
		}
	}
}

// Every answer carries the headers that keep it out of other sites'
// frames and keep the UI from loading or sending anything elsewhere.
func TestSecurityHeaders(t *testing.T) {
	ts := newTestServer(t)
	for _, p := range []string{"/", "/settings", "/api/health", "/api/nope"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		for k, want := range map[string]string{
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
			"Referrer-Policy":              "no-referrer",
			"Cross-Origin-Resource-Policy": "same-origin",
		} {
			if got := resp.Header.Get(k); got != want {
				t.Errorf("%s: %s = %q, want %q", p, k, got, want)
			}
		}
		csp := resp.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "default-src 'none'") {
			t.Errorf("%s: Content-Security-Policy = %q", p, csp)
		}
		if !strings.HasPrefix(p, "/api/") {
			for _, d := range []string{"connect-src 'self'", "script-src 'self'", "form-action 'self'"} {
				if !strings.Contains(csp, d) {
					t.Errorf("%s: the UI's policy lacks %q: %q", p, d, csp)
				}
			}
			if strings.Contains(csp, "unsafe") || strings.Contains(csp, "*") || strings.Contains(csp, "http") {
				t.Errorf("%s: the UI's policy admits more than the daemon itself: %q", p, csp)
			}
		}
	}
}

func TestRequestBodiesAreBounded(t *testing.T) {
	ts := newTestServer(t)
	big := strings.Repeat("x", maxRequestBody+1)
	resp, err := http.Post(ts.URL+"/api/models/pull", "application/json", strings.NewReader(`{"ollama_tag":"`+big+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("an oversized body: status %d, want 400", resp.StatusCode)
	}
}

// TestListenIsLoopbackOnly is product rule 7 as a test: the only listener
// constructor binds to 127.0.0.1, and Serve refuses anything else.
func TestListenIsLoopbackOnly(t *testing.T) {
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if !IsLoopback(l.Addr()) {
		t.Fatalf("Listen bound to %s, want %s", l.Addr(), LoopbackHost)
	}
	if got := l.Addr().(*net.TCPAddr).IP.String(); got != LoopbackHost {
		t.Fatalf("Listen bound to %s, want %s", got, LoopbackHost)
	}
}

func TestServeRefusesNonLoopbackListener(t *testing.T) {
	// A listener on the wildcard address — what a bind flag would allow.
	l, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Skip("cannot bind 0.0.0.0 here:", err)
	}
	err = New(nil, nil).Serve(context.Background(), l)
	if !errors.Is(err, ErrNotLoopback) {
		t.Fatalf("Serve on 0.0.0.0 returned %v, want ErrNotLoopback", err)
	}
}

func TestServeAndShutdown(t *testing.T) {
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(nil, nil).Serve(ctx, l) }()

	resp, err := http.Get("http://" + l.Addr().String() + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after the context was cancelled")
	}
}
