package archtest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// directGoDependencies is ARCHITECTURE.md D-22/D-26/D-61's list, as go.mod
// states it: the standard library plus these. A new one fails here until
// it is argued for in ARCHITECTURE.md and added (CLAUDE.md: "a new one is
// a sentence in the PR saying what it replaces").
var directGoDependencies = []string{
	"github.com/goccy/go-yaml", // the data files (D-26)
	"github.com/gogpu/systray", // the tray icon, pure Go (D-61)
	"golang.org/x/sys",         // CPUID and the Windows registry (D-26)
	"modernc.org/sqlite",       // the database, pure Go (D-2)
}

// uiDependencies is what the UI ships in its bundle (D-22): React and its
// router. Build and test tools are devDependencies.
var uiDependencies = []string{"react", "react-dom", "react-router"}

// TestGoDependenciesArePinnedAndNamed is D-69 for the Go side: go.mod's
// direct requirements are exactly the named ones, each at one version
// (go.sum holds their hashes; CI runs go mod verify), and nothing is
// replaced by a local path or a fork.
func TestGoDependenciesArePinnedAndNamed(t *testing.T) {
	f, err := os.Open(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var direct []string
	inRequire := false
	sc := bufio.NewScanner(f)
	version := regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "replace"):
			t.Errorf("go.mod replaces a module (%s): dependencies come from their published versions", line)
		case line == "require (":
			inRequire = true
			continue
		case inRequire && line == ")":
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
			line, inRequire = strings.TrimPrefix(line, "require "), false
		} else if !inRequire {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if !version.MatchString(fields[1]) {
			t.Errorf("go.mod: %s is at %q, not one released version", fields[0], fields[1])
		}
		if !strings.Contains(line, "// indirect") {
			direct = append(direct, fields[0])
		}
	}
	sort.Strings(direct)
	want := append([]string(nil), directGoDependencies...)
	sort.Strings(want)
	if strings.Join(direct, " ") != strings.Join(want, " ") {
		t.Errorf("go.mod's direct dependencies are %v, ARCHITECTURE.md names %v: argue for a new one there first, then add it here", direct, want)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.sum")); err != nil {
		t.Errorf("go.sum is missing: %v", err)
	}
}

// TestUIDependenciesArePinned is D-69 for the UI: package.json names exact
// versions — the ones package-lock.json holds — never a range, a tag, a
// git URL or a path; the lockfile pins every transitive package by hash;
// and npm is told to keep it that way and to run no package's install
// scripts.
func TestUIDependenciesArePinned(t *testing.T) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	b, err := os.ReadFile(filepath.Join(repoRoot, "ui", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		t.Fatal(err)
	}
	exact := regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	var shipped []string
	for name, v := range pkg.Dependencies {
		shipped = append(shipped, name)
		if !exact.MatchString(v) {
			t.Errorf("ui/package.json: %s is %q, not an exact version", name, v)
		}
	}
	for name, v := range pkg.DevDependencies {
		if !exact.MatchString(v) {
			t.Errorf("ui/package.json: %s (dev) is %q, not an exact version", name, v)
		}
	}
	sort.Strings(shipped)
	if strings.Join(shipped, " ") != strings.Join(uiDependencies, " ") {
		t.Errorf("the UI ships %v; ARCHITECTURE.md names %v", shipped, uiDependencies)
	}

	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version   string `json:"version"`
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
			Link      bool   `json:"link"`
		} `json:"packages"`
	}
	lb, err := os.ReadFile(filepath.Join(repoRoot, "ui", "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lb, &lock); err != nil {
		t.Fatal(err)
	}
	for path, p := range lock.Packages {
		if path == "" || p.Link {
			continue
		}
		if !strings.HasPrefix(p.Resolved, "https://registry.npmjs.org/") {
			t.Errorf("package-lock.json: %s resolves to %q, not the npm registry over https", path, p.Resolved)
		}
		if !strings.HasPrefix(p.Integrity, "sha512-") {
			t.Errorf("package-lock.json: %s has no sha512 integrity hash", path)
		}
	}
	for name, v := range pkg.Dependencies {
		if got := lock.Packages["node_modules/"+name].Version; got != v {
			t.Errorf("%s: package.json says %s, the lockfile %s", name, v, got)
		}
	}
	for name, v := range pkg.DevDependencies {
		if got := lock.Packages["node_modules/"+name].Version; got != v {
			t.Errorf("%s: package.json says %s, the lockfile %s", name, v, got)
		}
	}

	rc, err := os.ReadFile(filepath.Join(repoRoot, "ui", ".npmrc"))
	if err != nil {
		t.Fatalf("ui/.npmrc: %v", err)
	}
	for _, want := range []string{"save-exact=true", "ignore-scripts=true"} {
		if !strings.Contains(string(rc), want) {
			t.Errorf("ui/.npmrc does not say %s", want)
		}
	}
}

// TestCIActionsArePinnedToCommits: a workflow step that runs someone
// else's code names the exact commit, so a moved tag cannot change what
// builds the release.
func TestCIActionsArePinnedToCommits(t *testing.T) {
	matches, _ := filepath.Glob(filepath.Join(repoRoot, ".github", "workflows", "*.yml"))
	if len(matches) == 0 {
		t.Fatal("no workflows found")
	}
	uses := regexp.MustCompile(`uses:\s*([^\s#]+)`)
	sha := regexp.MustCompile(`@[0-9a-f]{40}$`)
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range uses.FindAllStringSubmatch(string(b), -1) {
			if strings.HasPrefix(u[1], "./") {
				continue
			}
			if !sha.MatchString(u[1]) {
				t.Errorf("%s: %s is not pinned to a commit", filepath.Base(m), u[1])
			}
		}
		if strings.Contains(string(b), "/continuous/") {
			t.Errorf("%s downloads a moving \"continuous\" build", filepath.Base(m))
		}
	}
}
