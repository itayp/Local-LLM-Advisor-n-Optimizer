package egress

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAllowedAdmitsTheListAndNothingElse(t *testing.T) {
	cases := []struct {
		p    Purpose
		url  string
		want bool
	}{
		// The list, as each purpose uses it.
		{ModelList, "https://huggingface.co/api/models/bartowski/x?blobs=true", true},
		{ModelList, "https://cdn-lfs.huggingface.co/repos/ab/cd/file", true},
		{ModelList, "https://cas-bridge.xethub.hf.co/xet-bridge-us/abc", true},
		{PublicScores, "https://huggingface.co/datasets/lmarena-ai/leaderboard-dataset/resolve/main/x.parquet", true},
		{PublicScores, "https://us.aws.cdn.hf.co/x", true},
		{PublicScores, "https://epoch.ai/data/benchmark_data.zip", true},
		{OllamaDownload, "https://ollama.com/download/Ollama.dmg", true},
		{OllamaDownload, "https://github.com/ollama/ollama/releases/download/v0.34.4/sha256sum.txt", true},
		{OllamaDownload, "https://release-assets.githubusercontent.com/github-production-release-asset/1/2?sig=x", true},
		{UpdateCheck, "https://api.github.com/repos/itayp/-Local-LLM-Advisor-n-Optimizer/releases/latest", true},

		// Not HTTPS, credentials, another port.
		{ModelList, "http://huggingface.co/api/models/x", false},
		{ModelList, "https://user:secret@huggingface.co/api/models/x", false},
		{ModelList, "https://huggingface.co:8443/api/models/x", false},
		// Look-alikes are not subdomains.
		{ModelList, "https://evilhuggingface.co/x", false},
		{ModelList, "https://huggingface.co.evil.example/x", false},
		{PublicScores, "https://epoch.ai.evil.example/x", false},
		{PublicScores, "https://sub.epoch.ai/x", false}, // no Subdomains on this entry
		// A host that serves many parties is narrowed to the one project.
		{OllamaDownload, "https://github.com/someone/else/releases/download/v1/x", false},
		{OllamaDownload, "https://ollama.com/library/llama3", false},
		{UpdateCheck, "https://api.github.com/user", false},
		// Purposes do not borrow each other's hosts.
		{UpdateCheck, "https://huggingface.co/api/models/x", false},
		{ModelList, "https://api.github.com/repos/itayp/-Local-LLM-Advisor-n-Optimizer/releases/latest", false},
		{ModelList, "https://epoch.ai/data/benchmark_data.zip", false},
		{PublicScores, "https://ollama.com/download/Ollama.dmg", false},
		// Everything else.
		{ModelList, "https://example.com/", false},
		{ModelList, "https://127.0.0.1/", false},
		{Purpose("telemetry"), "https://huggingface.co/", false},
	}
	for _, c := range cases {
		err := Allowed(c.p, mustURL(t, c.url))
		if got := err == nil; got != c.want {
			t.Errorf("Allowed(%s, %s) = %v, want allowed=%v", c.p, c.url, err, c.want)
		}
		if err != nil && !errors.Is(err, ErrNotAllowed) {
			t.Errorf("Allowed(%s, %s): error %v does not wrap ErrNotAllowed", c.p, c.url, err)
		}
	}
}

func TestTheListIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Destinations() {
		if d.Host == "" || d.Host != strings.ToLower(d.Host) || strings.ContainsAny(d.Host, ":/*") {
			t.Errorf("host %q must be a bare lower-case host name", d.Host)
		}
		if len(d.For) == 0 {
			t.Errorf("%s serves no purpose", d.Host)
		}
		if strings.TrimSpace(d.What) == "" {
			t.Errorf("%s does not say what is fetched from it", d.Host)
		}
		if d.PathPrefix != "" && !strings.HasPrefix(d.PathPrefix, "/") {
			t.Errorf("%s: path prefix %q must start with /", d.Host, d.PathPrefix)
		}
		key := d.Host + d.PathPrefix
		if seen[key] {
			t.Errorf("%s listed twice", key)
		}
		seen[key] = true
	}
}

// A client refuses before anything is sent: the server never sees the
// request.
func TestClientRefusesBeforeSending(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	c := Client(ModelList, 0)
	resp, err := c.Get(srv.URL + "/api/models/x")
	if err == nil {
		resp.Body.Close()
		t.Fatal("a request to a host not on the list was sent")
	}
	if !errors.Is(err, ErrNotAllowed) {
		t.Errorf("error %v does not wrap ErrNotAllowed", err)
	}
	if hits.Load() != 0 {
		t.Errorf("the server saw %d requests", hits.Load())
	}
}

