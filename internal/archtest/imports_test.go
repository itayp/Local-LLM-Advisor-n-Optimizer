package archtest

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// layer is ARCHITECTURE.md D-11's dependency direction as numbers: a
// package imports only packages on a lower layer. cmd → server → domain
// packages → figure/version/data, and within the domain the order of the
// workflow (hardware → backend → catalogue → estimate → recommend → bench
// → watch). A new package fails TestDependenciesPointOneWay until it is
// placed here — which is the moment to decide where it sits.
var layer = map[string]int{
	// Leaves: data, types, and the one network package.
	"data":                     0,
	"data/icon":                0,
	"internal/figure":          0,
	"internal/version":         0,
	"internal/egress":          0,
	"internal/catalog/gguf":    0,
	"internal/catalog/parquet": 0,
	"internal/autostart":       0,
	"internal/tray":            0,
	"internal/winapp":          0,
	"internal/chatapps":        0,

	"internal/hardware":   1,
	"internal/suite":      1,
	"internal/update":     1,
	"internal/catalog/hf": 1,

	"internal/catalog": 2,
	"internal/backend": 2,

	"internal/backend/ollama": 3,
	"internal/estimate":       3,
	"internal/store":          3,

	"internal/recommend":        4,
	"internal/catalog/external": 4,

	"internal/catalog/refresh": 5,
	"internal/bench":           5,

	"internal/watch": 6,

	"internal/server": 7,

	"cmd/advisor": 8,
}

// forbidden are edges the layers would allow but a decision rules out.
var forbidden = map[[2]string]string{
	{"internal/catalog", "internal/store"}:              "D-35: catalog itself imports neither store nor hf",
	{"internal/catalog", "internal/catalog/hf"}:         "D-35: catalog itself imports neither store nor hf",
	{"internal/recommend", "internal/catalog/external"}: "D-57: the engine scores public data through a closure the server passes, never by importing external",
	{"internal/watch", "internal/catalog/external"}:     "D-57: watch.Options.PublicLine is a closure, not an import",
	{"internal/backend", "internal/store"}:              "D-11: backend never imports store",
}

func TestDependenciesPointOneWay(t *testing.T) {
	imports := map[string]map[string]bool{}
	for _, sf := range daemonSources(t) {
		if imports[sf.pkg] == nil {
			imports[sf.pkg] = map[string]bool{}
		}
		for p := range sf.imports {
			if strings.HasPrefix(p, modulePath) {
				imports[sf.pkg][strings.TrimPrefix(p, modulePath)] = true
			}
		}
	}
	var pkgs []string
	for p := range imports {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	for _, from := range pkgs {
		lf, ok := layer[from]
		if !ok {
			t.Errorf("%s is not placed in archtest's layer table (ARCHITECTURE.md D-11): decide which packages it may import, and add it", from)
			continue
		}
		for to := range imports[from] {
			if why, bad := forbidden[[2]string{from, to}]; bad {
				t.Errorf("%s imports %s — %s", from, to, why)
			}
			lt, ok := layer[to]
			if !ok {
				if _, isDaemon := imports[to]; !isDaemon {
					// A package outside cmd/ and internal/ (data is placed above).
					t.Errorf("%s imports %s, which is not placed in the layer table", from, to)
				}
				continue
			}
			if lt >= lf {
				t.Errorf("%s (layer %d) imports %s (layer %d): dependencies point one way, down (ARCHITECTURE.md D-11)",
					from, lf, to, lt)
			}
		}
	}
	// And the two statements D-11 makes in words.
	for _, from := range pkgs {
		if imports[from]["internal/server"] && from != "cmd/advisor" {
			t.Errorf("%s imports internal/server; nothing but cmd/advisor may", from)
		}
		for to := range imports[from] {
			if strings.HasPrefix(to, "cmd/") {
				t.Errorf("%s imports %s; nothing imports cmd", from, to)
			}
		}
	}
	if len(imports) < 20 {
		t.Fatalf("only %d packages seen: %s", len(imports), strconv.Quote(strings.Join(pkgs, " ")))
	}
}
