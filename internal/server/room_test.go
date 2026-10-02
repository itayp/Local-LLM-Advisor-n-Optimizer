package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"advisor/internal/backend"
	"advisor/internal/diskroom"
	"advisor/internal/figure"
	"advisor/internal/hardware"
)

// roomServer is the recommendation test server (a refreshed catalogue whose
// one model has a known file size) with a runtime that reports a models
// folder and a disk whose free space the test sets. folders records every
// folder the disk was asked about.
type roomEnv struct {
	srv  *Server
	url  string
	fake *fakeBackend

	mu      sync.Mutex
	free    uint64
	freeErr error
	asked   []string
}

func newRoomEnv(t *testing.T, free uint64) *roomEnv {
	t.Helper()
	fake := &fakeBackend{
		name:   "ollama",
		status: backend.Status{State: backend.StateRunning, Version: "0.34.2"},
		folder: backend.ModelsFolder{Path: "/mnt/models/ollama", Known: true, How: "from a model Ollama has installed", Control: backend.FolderAdvisor},
	}
	srv, ts := recommendTestServer(t, fake)
	e := &roomEnv{srv: srv, url: ts.URL, fake: fake, free: free}
	srv.room.Free = func(_ context.Context, dir string) (hardware.FreeSpace, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.asked = append(e.asked, dir)
		if e.freeErr != nil {
			return hardware.FreeSpace{}, e.freeErr
		}
		return hardware.FreeSpace{Dir: dir, ReadOn: dir, Bytes: e.free, Volume: hardware.VolumeName(dir, "windows")}, nil
	}
	return e
}

func (e *roomEnv) setFree(n uint64, err error) {
	e.mu.Lock()
	e.free, e.freeErr = n, err
	e.mu.Unlock()
}

func (e *roomEnv) check(t *testing.T) diskroom.Result {
	t.Helper()
	var res diskroom.Result
	getJSON(t, e.url+"/api/models/pull/check?ollama_tag=llama3.2:1b", http.StatusOK, &res)
	return res
}

func (e *roomEnv) need(t *testing.T) uint64 {
	t.Helper()
	res := e.check(t)
	if res.Need == nil || res.Need.Value == 0 {
		t.Fatalf("the catalogue's file size is not known: %+v", res)
	}
	return res.Need.Value
}

func waitIdle(srv *Server) {
	for i := 0; i < 200 && srv.pulls.current().Status == "running"; i++ {
		time.Sleep(5 * time.Millisecond)
	}
}

