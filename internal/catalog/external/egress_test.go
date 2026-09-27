package external

import (
	"strings"
	"testing"

	"advisor/internal/egress"
)

// PermittedHosts is this package's narrower list — which host each source
// may use — and every host on it must also be on the one allow-list
// (internal/egress/hosts.go, D-64) for public scores; the transport refuses
// anything that is not, whatever this map says.
func TestPermittedHostsAreOnTheAllowList(t *testing.T) {
	for id, hosts := range PermittedHosts {
		for _, h := range hosts {
			probe := h
			if rest, ok := strings.CutPrefix(h, "*."); ok {
				probe = "cdn." + rest
			}
			if !egress.AllowedHost(egress.PublicScores, probe) {
				t.Errorf("source %s: host %s is not on internal/egress/hosts.go's list for public scores", id, h)
			}
		}
	}
}

// A result's own "read it at the source" link is kept only when it is an
// https address; anything else is replaced by the repo's page.
func TestOnlyHTTPSLinksAreKept(t *testing.T) {
	for u, want := range map[string]bool{
		"https://huggingface.co/datasets/x": true,
		"https://arxiv.org/abs/2401.00001":  true,
		"http://example.com/results":        false,
		"javascript:alert(1)":               false,
		"https://user:pw@example.com/":      false,
		"//example.com/x":                   false,
		"":                                  false,
		"not a url":                         false,
	} {
		if got := httpsURL(u); got != want {
			t.Errorf("httpsURL(%q) = %v, want %v", u, got, want)
		}
	}
}
