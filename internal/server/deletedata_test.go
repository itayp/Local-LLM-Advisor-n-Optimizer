package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"advisor/internal/backend"
	"advisor/internal/store"
)

// forgettingBackend is a fake runtime that wrote files of its own.
type forgettingBackend struct {
	fakeBackend
	forgot atomic.Bool
}

func (f *forgettingBackend) ForgetData() ([]string, []string, error) {
	f.forgot.Store(true)
	return []string{"the runtime's log"}, []string{"the runtime itself"}, nil
}

func deleteTestServer(t *testing.T, b backend.Backend) (*Server, *httptest.Server, string, *atomic.Bool, *atomic.Bool) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Advisor")
	path := filepath.Join(dir, "advisor.db")
	st, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(context.Background(), "settings.advanced", "1"); err != nil {
		t.Fatal(err)
	}
	srv := New(nil, st)
	srv.backendList = func() []backend.Backend { return []backend.Backend{b} }
	var forgotOS, stopped atomic.Bool
	srv.forgetOS = func(context.Context) ([]string, []string) {
		forgotOS.Store(true)
		return []string{`"start at login"`}, nil
	}
	srv.SetShutdown(func() { stopped.Store(true) })
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts, path, &forgotOS, &stopped
}

func postDelete(t *testing.T, ts *httptest.Server, body string) (*http.Response, DataDeleteResponse, APIError) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/data/delete", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var ok DataDeleteResponse
	var bad APIError
	if resp.StatusCode == http.StatusOK {
		_ = json.NewDecoder(resp.Body).Decode(&ok)
	} else {
		_ = json.NewDecoder(resp.Body).Decode(&bad)
	}
	return resp, ok, bad
}

// D-68: "delete everything" deletes the database and everything SQLite
// keeps beside it, the data folder when nothing else is in it, the login
// item and the runtime adapter's own files — and then the daemon quits.
func TestDeleteEverythingDeletesItAndQuits(t *testing.T) {
	b := &forgettingBackend{fakeBackend: fakeBackend{name: "ollama"}}
	_, ts, path, forgotOS, stopped := deleteTestServer(t, b)

	resp, out, _ := postDelete(t, ts, `{"confirm": true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for _, p := range store.DatabaseFiles(path) {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there (%v)", p, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("the empty data folder is still there (%v)", err)
	}
	if !forgotOS.Load() || !b.forgot.Load() {
		t.Errorf("forgot the OS entries %v, the runtime's files %v", forgotOS.Load(), b.forgot.Load())
	}
	if !out.Closing || len(out.Problems) != 0 {
		t.Errorf("answer %+v", out)
	}
	joined := strings.Join(out.Deleted, " | ")
	for _, want := range []string{"database", "start at login", "the runtime's log", "data folder"} {
		if !strings.Contains(joined, want) {
			t.Errorf("deleted list %q does not mention %q", joined, want)
		}
	}
	kept := strings.Join(out.Kept, " | ")
	if !strings.Contains(kept, "models") || !strings.Contains(kept, "the runtime itself") {
		t.Errorf("kept list %q does not say what was left and why", kept)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !stopped.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !stopped.Load() {
		t.Error("the daemon did not quit after deleting everything")
	}
}

// It deletes the files it knows, never the folder's other contents: a
// data folder a developer pointed somewhere shared keeps what is not the
// advisor's.
func TestDeleteEverythingLeavesOtherFilesAlone(t *testing.T) {
	_, ts, path, _, _ := deleteTestServer(t, &fakeBackend{name: "ollama"})
	other := filepath.Join(filepath.Dir(path), "notes.txt")
	if err := os.WriteFile(other, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	resp, out, _ := postDelete(t, ts, `{"confirm": true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a file that is not the advisor's was deleted: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the database is still there")
	}
	if !strings.Contains(strings.Join(out.Kept, " "), "data folder") {
		t.Errorf("the answer does not say the folder was kept: %+v", out.Kept)
	}
}

// Never by accident: the request must say so, and nothing runs meanwhile.
func TestDeleteEverythingNeedsConfirmationAndAQuietMoment(t *testing.T) {
	srv, ts, path, forgotOS, stopped := deleteTestServer(t, &fakeBackend{name: "ollama"})
	for _, body := range []string{``, `{}`, `{"confirm": false}`, `{"confirm": "yes"}`} {
		resp, _, e := postDelete(t, ts, body)
		if resp.StatusCode != http.StatusBadRequest || e.Error.Code != "confirm" {
			t.Errorf("body %q: status %d code %q", body, resp.StatusCode, e.Error.Code)
		}
	}
	srv.cat.running.Lock() // a refresh of the model list is running
	resp, _, e := postDelete(t, ts, `{"confirm": true}`)
	srv.cat.running.Unlock()
	if resp.StatusCode != http.StatusConflict || e.Error.Code != "busy" {
		t.Errorf("during a refresh: status %d code %q", resp.StatusCode, e.Error.Code)
	}
	// A refused request left every lock as it was: a refresh can still start.
	if !srv.cat.publicRunning.TryLock() {
		t.Error("a refused delete left the public-scores lock held")
	} else {
		srv.cat.publicRunning.Unlock()
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the database is gone after a refused delete: %v", err)
	}
	if forgotOS.Load() || stopped.Load() {
		t.Error("a refused delete changed something")
	}
}
