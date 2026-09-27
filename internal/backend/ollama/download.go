package ollama

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"advisor/internal/backend"
	"advisor/internal/egress"
)

// How Install gets Ollama (ARCHITECTURE.md D-66, superseding D-29's "no
// checksum is published"):
//
//  1. ask ollama.com's download link for the file where it leads, and stop
//     at the first hop that names a release: github.com/ollama/ollama/
//     releases/download/<tag>/<file>. That pins the version — every later
//     request is for that release, never "latest", so the checksum and the
//     file cannot come from two different releases;
//  2. read that release's sha256sum.txt, which Ollama publishes with every
//     release, and find the file's line;
//  3. download the file from the pinned release into a folder only this
//     user can read, hashing it on the way;
//  4. refuse it — delete it, say why — unless the hash is the published one.
//
// Every request goes through egress.Client(egress.OllamaDownload): HTTPS,
// certificates verified, and only ollama.com/download/, github.com/ollama/
// ollama/releases/ and GitHub's two download hosts, the redirects included.
// If Ollama stops publishing a checksum, or its link stops leading to a
// release, the install fails with a sentence saying so rather than running
// a file nobody checked.

// Where Ollama's downloads start and the release they lead to.
const (
	downloadBase    = "https://ollama.com/download/"
	releaseHost     = "github.com"
	releasePath     = "/ollama/ollama/releases/download/"
	checksumFile    = "sha256sum.txt"
	maxChecksumSize = 64 << 10
)

// releaseTag is what a release's tag looks like: v0.34.4, v0.35.0-rc1.
var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

// Errors a caller (and the install's status line) can tell apart.
var (
	// ErrNoRelease: ollama.com's link did not lead to a GitHub release, so
	// there is no published checksum to check the file against.
	ErrNoRelease = errors.New("ollama: the download link did not lead to a published Ollama release, so its checksum could not be checked; nothing was installed")
	// ErrNoChecksum: the release does not list a checksum for the file.
	ErrNoChecksum = errors.New("ollama: Ollama did not publish a checksum for this file, so it could not be checked; nothing was installed")
	// ErrChecksumMismatch: the file is not the one Ollama published.
	ErrChecksumMismatch = errors.New("ollama: the download is not the file Ollama published (its checksum does not match); it was deleted and nothing was installed")
)

// release is one file of one Ollama release, pinned.
type release struct {
	Tag     string   // "v0.34.4"
	File    string   // "Ollama.dmg"
	FileURL *url.URL // https://github.com/ollama/ollama/releases/download/v0.34.4/Ollama.dmg
	SHA256  string   // from the release's sha256sum.txt; "" until readChecksum
}

// downloader is Install's network side. Its fields are the seam tests point
// at a fake: base (the first link), releaseHost (where a release lives) and
// the client. In the daemon they are the constants above and the egress
// client.
type downloader struct {
	client      *http.Client
	base        string
	releaseHost string
}

func newDownloader() *downloader {
	return &downloader{client: egress.Client(egress.OllamaDownload, 0), base: downloadBase, releaseHost: releaseHost}
}

// resolve follows ollama.com's link for file only as far as the first hop
// that names a pinned release, and stops there.
func (d *downloader) resolve(ctx context.Context, file string) (release, error) {
	var pinned *url.URL
	c := *d.client
	next := d.client.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if next != nil {
			if err := next(req, via); err != nil {
				return err
			}
		}
		if u := req.URL; u.Scheme == "https" && strings.EqualFold(u.Host, d.releaseHost) && strings.HasPrefix(u.Path, releasePath) {
			rest := strings.TrimPrefix(u.Path, releasePath)
			if tag, name, ok := strings.Cut(rest, "/"); ok && name == file && releaseTag.MatchString(tag) {
				pinned = u
				return http.ErrUseLastResponse
			}
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, d.base+url.PathEscape(file), nil)
	if err != nil {
		return release{}, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("ollama: install: asking %s where the download is: %w", d.base+file, err)
	}
	resp.Body.Close()
	if pinned == nil {
		return release{}, ErrNoRelease
	}
	tag, _, _ := strings.Cut(strings.TrimPrefix(pinned.Path, releasePath), "/")
	return release{Tag: tag, File: file, FileURL: pinned}, nil
}

