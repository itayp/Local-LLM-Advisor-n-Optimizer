package ollama

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"advisor/internal/backend"
)

// Install downloads and sets up Ollama itself. Product rule 5: this only
// ever runs from a UI button that already told the user what will happen;
// Install changes nothing on the machine until it is called. The download
// is download.go's: pinned to one release, over HTTPS to Ollama's own
// hosts only, and checked against the SHA-256 Ollama publishes for it
// before anything is opened or unpacked (D-66). The per-OS approach is
// osInstall (install_darwin.go, install_windows.go, install_linux.go): on
// macOS and Windows it opens the checked installer — the user still clicks
// through it, same as downloading it by hand — and Detect() picks up the
// result afterwards; on Linux it is a full user-space install (no sudo, no
// shell script piped from the network, no system service), because there
// is no installer to click through in the first place.
func (b *Backend) Install(ctx context.Context, progress func(backend.InstallProgress)) error {
	return b.osInstall(ctx, progress)
}

// Start launches Ollama (`ollama serve`) so a subsequent Detect can report
// StateRunning. Its output is captured to this daemon's own data folder so
// runtimePaths (runtimepath.go) can read this exact run's device-discovery
// lines instead of guessing at Ollama's default log location.
func (b *Backend) Start(ctx context.Context) error {
	return b.osStart(ctx)
}

// InstallSize reports the size of the installer/archive Install would
// download, before Install is ever called — product rule 5: the button
// says what it will cost before it is clicked. It pins the release the
// way Install does (download.go) and asks for the file's size with a HEAD
// request; no body is fetched. known is false rather than a guess when the
// server states no length (D-21).
func (b *Backend) InstallSize(ctx context.Context) (int64, bool, error) {
	file, err := b.installFile()
	if err != nil {
		return 0, false, err
	}
	d := b.downloader()
	r, err := d.resolve(ctx, file)
	if err != nil {
		return 0, false, err
	}
	return d.size(ctx, r)
}

// downloader is Install's network side (download.go); tests replace it.
func (b *Backend) downloader() *downloader {
	if b.newDownloader != nil {
		return b.newDownloader()
	}
	return newDownloader()
}

// dataDir mirrors store.DefaultDataDir's per-OS convention
// (ADVISOR_DATA_DIR override, then each OS's application-data folder). It
// is deliberately duplicated here rather than imported: ARCHITECTURE.md
// D-11's dependency direction is one way — server imports store and
// backend, backend never imports store — and this OS-folder convention is
// the one place backend needs to agree with store on where the daemon's
// data lives (D-16: the Linux user-space Ollama install goes under the same
// data folder store.DefaultDataDir() returns).
func dataDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("ADVISOR_DATA_DIR")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ollama: cannot find the home directory: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Advisor"), nil
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "Advisor"), nil
		}
		return filepath.Join(home, "AppData", "Local", "Advisor"), nil
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, "advisor"), nil
		}
		return filepath.Join(home, ".local", "share", "advisor"), nil
	}
}

// serveEnv is the environment "ollama serve" starts with when this app
// starts it: the user's own, except that OLLAMA_HOST is the loopback
// address the advisor will talk to. An Ollama this app starts listens on
// this computer only — the same rule the advisor holds itself to (product
// rule 7) — whatever OLLAMA_HOST said; an Ollama the person starts
// themselves is theirs to configure.
func serveEnv(environ []string, base string) []string {
	hostPort := strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")
	out := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		if k, _, _ := strings.Cut(kv, "="); strings.EqualFold(k, "OLLAMA_HOST") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "OLLAMA_HOST="+hostPort)
}

// startServeProcess launches "<binary> serve" detached from this process,
// with stdout/stderr captured to a log file under this daemon's data
// folder. It is shared by every OS's osStart: Ollama's `serve` subcommand
// is the one documented, cross-platform way to run the server directly,
// whether or not the platform also has its own auto-starting background
// app.
func (b *Backend) startServeProcess(ctx context.Context) error {
	path, ok := b.findBinary()
	if !ok {
		return fmt.Errorf("ollama: start: not installed")
	}
	dir, err := dataDir()
	if err != nil {
		return fmt.Errorf("ollama: start: %w", err)
	}
	logDir := filepath.Join(dir, "ollama", "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("ollama: start: %w", err)
	}
	logPath := filepath.Join(logDir, "server-supervised.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("ollama: start: opening %s: %w", logPath, err)
	}

	cmd := exec.Command(path, "serve")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = serveEnv(os.Environ(), b.host())
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("ollama: start: launching %s serve: %w", path, err)
	}
	// This is Ollama's own long-running process, not a child we wait on
	// here; reap it in the background so it never becomes a zombie once it
	// eventually exits, and close the log file at that point.
	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()
	b.setSupervisedLog(logPath)
	return nil
}

// ForgetData removes what this adapter wrote for the advisor (D-68): the
// log it captured from an Ollama it started, and any installer download
// an interrupted install left in the temporary folder. The copy of Ollama
// itself that a Linux install put under the data folder is a program the
// person asked for, not data about them: it is kept, and said so. Models
// are Ollama's, in Ollama's folder, and are not touched.
func (b *Backend) ForgetData() (removed, kept []string, err error) {
	var errs []error
	if dir, derr := dataDir(); derr == nil {
		logs := filepath.Join(dir, "ollama", "logs")
		if _, serr := os.Stat(logs); serr == nil {
			if rerr := os.RemoveAll(logs); rerr != nil {
				errs = append(errs, rerr)
			} else {
				removed = append(removed, "the log of the Ollama this app started ("+logs+")")
			}
		}
		if _, serr := os.Stat(filepath.Join(dir, "ollama", "bin")); serr == nil {
			kept = append(kept, "the copy of Ollama this app installed ("+filepath.Join(dir, "ollama")+"): it is a program, not data about you — remove that folder to uninstall it")
		} else {
			_ = os.Remove(filepath.Join(dir, "ollama")) // only if empty
		}
	}
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), tempDirPattern))
	for _, m := range matches {
		if rerr := os.RemoveAll(m); rerr != nil {
			errs = append(errs, rerr)
		} else {
			removed = append(removed, "a downloaded Ollama installer ("+m+")")
		}
	}
	b.setSupervisedLog("")
	if len(errs) > 0 {
		err = fmt.Errorf("ollama: %v", errs)
	}
	return removed, kept, err
}