// fakeNet answers an allow-listed URL with a redirect to wherever the test
// says, and records every request that reached it.
type fakeNet struct {
	location string
	seen     []string
}

func (f *fakeNet) RoundTrip(req *http.Request) (*http.Response, error) {
	f.seen = append(f.seen, req.URL.String())
	h := http.Header{}
	h.Set("Location", f.location)
	return &http.Response{StatusCode: http.StatusFound, Header: h, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

// A redirect is a request like any other: one to a host off the list is
// refused by the transport even when the client's own redirect policy is
// replaced (as internal/catalog/hf and internal/catalog/external do).
func TestRedirectOffTheListIsRefused(t *testing.T) {
	for _, loc := range []string{
		"https://evil.example/steal",
		"http://cdn-lfs.huggingface.co/x", // a downgrade to plain HTTP
		"https://api.github.com/repos/itayp/-Local-LLM-Advisor-n-Optimizer/releases/latest", // another purpose's host
	} {
		fake := &fakeNet{location: loc}
		c := &http.Client{Transport: &guard{purpose: ModelList, next: fake}}
		resp, err := c.Get("https://huggingface.co/api/models/x")
		if err == nil {
			resp.Body.Close()
			t.Errorf("redirect to %s was followed", loc)
			continue
		}
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("redirect to %s: error %v does not wrap ErrNotAllowed", loc, err)
		}
		if len(fake.seen) != 1 {
			t.Errorf("redirect to %s: %d requests left, want only the first: %v", loc, len(fake.seen), fake.seen)
		}
	}
	// And the client's own redirect policy says the same.
	c := Client(OllamaDownload, 0)
	req, _ := http.NewRequest(http.MethodGet, "https://evil.example/x", nil)
	prev, _ := http.NewRequest(http.MethodGet, "https://ollama.com/download/Ollama.dmg", nil)
	if err := c.CheckRedirect(req, []*http.Request{prev}); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("CheckRedirect to a foreign host = %v, want ErrNotAllowed", err)
	}
}

func TestClientVerifiesTLS(t *testing.T) {
	tr := internetTransport()
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("the internet transport must verify certificates")
	}
	if tr.TLSClientConfig.MinVersion < 0x0303 {
		t.Errorf("minimum TLS version %x, want TLS 1.2 or later", tr.TLSClientConfig.MinVersion)
	}
}

func TestLocalReachesThisComputer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer srv.Close()
	resp, err := Local(0).Get(srv.URL)
	if err != nil {
		t.Fatalf("Local could not reach a loopback server: %v", err)
	}
	resp.Body.Close()
	// "localhost" is this computer too.
	u := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	if resp, err := Local(0).Get(u); err != nil {
		t.Errorf("Local could not reach %s: %v", u, err)
	} else {
		resp.Body.Close()
	}
}

func TestLocalRefusesAnotherComputer(t *testing.T) {
	for _, u := range []string{
		"http://example.com:11434/api/version",
		"http://192.0.2.10:11434/api/version", // TEST-NET-1: never routed
		"http://0.0.0.0:11434/api/version",
		"ftp://127.0.0.1/x",
	} {
		resp, err := Local(0).Get(u)
		if err == nil {
			resp.Body.Close()
			t.Errorf("Local reached %s", u)
			continue
		}
		if !errors.Is(err, ErrNotLocal) {
			t.Errorf("%s: error %v does not wrap ErrNotLocal", u, err)
		}
	}
}

// The dial check runs on the resolved address: a name that passed the URL
// check but resolved elsewhere would still be refused at the socket.
func TestLocalDialRefusesANonLoopbackAddress(t *testing.T) {
	dial := Local(0).Transport.(*localGuard).next.(*http.Transport).DialContext
	for _, addr := range []string{"192.0.2.10:11434", "0.0.0.0:11434"} {
		conn, err := dial(context.Background(), "tcp", addr)
		if err == nil {
			conn.Close()
			t.Errorf("dialled %s", addr)
			continue
		}
		if !errors.Is(err, ErrNotLocal) {
			t.Errorf("dial %s: %v does not wrap ErrNotLocal", addr, err)
		}
	}
}

func TestLocalNeverUsesAProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.example:3128")
	t.Setenv("HTTPS_PROXY", "http://proxy.example:3128")
	tr := Local(0).Transport.(*localGuard).next.(*http.Transport)
	if tr.Proxy != nil {
		t.Fatal("the local transport must not have a proxy function")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1": true, "127.1.2.3": true, "::1": true, "[::1]": true, "localhost": true, "LOCALHOST": true,
		"0.0.0.0": false, "::": false, "192.168.1.5": false, "example.com": false, "localhost.example.com": false, "": false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
