package ollama

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"advisor/internal/backend"
	"advisor/internal/egress"
)

// fakeRelease is ollama.com and GitHub in one TLS server: the download
// link, "latest", the pinned release (with its sha256sum.txt), and a CDN
// the release file redirects to — the chain the real hosts answer with
// (checked 2026-09-27: ollama.com/download/Ollama.dmg → 307 github.com/…/
// releases/latest/download/Ollama.dmg → 302 …/releases/download/v0.34.4/
// Ollama.dmg → 302 release-assets.githubusercontent.com).
type fakeRelease struct {
	srv       *httptest.Server
	file      string
	body      []byte
	listing   string // sha256sum.txt; "" answers 404
	pinned    bool   // false: "latest" goes straight to the CDN, naming no release
	fileGets  atomic.Int32
	sumGets   atomic.Int32
	finalHost string
}

func newFakeRelease(t *testing.T, file string, body []byte) *fakeRelease {
	t.Helper()
	sum := sha256.Sum256(body)
	f := &fakeRelease{file: file, body: body, pinned: true,
		listing: hex.EncodeToString(sum[:]) + "  ./install.sh\n" + hex.EncodeToString(sum[:]) + "  ./" + file + "\n"}
	mux := http.NewServeMux()
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ollama/ollama/releases/latest/download/"+strings.TrimPrefix(r.URL.Path, "/download/"), http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/ollama/ollama/releases/latest/download/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/ollama/ollama/releases/latest/download/")
		if !f.pinned {
			http.Redirect(w, r, "/cdn/"+name, http.StatusFound)
			return
		}
		http.Redirect(w, r, "/ollama/ollama/releases/download/v0.34.4/"+name, http.StatusFound)
	})
	mux.HandleFunc("/ollama/ollama/releases/download/v0.34.4/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/ollama/ollama/releases/download/v0.34.4/")
		if name == checksumFile {
			f.sumGets.Add(1)
			if f.listing == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(f.listing))
			return
		}
		http.Redirect(w, r, "/cdn/"+name+"?sig=signed", http.StatusFound)
	})
	mux.HandleFunc("/cdn/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			f.fileGets.Add(1)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(f.body)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(f.body)
	})
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	u, _ := url.Parse(f.srv.URL)
	f.finalHost = u.Host
	return f
}

func (f *fakeRelease) downloader() *downloader {
	return &downloader{client: f.srv.Client(), base: f.srv.URL + "/download/", releaseHost: f.finalHost}
}

// isolateTemp points os.TempDir at a fresh folder, so a test can see what a
// download left behind.
func isolateTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(k, dir)
	}
	return dir
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestDownloadChecksTheFileAgainstThePublishedChecksum(t *testing.T) {
	tmp := isolateTemp(t)
	body := []byte("the installer's bytes")
	f := newFakeRelease(t, "Ollama.dmg", body)
	var saw []backend.InstallProgress
	path, tag, err := f.downloader().download(context.Background(), "Ollama.dmg", func(p backend.InstallProgress) { saw = append(saw, p) })
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if tag != "v0.34.4" {
		t.Errorf("tag %q", tag)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(body) {
		t.Fatalf("the file: %q, %v", got, err)
	}
	if !strings.HasPrefix(path, tmp) || filepath.Base(path) != "Ollama.dmg" {
		t.Errorf("downloaded to %s, want a folder under %s", path, tmp)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Dir(path))
		if err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("the download folder's mode is %v, want 0700 (only this user): %v", st.Mode().Perm(), err)
		}
		fst, _ := os.Stat(path)
		if fst.Mode().Perm() != 0o600 {
			t.Errorf("the file's mode is %v, want 0600", fst.Mode().Perm())
		}
	}
	if last := saw[len(saw)-1]; !strings.Contains(last.Status, "checksum") {
		t.Errorf("the last progress line should say the file was checked: %+v", last)
	}
}

func TestDownloadRefusesAFileThatDoesNotMatch(t *testing.T) {
	tmp := isolateTemp(t)
	f := newFakeRelease(t, "OllamaSetup.exe", []byte("what the release published"))
	f.body = []byte("something else, served in its place")
	_, _, err := f.downloader().download(context.Background(), "OllamaSetup.exe", nil)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
	if left := leftovers(t, tmp); len(left) != 0 {
		t.Errorf("a refused download left files behind: %v", left)
	}
}

func TestDownloadRefusesWhenNoReleaseIsNamed(t *testing.T) {
	isolateTemp(t)
	f := newFakeRelease(t, "Ollama.dmg", []byte("x"))
	f.pinned = false
	_, _, err := f.downloader().download(context.Background(), "Ollama.dmg", nil)
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v, want ErrNoRelease", err)
	}
	if f.fileGets.Load() != 0 {
		t.Errorf("the file was downloaded %d times though it could not be checked", f.fileGets.Load())
	}
}

