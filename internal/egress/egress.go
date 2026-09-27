// Package egress is the advisor's outbound network, all of it: the one
// allow-list (hosts.go — the audit), the only HTTP clients that may reach
// another computer, and the only client that may reach the runtime on this
// one. Product rule 7 ("no prompt, no file, no usage data leaves the
// machine") and ARCHITECTURE.md D-10 ("every outbound host is on an
// allow-list in one file; the list is the audit") are enforced here, at the
// transport, rather than by each caller remembering to check:
//
//   - Client(purpose) refuses, before a byte is sent, any request that is
//     not HTTPS to a host hosts.go lists for that purpose — the first
//     request and every redirect, because every hop goes through the same
//     transport. TLS certificates are always verified; nothing here can
//     turn that off.
//   - Local() dials loopback addresses only. A name that resolves anywhere
//     else is refused at the socket, and no proxy is ever used for it.
//
// internal/archtest reads the source of every other package and fails the
// build if one builds its own http.Client or Transport, dials a socket,
// touches http.DefaultClient, or shells out to a download tool. A package
// that needs the network takes its client from here.
package egress

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrNotAllowed is the error a request gets when its destination is not on
// the allow-list for the client's purpose (or is not HTTPS).
var ErrNotAllowed = errors.New("egress: destination not on the advisor's allow-list")

// ErrNotLocal is the error a Local request gets when its address is not
// this computer.
var ErrNotLocal = errors.New("egress: only this computer (a loopback address) may be reached")

// maxRedirects bounds a redirect chain. Ollama's download is three hops
// (ollama.com → the latest release → the pinned release → GitHub's CDN).
const maxRedirects = 6

// Destinations returns a copy of the allow-list, for the report and tests.
func Destinations() []Destination {
	out := make([]Destination, len(allowList))
	for i, d := range allowList {
		d.For = append([]Purpose(nil), d.For...)
		out[i] = d
	}
	return out
}

// Allowed reports whether a request to u may be made for purpose p: HTTPS,
// no credentials in the URL, the default port, and a host (and path, where
// the entry names one) hosts.go lists for p.
func Allowed(p Purpose, u *url.URL) error {
	if u == nil {
		return fmt.Errorf("%w: no URL", ErrNotAllowed)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: %s is not https", ErrNotAllowed, redact(u))
	}
	if u.User != nil {
		return fmt.Errorf("%w: a URL carrying credentials", ErrNotAllowed)
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("%w: %s asks for port %s", ErrNotAllowed, redact(u), port)
	}
	host := strings.ToLower(u.Hostname())
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, d := range allowList {
		if !serves(d, p) || !hostMatches(d, host) {
			continue
		}
		if d.PathPrefix != "" && !strings.HasPrefix(path, d.PathPrefix) {
			continue
		}
		return nil
	}
	return fmt.Errorf("%w: %s (for %s)", ErrNotAllowed, redact(u), p)
}

// AllowedHost reports whether host is on the allow-list for p at all (any
// path). Packages that keep a narrower list of their own (internal/catalog/
// hf, internal/catalog/external) check theirs against this one.
func AllowedHost(p Purpose, host string) bool {
	host = strings.ToLower(host)
	for _, d := range allowList {
		if serves(d, p) && hostMatches(d, host) {
			return true
		}
	}
	return false
}

func serves(d Destination, p Purpose) bool {
	for _, q := range d.For {
		if q == p {
			return true
		}
	}
	return false
}

func hostMatches(d Destination, host string) bool {
	return host == d.Host || (d.Subdomains && strings.HasSuffix(host, "."+d.Host))
}

// redact is a URL for an error message: scheme, host and path, never the
// query (GitHub's CDN links carry signatures there).
func redact(u *url.URL) string {
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// Client returns an HTTP client for purpose p. Every request it sends, the
// redirects included, is checked against the allow-list before it leaves;
// TLS is verified (TLS 1.2 or later). A proxy the computer is configured to
// use (HTTPS_PROXY) is honoured: the request is still end-to-end TLS to the
// allow-listed host. timeout 0 means no overall deadline (a download that
// takes as long as it takes); the caller's context still bounds it.
func Client(p Purpose, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &guard{purpose: p, next: internetTransport()},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("egress: more than %d redirects from %s", maxRedirects, redact(via[0].URL))
			}
			return Allowed(p, req.URL)
		},
	}
}

// guard is the transport every internet client uses: the allow-list, then
// the real transport.
type guard struct {
	purpose Purpose
	next    http.RoundTripper
}

func (g *guard) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := Allowed(g.purpose, req.URL); err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	return g.next.RoundTrip(req)
}

func internetTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return t
}

// IsLoopbackHost reports whether host (a name or an IP literal, without a
// port) names this computer: "localhost" or a loopback IP. The unspecified
// addresses (0.0.0.0, ::) are not: a caller that means "this computer" by
// them says so itself.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Local returns an HTTP client for services on this computer — the
// runtime's API, and the advisor's own daemon for its command-line tools.
// It never uses a proxy, and it dials loopback addresses only: the check
// runs on the resolved address at the socket, so a name that resolves to
// another machine is refused before a connection is made. timeout 0 means
// no overall deadline.
func Local(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrNotLocal, address)
			}
			if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
				return fmt.Errorf("%w: %s", ErrNotLocal, address)
			}
			return nil
		},
	}
	// An IP literal is checked before the dial as well as at the socket:
	// on Windows (and AIX, OpenBSD) Go rewrites a dial to an unspecified
	// address (0.0.0.0, ::) into one to loopback before Control runs, so
	// Control alone would let "0.0.0.0" through as "127.0.0.1". The rule is
	// the same on every OS: an unspecified address is not this computer.
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrNotLocal, address)
		}
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			return nil, fmt.Errorf("%w: %s", ErrNotLocal, address)
		}
		return dialer.DialContext(ctx, network, address)
	}
	t := &http.Transport{
		Proxy:                 nil,
		DialContext:           dial,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 0,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &localGuard{next: t},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("egress: more than %d redirects", maxRedirects)
			}
			if !IsLoopbackHost(req.URL.Hostname()) {
				return fmt.Errorf("%w: a redirect to %s", ErrNotLocal, req.URL.Host)
			}
			return nil
		},
	}
}

// localGuard refuses a URL whose host is not loopback before dialling, so
// the error says which host was refused rather than which address.
type localGuard struct{ next http.RoundTripper }

func (g *localGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || !IsLoopbackHost(req.URL.Hostname()) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, fmt.Errorf("%w: %s://%s", ErrNotLocal, req.URL.Scheme, req.URL.Host)
	}
	return g.next.RoundTrip(req)
}
