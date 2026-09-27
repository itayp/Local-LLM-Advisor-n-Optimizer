package archtest

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The one package that may build clients and dial (internal/egress), and
// the one file that may listen (internal/server/server.go, whose Listen
// takes only a port and binds only server.LoopbackHost).
const (
	egressPkg    = "internal/egress"
	listenerFile = "internal/server/server.go"
)

// forbiddenImports are standard-library packages that talk to other
// computers without going through an HTTP client at all.
var forbiddenImports = map[string]string{
	"net/smtp":               "sends mail",
	"net/rpc":                "makes RPC connections",
	"net/rpc/jsonrpc":        "makes RPC connections",
	"net/http/httputil":      "can forward requests (a reverse proxy)",
	"net/http/cookiejar":     "keeps cookies for a remote site; the advisor sends none",
	"log/syslog":             "can send logs to a remote syslog server",
	"golang.org/x/net/proxy": "dials through arbitrary proxies",
}

// downloadTools are programs (and PowerShell/.NET calls) that fetch from
// the network. No string the daemon passes to a command may name one: the
// daemon's own requests go through internal/egress, where the allow-list
// can see them.
var downloadTools = regexp.MustCompile(`(?i)\b(curl|wget|bitsadmin|certutil|invoke-webrequest|invoke-restmethod|iwr|irm|start-bitstransfer|webclient|downloadstring|downloadfile|httpclient|xmlhttp)\b`)

// credentials are the headers and environment variables that would carry
// something of the person's to a server: a login token, a cookie, an API
// key. The advisor sends none (D-34: "no token is ever sent"; the fetcher's
// "no token and no cookies ever sent"), so no string in the daemon names
// one.
var credentials = regexp.MustCompile(`(?i)^(authorization|proxy-authorization|cookie|x-api-key|api-key|hf_token|hugging_face_hub_token|huggingface_token|github_token|gh_token)$`)

// TestNoCredentialIsEverSent: no request header or environment variable
// that carries a credential is named anywhere in the daemon's code.
func TestNoCredentialIsEverSent(t *testing.T) {
	for _, sf := range daemonSources(t) {
		ast.Inspect(sf.file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && credentials.MatchString(strings.TrimSpace(s)) {
				t.Errorf("%s: names %q; the advisor sends no credential, cookie or key to anyone", sf.pos(lit), s)
			}
			return true
		})
		ast.Inspect(sf.file, func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Jar" {
					t.Errorf("%s: gives a client a cookie jar", sf.pos(kv))
				}
			}
			return true
		})
	}
}

// TestOnlyEgressReachesTheNetwork is D-64's rule: every connection to
// another computer is made by a client internal/egress built, so the
// allow-list in internal/egress/hosts.go is the whole audit.
func TestOnlyEgressReachesTheNetwork(t *testing.T) {
	var problems []string
	bad := func(sf sourceFile, n ast.Node, what string) {
		problems = append(problems, sf.pos(n)+": "+what)
	}
	for _, sf := range daemonSources(t) {
		inEgress := sf.pkg == egressPkg
		for p, why := range forbiddenImports {
			if _, ok := sf.imports[p]; ok {
				bad(sf, sf.file, "imports "+p+", which "+why)
			}
		}
		ast.Inspect(sf.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if sel, ok := sf.selectorOf(x, "net/http"); ok {
					switch sel {
					case "DefaultClient", "DefaultTransport":
						if !inEgress {
							bad(sf, x, "uses http."+sel+", which reaches any host; take a client from internal/egress")
						}
					case "Get", "Post", "Head", "PostForm":
						bad(sf, x, "calls http."+sel+" (the default client); take a client from internal/egress")
					case "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS":
						bad(sf, x, "serves with http."+sel+"; the daemon serves only through server.Serve on server.Listen's loopback listener")
					}
				}
				if sel, ok := sf.selectorOf(x, "net"); ok {
					switch {
					case strings.HasPrefix(sel, "Dial"):
						if !inEgress {
							bad(sf, x, "dials with net."+sel+"; take a client from internal/egress")
						}
					case strings.HasPrefix(sel, "Listen"):
						if sf.rel != listenerFile {
							bad(sf, x, "listens with net."+sel+"; only server.Listen (loopback, product rule 7) may")
						}
					}
				}
				if sel, ok := sf.selectorOf(x, "crypto/tls"); ok && (sel == "Dial" || sel == "DialWithDialer" || sel == "Listen") {
					bad(sf, x, "uses tls."+sel+"; take a client from internal/egress")
				}
			case *ast.CompositeLit:
				if !inEgress {
					if sel, ok := sf.selectorOf(x.Type, "net/http"); ok && (sel == "Client" || sel == "Transport") {
						bad(sf, x, "builds an http."+sel+"; take a client from internal/egress (egress.Client for another computer, egress.Local for this one)")
					}
					if sel, ok := sf.selectorOf(x.Type, "net"); ok && (sel == "Dialer" || sel == "Resolver") {
						bad(sf, x, "builds a net."+sel+"; take a client from internal/egress")
					}
					if sel, ok := sf.selectorOf(x.Type, "crypto/tls"); ok && sel == "Config" {
						bad(sf, x, "builds a tls.Config; TLS settings live in internal/egress")
					}
				}
			case *ast.CallExpr:
				// new(http.Client), new(http.Transport)
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 && !inEgress {
					if sel, ok := sf.selectorOf(x.Args[0], "net/http"); ok && (sel == "Client" || sel == "Transport") {
						bad(sf, x, "builds an http."+sel+" with new(); take a client from internal/egress")
					}
				}
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "InsecureSkipVerify" {
					bad(sf, x, "sets InsecureSkipVerify; certificates are always verified")
				}
			case *ast.AssignStmt:
				if inEgress {
					return true
				}
				for _, lhs := range x.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Transport" || sel.Sel.Name == "InsecureSkipVerify") {
						bad(sf, x, "replaces a client's "+sel.Sel.Name+"; only tests may, and only with a fake")
					}
				}
			case *ast.BasicLit:
				if x.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(x.Value)
				if err != nil {
					s = x.Value
				}
				if m := downloadTools.FindString(s); m != "" {
					bad(sf, x, "a string names "+strconv.Quote(m)+", a program that downloads; the daemon's requests go through internal/egress")
				}
			}
			return true
		})
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// TestTheScanSeesWhatItLooksFor keeps the scan honest: it must find the
// listener it permits and the egress package it exempts, or it is looking
// in the wrong place.
func TestTheScanSeesWhatItLooksFor(t *testing.T) {
	var sawListener, sawEgress bool
	for _, sf := range daemonSources(t) {
		if sf.rel == listenerFile {
			ast.Inspect(sf.file, func(n ast.Node) bool {
				if sel, ok := sf.selectorOf(n, "net"); ok && sel == "Listen" {
					sawListener = true
				}
				return true
			})
		}
		if sf.pkg == egressPkg {
			if _, ok := sf.imports["net/http"]; ok {
				sawEgress = true
			}
		}
	}
	if !sawListener {
		t.Errorf("the scan did not find net.Listen in %s", listenerFile)
	}
	if !sawEgress {
		t.Errorf("the scan did not find %s", egressPkg)
	}
}