// P2-3: a download that would fill the disk is refused before the runtime is
// called, with both numbers in the sentence.
func TestPullIsRefusedBeforeTheRuntimeIsCalled(t *testing.T) {
	e := newRoomEnv(t, 1_000)
	need := e.need(t)
	e.setFree(need-1, nil)

	var apiErr APIError
	resp, err := http.Post(e.url+"/api/models/pull", "application/json", strings.NewReader(`{"ollama_tag":"llama3.2:1b"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInsufficientStorage || apiErr.Error.Code != "not_enough_room" {
		t.Fatalf("status %d, error %+v", resp.StatusCode, apiErr)
	}
	msg := apiErr.Error.Message
	if !strings.Contains(msg, "This needs about") || !strings.Contains(msg, "the drive Ollama saves models to") || !strings.Contains(msg, "free.") ||
		!strings.Contains(msg, "Remove a model you no longer use, or free up space.") {
		t.Errorf("message: %q", msg)
	}
	if n := e.fake.pullCalls.Load(); n != 0 {
		t.Fatalf("the runtime was asked to pull %d times; a refused download must not reach it", n)
	}
	if st := e.srv.pulls.current(); st.Status != "idle" {
		t.Errorf("a pull was recorded: %+v", st)
	}
}

// The folder checked is the one the runtime reports, read fresh each time.
func TestTheCheckReadsTheFolderTheRuntimeReports(t *testing.T) {
	e := newRoomEnv(t, 1_000)
	e.check(t)
	e.check(t)
	if len(e.asked) != 2 || e.asked[0] != "/mnt/models/ollama" || e.asked[1] != "/mnt/models/ollama" {
		t.Fatalf("the disk was read for %v", e.asked)
	}
	// Not the profile's folder (/home/u/.ollama/models in other tests): the
	// runtime's own answer wins.
	var f ModelsFolderResponse
	getJSON(t, e.url+"/api/models/folder", http.StatusOK, &f)
	if f.Path != "/mnt/models/ollama" || !f.Known || f.Control != backend.FolderAdvisor || !f.FreeKnown || f.FreeBytes != 1_000 {
		t.Fatalf("folder: %+v", f)
	}
}

// Where the runtime has not said, the profile's reading is the fallback and
// is not dressed as Ollama's word.
func TestTheProfilesFolderIsOnlyTheFallback(t *testing.T) {
	e := newRoomEnv(t, 100e9)
	e.fake.folder = backend.ModelsFolder{Control: backend.FolderUnknown}
	var f ModelsFolderResponse
	getJSON(t, e.url+"/api/models/folder", http.StatusOK, &f)
	if f.Known || f.Path == "" || f.Path == "/mnt/models/ollama" {
		t.Fatalf("folder: %+v", f)
	}
	if !strings.Contains(f.How, "has not said") {
		t.Errorf("How = %q", f.How)
	}
	res := e.check(t)
	if !strings.Contains(res.Message, "will save") {
		t.Errorf("a default folder is where Ollama will save: %q", res.Message)
	}
}

// Just enough, then a download that leaves little: allowed, with a warning
// before the click and the figure of what is left.
func TestPullWithLittleLeftIsAllowedButWarned(t *testing.T) {
	e := newRoomEnv(t, 0)
	need := e.need(t)
	e.setFree(need+7e9, nil)

	res := e.check(t)
	if res.Verdict != diskroom.Low || res.Left == nil || res.Left.Value != 7e9 || res.Left.Source != figure.Estimated {
		t.Fatalf("check: %+v", res)
	}
	if res.Volume != "" && !strings.Contains(res.Message, res.Volume) {
		t.Errorf("message %q does not name the drive", res.Message)
	}
	if !strings.Contains(res.Message, "After this download, about 7 GB will be left on") {
		t.Errorf("message: %q", res.Message)
	}
	postJSON(t, e.url+"/api/models/pull", `{"ollama_tag":"llama3.2:1b"}`, http.StatusAccepted, nil)
	waitIdle(e.srv)
	if e.fake.pullCalls.Load() != 1 {
		t.Fatal("the runtime should have been asked to pull")
	}
}

// Plenty of room: nothing to warn about, and the pull starts.
func TestPullWithRoomStarts(t *testing.T) {
	e := newRoomEnv(t, 0)
	e.setFree(e.need(t)+500e9, nil)
	if res := e.check(t); res.Verdict != diskroom.Enough || len(res.Actions) != 0 {
		t.Fatalf("check: %+v", res)
	}
	postJSON(t, e.url+"/api/models/pull", `{"ollama_tag":"llama3.2:1b"}`, http.StatusAccepted, nil)
	waitIdle(e.srv)
	if e.fake.pullCalls.Load() != 1 {
		t.Fatal("the runtime should have been asked to pull")
	}
}

// When free space cannot be read, say so and let the person go on.
func TestUnknownFreeSpaceNeverBlocks(t *testing.T) {
	e := newRoomEnv(t, 0)
	e.setFree(0, errors.New("volume gone"))
	res := e.check(t)
	if res.Verdict != diskroom.Unknown || res.FreeKnown || !strings.Contains(res.Message, "could not read") {
		t.Fatalf("check: %+v", res)
	}
	postJSON(t, e.url+"/api/models/pull", `{"ollama_tag":"llama3.2:1b"}`, http.StatusAccepted, nil)
	waitIdle(e.srv)
	if e.fake.pullCalls.Load() != 1 {
		t.Fatal("an unreadable disk must not stop a download")
	}
}

// A size the catalogue does not list for a tag (a size with no file read yet)
// is unknown too: nothing to compare, nothing blocked.
func TestUnknownDownloadSizeNeverBlocks(t *testing.T) {
	e := newRoomEnv(t, 1)
	var res diskroom.Result
	getJSON(t, e.url+"/api/models/pull/check?ollama_tag=llama3.2:3b", http.StatusOK, &res) // no file in the fake hub
	if res.Verdict != diskroom.Unknown || res.Need != nil || res.Refuses() {
		t.Fatalf("check: %+v", res)
	}
	postJSON(t, e.url+"/api/models/pull", `{"ollama_tag":"llama3.2:3b"}`, http.StatusAccepted, nil)
	waitIdle(e.srv)
	if e.fake.pullCalls.Load() != 1 {
		t.Fatal("an unknown size must not stop a download")
	}
}

func TestPullCheckOnlyAnswersForTheCatalogue(t *testing.T) {
	e := newRoomEnv(t, 1)
	getJSON(t, e.url+"/api/models/pull/check?ollama_tag=evil/model:latest", http.StatusBadRequest, nil)
	getJSON(t, e.url+"/api/models/pull/check", http.StatusBadRequest, nil)
}

// sizedBackend is a runtime with an installer of a stated size.
type sizedBackend struct {
	*fakeBackend
	size    int64
	known   bool
	err     error
	install int
}

func (s *sizedBackend) InstallSize(context.Context) (int64, bool, error) {
	return s.size, s.known, s.err
}
func (s *sizedBackend) Install(context.Context, func(backend.InstallProgress)) error {
	s.install++
	return nil
}

// Item 3: the installer's download into the temporary folder gets the same
// check, from InstallSizer's size, and is refused before Install runs.
func TestInstallerIsCheckedAgainstTheTemporaryFolder(t *testing.T) {
	sb := &sizedBackend{fakeBackend: &fakeBackend{name: "ollama", status: backend.Status{State: backend.StateNotInstalled}}, size: 1_800_000_000, known: true}
	srv, ts := newCatalogTestServer(t, sb)
	srv.tempDir = func() string { return `C:\Users\jo\AppData\Local\Temp` }
	var asked string
	var free uint64 = 500_000_000
	srv.room.Free = func(_ context.Context, dir string) (hardware.FreeSpace, error) {
		asked = dir
		return hardware.FreeSpace{Dir: dir, Bytes: free, Volume: hardware.VolumeName(dir, "windows")}, nil
	}

	var res diskroom.Result
	getJSON(t, ts.URL+"/api/backends/ollama/install/check", http.StatusOK, &res)
	if res.Verdict != diskroom.NotEnough || res.Need == nil || res.Need.Source != figure.Measured || res.Need.Value != 1_800_000_000 {
		t.Fatalf("check: %+v", res)
	}
	if asked != `C:\Users\jo\AppData\Local\Temp` || !strings.Contains(res.Message, "1.8 GB") || !strings.Contains(res.Message, "500 MB") {
		t.Errorf("asked %q, message %q", asked, res.Message)
	}

	var apiErr APIError
	resp, err := http.Post(ts.URL+"/api/backends/ollama/install", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInsufficientStorage || apiErr.Error.Code != "not_enough_room" {
		t.Fatalf("status %d, %+v", resp.StatusCode, apiErr)
	}
	if sb.install != 0 {
		t.Fatal("Install ran after a refusal")
	}

	// With room, it runs; with the size unreadable, it runs too.
	free = 50e9
	postJSON(t, ts.URL+"/api/backends/ollama/install", "", http.StatusAccepted, nil)
	sb.known, free = false, 1
	for i := 0; i < 200 && srv.installs.anyRunning(); i++ {
		time.Sleep(5 * time.Millisecond)
	}
	getJSON(t, ts.URL+"/api/backends/ollama/install/check", http.StatusOK, &res)
	if res.Verdict != diskroom.Unknown || res.Refuses() {
		t.Fatalf("unknown installer size: %+v", res)
	}
}

// Settings' "open the models folder" opens the folder the runtime reports,
// not the profile's reading of a variable.
func TestOpenModelsDirOpensTheFolderTheRuntimeReports(t *testing.T) {
	e := newRoomEnv(t, 1_000)
	var opened string
	e.srv.open = func(dir string) error { opened = dir; return nil }
	postJSON(t, e.url+"/api/settings/open-models-dir", "", http.StatusOK, nil)
	if opened != "/mnt/models/ollama" {
		t.Fatalf("opened %q", opened)
	}
}
