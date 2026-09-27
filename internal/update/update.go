// Package update answers one question for the Settings screen's manual
// "Check for updates" button (build-plan step 11): is a newer release out,
// and where do I get it? It never runs on a timer and never carries
// anything about the user or the machine — a single unauthenticated GET to
// GitHub's own release feed, the same request anyone's browser would make
// hitting the download page.
//
// The request goes through internal/egress's client for its own purpose
// (egress.UpdateCheck), which can reach api.github.com's releases of this
// project and nothing else — the host is a line in the one allow-list
// (internal/egress/hosts.go, D-64), kept apart from the model-data hosts
// by purpose, as D-62 wanted it apart.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"advisor/internal/egress"
	"advisor/internal/version"
)

// PermittedHost is the only host Check ever connects to — enforced by the
// egress client, whose allow-list names it for egress.UpdateCheck only.
const PermittedHost = "api.github.com"

// ReleasesURL is this project's GitHub "latest release" API endpoint.
// Hard-coded, not configurable: the whole point of a fixed allow-list is
// that there is nowhere else this can be pointed.
const ReleasesURL = "https://" + PermittedHost + "/repos/itayp/-Local-LLM-Advisor-n-Optimizer/releases/latest"

// DownloadURL is where "Check for updates" links the user regardless of
// what Check finds — the release page itself. Which file to click for
// their OS is INSTALL.md's job, not this package's.
const DownloadURL = "https://github.com/itayp/-Local-LLM-Advisor-n-Optimizer/releases/latest"

// ReleasePagePrefix is what a release page GitHub names must start with to
// be shown as the download link.
const ReleasePagePrefix = "https://github.com/itayp/-Local-LLM-Advisor-n-Optimizer/releases/"

// DefaultTimeout bounds one check; a person is watching a button.
const DefaultTimeout = 8 * time.Second

// maxBody bounds how much of a (misbehaving or unexpected) answer this
// package will read; GitHub's release JSON for this project is a few KB.
const maxBody = 1 << 20

// Info is GET /api/update/check's payload. There is no figure.* type here
// on purpose: product rule 4 is about numbers with provenance, and a
// version string, a URL and a bool have none to carry.
type Info struct {
	// Current is the running build's version.Version, echoed back so the
	// UI never has to ask twice.
	Current string `json:"current"`
	// Latest is the newest tag the release feed reported, empty if the
	// check did not succeed.
	Latest string `json:"latest,omitempty"`
	// URL is always DownloadURL, present even when the check failed, so
	// the button can still offer "see the releases page yourself."
	URL string `json:"url"`
	// UpdateAvailable is true only when Current and Latest both parsed as
	// vMAJOR.MINOR.PATCH and Latest is strictly newer. A "dev" build (no
	// tag, or a git-describe suffix past one) never claims this — there is
	// nothing honest to compare it against.
	UpdateAvailable bool `json:"update_available"`
	// Checked is false when the request itself failed (network, rate
	// limit, an unexpected answer); Error then says why, in words.
	Checked bool   `json:"checked"`
	Error   string `json:"error,omitempty"`
}

// release is the sliver of GitHub's release JSON this package reads.
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check makes one GET to ReleasesURL and compares its tag against current
// (version.Version). client may be nil (the egress client for
// egress.UpdateCheck is used); tests pass one pointed at an
// httptest.Server. Check never retries — a person just clicked a button and
// wants one honest answer, not a background poller.
func Check(ctx context.Context, client *http.Client, current string) Info {
	return checkURL(ctx, client, ReleasesURL, current)
}

// checkURL is Check with the URL as a parameter, so tests can point it at
// an httptest.Server without touching the real network.
func checkURL(ctx context.Context, client *http.Client, url string, current string) Info {
	info := Info{Current: current, URL: DownloadURL}
	if client == nil {
		client = egress.Client(egress.UpdateCheck, DefaultTimeout)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		info.Error = "could not build the request: " + err.Error()
		return info
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		info.Error = "could not reach the update feed: " + err.Error()
		return info
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		info.Error = "no release has been published yet"
		return info
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		info.Error = "the update feed asked us to slow down; try again in a while"
		return info
	case resp.StatusCode != http.StatusOK:
		info.Error = fmt.Sprintf("the update feed answered HTTP %d", resp.StatusCode)
		return info
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		info.Error = "could not read the update feed's answer: " + err.Error()
		return info
	}
	if len(body) > maxBody {
		info.Error = "the update feed's answer was larger than expected"
		return info
	}

	var rel release
	if err := json.Unmarshal(body, &rel); err != nil {
		info.Error = "the update feed's answer could not be read"
		return info
	}
	if rel.TagName == "" {
		info.Error = "the update feed's answer had no version in it"
		return info
	}

	info.Checked = true
	info.Latest = rel.TagName
	// The link the Settings screen shows is the release page GitHub named,
	// but only when it is this project's own release page: the answer is
	// someone else's server's words, and a link the person clicks must not
	// be able to point anywhere else.
	if strings.HasPrefix(rel.HTMLURL, ReleasePagePrefix) {
		info.URL = rel.HTMLURL
	}
	if newer, ok := isNewer(current, rel.TagName); ok {
		info.UpdateAvailable = newer
	}
	return info
}

var errNotComparable = errors.New("update: not a comparable version")

// semverPrefixRE reads the vMAJOR.MINOR.PATCH core off the front of a
// version string, ignoring anything after it — `git describe`'s
// "-N-gHASH" and "-dirty" suffixes on a dev build, in particular.
var semverPrefixRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

type semver struct{ major, minor, patch int }

func parseSemver(s string) (semver, error) {
	m := semverPrefixRE.FindStringSubmatch(s)
	if m == nil {
		return semver{}, errNotComparable
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return semver{major, minor, patch}, nil
}

func (a semver) less(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}

// isNewer reports whether latest is a strictly newer release than current.
// ok is false when either string is not a plain vMAJOR.MINOR.PATCH-prefixed
// version — most commonly current == "dev" (an unstamped developer build),
// which has nothing honest to compare against.
func isNewer(current, latest string) (newer bool, ok bool) {
	cv, err := parseSemver(current)
	if err != nil {
		return false, false
	}
	lv, err := parseSemver(latest)
	if err != nil {
		return false, false
	}
	return cv.less(lv), true
}