func TestDownloadRefusesWithoutAPublishedChecksum(t *testing.T) {
	for name, listing := range map[string]string{
		"no sha256sum.txt":   "",
		"file not listed":    strings.Repeat("a", 64) + "  ./something-else.zip\n",
		"not a sha256 value": "abc  ./Ollama.dmg\n",
	} {
		t.Run(name, func(t *testing.T) {
			isolateTemp(t)
			f := newFakeRelease(t, "Ollama.dmg", []byte("x"))
			f.listing = listing
			_, _, err := f.downloader().download(context.Background(), "Ollama.dmg", nil)
			if !errors.Is(err, ErrNoChecksum) {
				t.Fatalf("err = %v, want ErrNoChecksum", err)
			}
			if f.fileGets.Load() != 0 {
				t.Errorf("the file was downloaded though there was nothing to check it against")
			}
		})
	}
}

func TestChecksumForReadsOllamasListing(t *testing.T) {
	sum := strings.Repeat("0123456789abcdef", 4)
	for _, line := range []string{
		sum + "  ./Ollama.dmg",
		sum + "  Ollama.dmg",
		sum + " *Ollama.dmg",
		strings.ToUpper(sum) + "  ./Ollama.dmg",
	} {
		got, err := checksumFor(strings.NewReader("ffff  ./other\n"+line+"\n"), "Ollama.dmg")
		if err != nil || got != sum {
			t.Errorf("%q: %q, %v", line, got, err)
		}
	}
	if _, err := checksumFor(strings.NewReader(sum+"  ./Ollama.dmg.sig\n"), "Ollama.dmg"); err == nil {
		t.Error("a line for another file was taken")
	}
}

func TestInstallSizeAsksThePinnedRelease(t *testing.T) {
	f := newFakeRelease(t, "Ollama.dmg", []byte("0123456789"))
	d := f.downloader()
	r, err := d.resolve(context.Background(), "Ollama.dmg")
	if err != nil {
		t.Fatal(err)
	}
	n, known, err := d.size(context.Background(), r)
	if err != nil || !known || n != 10 {
		t.Fatalf("size = %d, %v, %v", n, known, err)
	}
	if f.fileGets.Load() != 0 {
		t.Error("asking the size downloaded the file")
	}
}

// The daemon's downloader starts at ollama.com and knows GitHub as the
// release host, and its client is the allow-listed one: those hosts are on
// the list for this purpose, and nothing else is reachable with it.
func TestTheRealDownloaderIsAllowListed(t *testing.T) {
	d := newDownloader()
	for _, raw := range []string{d.base + "Ollama.dmg", "https://" + d.releaseHost + releasePath + "v0.34.4/" + checksumFile} {
		u, _ := url.Parse(raw)
		if err := egress.Allowed(egress.OllamaDownload, u); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	f := newFakeRelease(t, "Ollama.dmg", []byte("x"))
	d.base = f.srv.URL + "/download/"
	if _, err := d.resolve(context.Background(), "Ollama.dmg"); !errors.Is(err, egress.ErrNotAllowed) {
		t.Errorf("the real client reached a host off the allow-list: %v", err)
	}
}

func TestServeEnvKeepsAnOllamaTheAdvisorStartsOnThisComputer(t *testing.T) {
	env := serveEnv([]string{"PATH=/bin", "OLLAMA_HOST=0.0.0.0:11434", "ollama_host=10.0.0.5", "OLLAMA_MODELS=/m"}, "http://127.0.0.1:11500")
	want := []string{"PATH=/bin", "OLLAMA_MODELS=/m", "OLLAMA_HOST=127.0.0.1:11500"}
	if strings.Join(env, "|") != strings.Join(want, "|") {
		t.Fatalf("serveEnv = %v, want %v", env, want)
	}
}

// D-68: what the adapter wrote for the advisor goes; the Ollama program a
// Linux install put in the data folder stays, and the answer says why.
func TestForgetDataRemovesTheAdaptersOwnFiles(t *testing.T) {
	tmp := isolateTemp(t)
	data := filepath.Join(t.TempDir(), "Advisor")
	t.Setenv("ADVISOR_DATA_DIR", data)
	for _, p := range []string{
		filepath.Join(data, "ollama", "logs", "server-supervised.log"),
		filepath.Join(data, "ollama", "bin", "ollama"),
		filepath.Join(tmp, "advisor-ollama-123", "Ollama.dmg"),
		filepath.Join(tmp, "someone-elses", "file"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := New()
	b.setSupervisedLog(filepath.Join(data, "ollama", "logs", "server-supervised.log"))
	removed, kept, err := b.ForgetData()
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]bool{
		filepath.Join(data, "ollama", "logs"):          false,
		filepath.Join(tmp, "advisor-ollama-123"):       false,
		filepath.Join(data, "ollama", "bin", "ollama"): true,
		filepath.Join(tmp, "someone-elses", "file"):    true,
	} {
		_, err := os.Stat(p)
		if exists := err == nil; exists != want {
			t.Errorf("%s exists = %v, want %v", p, exists, want)
		}
	}
	if len(removed) != 2 || len(kept) != 1 || !strings.Contains(kept[0], "program") {
		t.Errorf("removed %q, kept %q", removed, kept)
	}
	if b.getSupervisedLog() != "" {
		t.Error("the supervised log is still remembered")
	}
}