// readChecksum reads the release's sha256sum.txt and sets r.SHA256 from the
// line for r.File. Ollama writes "<hex>  ./<file>"; "<hex>  <file>" and the
// binary-mode "<hex> *<file>" are read too.
func (d *downloader) readChecksum(ctx context.Context, r *release) error {
	u := *r.FileURL
	u.Path = releasePath + r.Tag + "/" + checksumFile
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: install: reading the checksums of Ollama %s: %w", r.Tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w (Ollama %s has no %s)", ErrNoChecksum, r.Tag, checksumFile)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama: install: reading the checksums of Ollama %s: %s", r.Tag, resp.Status)
	}
	sum, err := checksumFor(io.LimitReader(resp.Body, maxChecksumSize), r.File)
	if err != nil {
		return fmt.Errorf("%w (Ollama %s: %v)", ErrNoChecksum, r.Tag, err)
	}
	r.SHA256 = sum
	return nil
}

// checksumFor finds file's SHA-256 in a sha256sum listing.
func checksumFor(listing io.Reader, file string) (string, error) {
	sc := bufio.NewScanner(listing)
	for sc.Scan() {
		sum, name, ok := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		if !ok {
			continue
		}
		name = strings.TrimLeft(strings.TrimSpace(name), "*")
		name = strings.TrimPrefix(name, "./")
		if name != file {
			continue
		}
		sum = strings.ToLower(sum)
		if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
			return "", fmt.Errorf("the checksum listed for %s is not a SHA-256", file)
		}
		return sum, nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s is not listed", file)
}

// fetch downloads the pinned file into dir, reporting progress in bytes,
// and returns its path only if its SHA-256 is the published one. On any
// failure the partial or refused file is removed.
func (d *downloader) fetch(ctx context.Context, r release, dir string, progress func(backend.InstallProgress)) (string, error) {
	if r.SHA256 == "" {
		return "", ErrNoChecksum
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.FileURL.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: install: downloading Ollama %s: %w", r.Tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama: install: downloading Ollama %s: %s", r.Tag, resp.Status)
	}

	dest := filepath.Join(dir, r.File)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("ollama: install: creating %s: %w", dest, err)
	}
	fail := func(err error) (string, error) {
		f.Close()
		os.Remove(dest)
		return "", err
	}
	h := sha256.New()
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return fail(fmt.Errorf("ollama: install: writing %s: %w", dest, werr))
			}
			h.Write(buf[:n])
			written += int64(n)
			if progress != nil {
				progress(backend.InstallProgress{Status: "downloading Ollama " + r.Tag, Completed: written, Total: total})
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fail(fmt.Errorf("ollama: install: downloading Ollama %s: %w", r.Tag, rerr))
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(dest)
		return "", fmt.Errorf("ollama: install: writing %s: %w", dest, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != r.SHA256 {
		os.Remove(dest)
		return "", fmt.Errorf("%w (Ollama %s, %s: expected %s, got %s)", ErrChecksumMismatch, r.Tag, r.File, r.SHA256, got)
	}
	if progress != nil {
		progress(backend.InstallProgress{Status: "checked Ollama " + r.Tag + " against the checksum Ollama published", Completed: written, Total: written})
	}
	return dest, nil
}

// size asks for the pinned file's size with a HEAD request that follows
// GitHub's redirect to its download host; known is false when no length
// is stated.
func (d *downloader) size(ctx context.Context, r release) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.FileURL.String(), nil)
	if err != nil {
		return 0, false, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("ollama: install size: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false, fmt.Errorf("ollama: install size: %s", resp.Status)
	}
	if resp.ContentLength <= 0 {
		return 0, false, nil
	}
	return resp.ContentLength, true, nil
}

// download is the whole of steps 1–4 for one file: it returns the checked
// file's path inside a new folder only this user can read (the caller
// removes the folder when it is done with the file), and the release tag.
func (d *downloader) download(ctx context.Context, file string, progress func(backend.InstallProgress)) (path, tag string, err error) {
	if progress != nil {
		progress(backend.InstallProgress{Status: "finding the current Ollama release"})
	}
	r, err := d.resolve(ctx, file)
	if err != nil {
		return "", "", err
	}
	if err := d.readChecksum(ctx, &r); err != nil {
		return "", r.Tag, err
	}
	dir, err := os.MkdirTemp("", tempDirPattern)
	if err != nil {
		return "", r.Tag, fmt.Errorf("ollama: install: %w", err)
	}
	p, err := d.fetch(ctx, r, dir, progress)
	if err != nil {
		os.RemoveAll(dir)
		return "", r.Tag, err
	}
	return p, r.Tag, nil
}

// tempDirPattern names the folders downloads land in, so "delete
// everything" can find any an interrupted install left behind (D-68).
const tempDirPattern = "advisor-ollama-*"
