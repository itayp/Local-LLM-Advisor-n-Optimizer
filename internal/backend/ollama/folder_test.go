package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"advisor/internal/backend"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "folder", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// withGOOS runs a test as another operating system's rules for who controls
// the folder, and keeps the advisor's data folder out of the real one.
func withGOOS(t *testing.T, name string) {
	t.Helper()
	old := goos
	goos = name
	t.Cleanup(func() { goos = old })
	t.Setenv("ADVISOR_DATA_DIR", t.TempDir())
}

// showServer is an Ollama with the named models, each answering /api/show
// with a fixture ("" = the model is listed but Show fails).
func showServer(t *testing.T, models []string, shows map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		var sb strings.Builder
		sb.WriteString(`{"models":[`)
		for i, m := range models {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{"name":"` + m + `","model":"` + m + `","size":1}`)
		}
		sb.WriteString("]}")
		_, _ = w.Write([]byte(sb.String()))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Model string }
		_ = decodeJSON(r, &req)
		f, ok := shows[req.Model]
		if !ok || f == "" {
			http.Error(w, `{"error":"nope"}`, http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(fixture(t, f))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func folderBackend(t *testing.T, srv *httptest.Server, e fakeEnv) *Backend {
	t.Helper()
	if e.vars == nil {
		e.vars = map[string]string{}
	}
	if e.home == "" {
		e.home = t.TempDir()
	}
	if srv != nil {
		e.vars["OLLAMA_HOST"] = srv.URL
	} else {
		e.vars["OLLAMA_HOST"] = "127.0.0.1:1" // nothing listens
	}
	b := backendWithEnv(e)
	b.defaultFolder = func(context.Context) (string, string) {
		return "/default/.ollama/models", "Ollama's default on this test"
	}
	b.journal = func(context.Context) string { return "" }
	return b
}

// D-72: the folder is the one Ollama reports, read from the FROM line of
// /api/show's Modelfile for an installed model — on every OS's path syntax.
func TestModelsFolderFromAShowFixture(t *testing.T) {
	for _, c := range []struct {
		name, show, want string
	}{
		{"darwin", "show-darwin.json", "/Users/jo/.ollama/models"},
		{"windows", "show-windows.json", `D:\Ollama models`},
		{"linux, weights and projector", "show-vision-linux.json", "/mnt/data/ollama/models"},
	} {
		t.Run(c.name, func(t *testing.T) {
			withGOOS(t, "linux")
			srv := showServer(t, []string{"m:1"}, map[string]string{"m:1": c.show})
			got, err := folderBackend(t, srv, fakeEnv{}).ModelsFolder(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !got.Known || got.Path != c.want {
				t.Fatalf("got %+v, want known %q", got, c.want)
			}
			if !strings.Contains(got.How, "model Ollama has installed") || !strings.Contains(got.How, "m:1") {
				t.Errorf("How = %q", got.How)
			}
		})
	}
}

// A cloud model has no local blob: the next installed model is asked.
func TestModelsFolderSkipsAModelWithNoLocalFiles(t *testing.T) {
	withGOOS(t, "darwin")
	srv := showServer(t, []string{"gpt-oss:120b-cloud", "broken:1", "llama3.2:1b"},
		map[string]string{"gpt-oss:120b-cloud": "show-cloud.json", "broken:1": "", "llama3.2:1b": "show-darwin.json"})
	got, _ := folderBackend(t, srv, fakeEnv{}).ModelsFolder(context.Background())
	if !got.Known || got.Path != "/Users/jo/.ollama/models" || !strings.Contains(got.How, "llama3.2:1b") {
		t.Fatalf("got %+v", got)
	}
}

// Show is tried for a few models, not a whole library.
func TestModelsFolderAsksShowAboutAFewModelsOnly(t *testing.T) {
	withGOOS(t, "darwin")
	names := []string{"a:1", "b:1", "c:1", "d:1", "e:1", "f:1"}
	shows := map[string]string{"f:1": "show-darwin.json"} // only the sixth would answer
	got, _ := folderBackend(t, showServer(t, names, shows), fakeEnv{}).ModelsFolder(context.Background())
	if got.Known {
		t.Fatalf("read a folder from the sixth model: %+v", got)
	}
}

// When nothing is installed (or Ollama is not running), the server's own log
// is read: the advisor's capture first; the last "server config" line wins.
func TestModelsFolderFromALogFixture(t *testing.T) {
	for _, c := range []struct {
		log, want string
	}{
		{"server-config-darwin.log", "/Users/jo/.ollama/models"},
		{"server-config-windows.log", `D:\Ollama models`}, // doubled backslashes undone; a space kept
		{"server-config-linux-restarted.log", "/mnt/data/ollama models/models"},
	} {
		t.Run(c.log, func(t *testing.T) {
			withGOOS(t, "linux")
			dir := t.TempDir()
			logPath := filepath.Join(dir, "server-supervised.log")
			if err := os.WriteFile(logPath, fixture(t, c.log), 0o600); err != nil {
				t.Fatal(err)
			}
			b := folderBackend(t, showServer(t, nil, nil), fakeEnv{}) // running, no models
			b.setSupervisedLog(logPath)
			got, err := b.ModelsFolder(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !got.Known || got.Path != c.want || !strings.Contains(got.How, "log") {
				t.Fatalf("got %+v, want known %q", got, c.want)
			}
		})
	}
}

// Ollama's app writes its own server.log; the advisor reads it where it is.
func TestModelsFolderFromOllamasOwnLog(t *testing.T) {
	withGOOS(t, "darwin")
	home := t.TempDir()
	logDir := filepath.Join(home, ".ollama", "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "server.log"), fixture(t, "server-config-darwin.log"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := folderBackend(t, nil, fakeEnv{home: home})
	if runtimeIsWindows() {
		t.Skip("Ollama's own log lives under LOCALAPPDATA on Windows")
	}
	got, _ := b.ModelsFolder(context.Background())
	if !got.Known || got.Path != "/Users/jo/.ollama/models" {
		t.Fatalf("got %+v", got)
	}
}

// A systemd install logs to the journal.
func TestModelsFolderFromTheJournal(t *testing.T) {
	withGOOS(t, "linux")
	b := folderBackend(t, nil, fakeEnv{})
	b.journal = func(context.Context) string { return string(fixture(t, "server-config-linux-restarted.log")) }
	got, _ := b.ModelsFolder(context.Background())
	if !got.Known || got.Path != "/mnt/data/ollama models/models" {
		t.Fatalf("got %+v", got)
	}
}

// Neither answers: Known is false and the path is the OS default, said to be
// where Ollama will put models. An empty log value is no answer.
func TestModelsFolderWithNeither(t *testing.T) {
	withGOOS(t, "linux")
	dir := t.TempDir()
	logPath := filepath.Join(dir, "s.log")
	if err := os.WriteFile(logPath, fixture(t, "server-config-empty.log"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, setup := range map[string]func(*Backend){
		"no log at all":  func(*Backend) {},
		"an empty value": func(b *Backend) { b.setSupervisedLog(logPath) },
	} {
		t.Run(name, func(t *testing.T) {
			b := folderBackend(t, nil, fakeEnv{})
			setup(b)
			got, err := b.ModelsFolder(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.Known || got.Path != "/default/.ollama/models" {
				t.Fatalf("got %+v", got)
			}
			if !strings.Contains(got.How, "will put") {
				t.Errorf("How = %q does not say it is where Ollama will put models", got.How)
			}
		})
	}
	// And when even the default cannot be worked out: unknown, with no path.
	b := folderBackend(t, nil, fakeEnv{})
	b.defaultFolder = func(context.Context) (string, string) { return "unknown", "the home folder could not be found" }
	got, _ := b.ModelsFolder(context.Background())
	if got.Known || got.Path != "" || !strings.Contains(got.How, "could not be worked out") {
		t.Fatalf("got %+v", got)
	}
}

func TestModelsFolderStopsWhenTheContextEnds(t *testing.T) {
	withGOOS(t, "linux")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := folderBackend(t, nil, fakeEnv{}).ModelsFolder(ctx); err == nil {
		t.Fatal("a cancelled context must be an error")
	}
}

func TestFolderOfBlobPath(t *testing.T) {
	d := strings.Repeat("a1", 32)
	for p, want := range map[string]string{
		"/Users/jo/.ollama/models/blobs/sha256-" + d:         "/Users/jo/.ollama/models",
		`C:\Users\Jo Smith\.ollama\models\blobs\sha256-` + d: `C:\Users\Jo Smith\.ollama\models`,
		`\\nas\share\models\blobs\sha256-` + d:               `\\nas\share\models`,
		"/blobs/sha256-" + d:                                 "",
		`C:\blobs\sha256-` + d:                               "",
		"models/blobs/sha256-" + d:                           "", // relative
		"/m/blobs/sha256-" + d[:10]:                          "", // not a digest
		"/m/other/sha256-" + d:                               "", // not in blobs
		"llama3.2:1b":                                        "",
	} {
		got, ok := folderOfBlobPath(p)
		if got != want || ok != (want != "") {
			t.Errorf("folderOfBlobPath(%q) = %q, %v; want %q", p, got, ok, want)
		}
	}
	// The commented FROM of a Modelfile is not a path.
	if _, ok := folderFromModelfile("# FROM /x/blobs/sha256-" + d + "\n"); ok {
		t.Error("a comment was read as a FROM line")
	}
}

// Who can change the folder (D-72): the app where there is one, an
// administrator for a system service, the advisor for an Ollama it starts.
func TestModelsFolderControl(t *testing.T) {
	const home = "/Users/jo"
	file := func(paths ...string) map[string]int64 {
		m := map[string]int64{}
		for _, p := range paths {
			m[p] = 1000
		}
		return m
	}
	for _, c := range []struct {
		name string
		goos string
		env  fakeEnv
		want backend.FolderControl
	}{
		{"mac, the app", "darwin", fakeEnv{dirs: map[string]bool{"/Applications/Ollama.app": true}, home: home}, backend.FolderRuntimeApp},
		{"mac, the app in the user's Applications", "darwin", fakeEnv{dirs: map[string]bool{home + "/Applications/Ollama.app": true}, home: home}, backend.FolderRuntimeApp},
		{"mac, only a command-line install", "darwin", fakeEnv{paths: map[string]string{"ollama": "/opt/homebrew/bin/ollama"}, home: home}, backend.FolderAdvisor},
		{"mac, nothing", "darwin", fakeEnv{home: home}, backend.FolderUnknown},
		{"windows, the app", "windows", fakeEnv{vars: map[string]string{"LOCALAPPDATA": `C:\Users\jo\AppData\Local`},
			files: file(`C:\Users\jo\AppData\Local\Programs\Ollama\ollama app.exe`)}, backend.FolderRuntimeApp},
		{"windows, the app beside ollama.exe on the path", "windows", fakeEnv{paths: map[string]string{"ollama.exe": `D:\Tools\Ollama\ollama.exe`},
			files: file(`D:\Tools\Ollama\ollama app.exe`)}, backend.FolderRuntimeApp},
		{"windows, ollama.exe without the app", "windows", fakeEnv{paths: map[string]string{"ollama.exe": `D:\Tools\ollama.exe`}}, backend.FolderAdvisor},
		{"windows, nothing", "windows", fakeEnv{}, backend.FolderUnknown},
		{"linux, systemd unit", "linux", fakeEnv{files: file("/etc/systemd/system/ollama.service"), paths: map[string]string{"ollama": "/usr/local/bin/ollama"}}, backend.FolderAdministrator},
		{"linux, a distribution's unit", "linux", fakeEnv{files: file("/usr/lib/systemd/system/ollama.service")}, backend.FolderAdministrator},
		{"linux, a program on the path, no unit", "linux", fakeEnv{paths: map[string]string{"ollama": "/usr/local/bin/ollama"}}, backend.FolderAdvisor},
		{"linux, nothing", "linux", fakeEnv{}, backend.FolderUnknown},
	} {
		t.Run(c.name, func(t *testing.T) {
			withGOOS(t, c.goos)
			if got := backendWithEnv(c.env).folderControl(); got != c.want {
				t.Fatalf("control = %q, want %q", got, c.want)
			}
		})
	}
	// The copy this app installs on Linux is one it starts itself.
	withGOOS(t, "linux")
	dir, _ := dataDir()
	mine := filepath.Join(dir, "ollama", "bin", "ollama")
	if got := backendWithEnv(fakeEnv{files: file(mine)}).folderControl(); got != backend.FolderAdvisor {
		t.Fatalf("the user-space install: control = %q", got)
	}
}

func runtimeIsWindows() bool { return os.PathSeparator == '\\' }

func decodeJSON(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
