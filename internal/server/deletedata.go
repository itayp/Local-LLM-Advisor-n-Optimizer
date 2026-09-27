package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"advisor/internal/autostart"
	"advisor/internal/backend"
	"advisor/internal/store"
	"advisor/internal/winapp"
)

// "Delete everything" (build-plan step 12, ARCHITECTURE.md D-68):
//
//	POST /api/data/delete   {"confirm": true} → what was deleted and kept; then the app quits
//
// The Settings screen's button says what it deletes before it is clicked
// (product rule 5). What it removes is everything the advisor itself wrote:
//
//   - the database (every table: this computer's description and name,
//     the runtime checks, the model list, public scores, every test, the
//     settings, the new-model watch's log) — the database file and
//     SQLite's -wal, -shm and -journal files beside it;
//   - "start at login", if it was turned on, and on Windows the app's
//     registration for notifications;
//   - what the runtime adapter wrote for the advisor (the Ollama log it
//     captured, a downloaded installer left in the temporary folder);
//   - the data folder itself, when nothing else is left in it.
//
// It deletes the files it knows by name, never "the folder and whatever is
// in it": a developer's -data-dir can point at a folder that holds other
// things. What it keeps on purpose, and says so: models (they are
// Ollama's, removed one by one on the Models screen) and a copy of Ollama
// the advisor installed on Linux (a program). The browser's own copy of
// the settings is cleared by the page.
//
// Then the daemon quits, so nothing it does afterwards writes a new file;
// opening the app again starts it fresh.

// DataDeleteRequest is the body of POST /api/data/delete.
type DataDeleteRequest struct {
	// Confirm must be true: the one request that deletes everything is
	// never a bare POST.
	Confirm bool `json:"confirm"`
}

// DataDeleteResponse says what happened, in words.
type DataDeleteResponse struct {
	Deleted  []string `json:"deleted"`
	Kept     []string `json:"kept"`
	Problems []string `json:"problems,omitempty"`
	// Closing is true when the app quits now (always, once deletion ran).
	Closing bool `json:"closing"`
}

// shutdownDelay lets the answer reach the browser before the daemon quits.
const shutdownDelay = 500 * time.Millisecond

// forgetOS removes what the advisor registered with the operating system
// outside its data folder: "start at login" and the Windows notification
// identity. A field on Server so tests never touch the real login items or
// registry.
type forgetOSFunc func(ctx context.Context) (removed, problems []string)

func defaultForgetOS(ctx context.Context) (removed, problems []string) {
	if ok, err := autostart.Forget(ctx); err != nil {
		problems = append(problems, `"start at login" could not be turned off: `+err.Error())
	} else if ok {
		removed = append(removed, `"start at login"`)
	}
	if ok, err := winapp.UnregisterIdentity(); err != nil {
		problems = append(problems, "the app's registration for notifications could not be removed: "+err.Error())
	} else if ok {
		removed = append(removed, "the app's registration for Windows notifications")
	}
	return removed, problems
}

// SetShutdown is how the daemon quits once everything is deleted: main
// passes the function that stops it.
func (s *Server) SetShutdown(stop func()) { s.shutdown = stop }

func (s *Server) handleDataDelete(w http.ResponseWriter, r *http.Request) {
	var req DataDeleteRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !req.Confirm {
		writeError(w, http.StatusBadRequest, "confirm", `deleting everything needs {"confirm": true}`)
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "no_store", "the advisor has no data to delete")
		return
	}

	// Nothing may be half-way through writing: a test, a download, an
	// install, a refresh of the model list or the public scores, a check
	// for new models. The locks taken here are never given back — the
	// daemon quits once this is done — so nothing new can start either.
	if s.bench != nil && s.bench.Active() != 0 {
		writeError(w, http.StatusConflict, "busy", "a test is running; stop it first, then delete everything")
		return
	}
	if s.pulls.current().Status == "running" {
		writeError(w, http.StatusConflict, "busy", "a model is downloading; stop the download first, then delete everything")
		return
	}
	if s.installs.anyRunning() {
		writeError(w, http.StatusConflict, "busy", "Ollama is being installed; wait for it to finish, then delete everything")
		return
	}
	var held []*sync.Mutex
	for _, l := range []*sync.Mutex{&s.cat.running, &s.cat.publicRunning, &s.wch.running} {
		if !l.TryLock() {
			for _, h := range held {
				h.Unlock()
			}
			writeError(w, http.StatusConflict, "busy", "the model list or the public scores are being fetched; wait a moment, then delete everything")
			return
		}
		held = append(held, l)
	}

	resp := DataDeleteResponse{Deleted: []string{}, Kept: []string{
		"the models you downloaded: they are Ollama's, in Ollama's own folder — remove them on the Models screen",
	}, Closing: true}

	forget := s.forgetOS
	if forget == nil {
		forget = defaultForgetOS
	}
	removed, problems := forget(r.Context())
	resp.Deleted = append(resp.Deleted, removed...)
	resp.Problems = append(resp.Problems, problems...)

	for _, b := range s.backendList() {
		if f, ok := b.(backend.DataForgetter); ok {
			removed, kept, err := f.ForgetData()
			resp.Deleted = append(resp.Deleted, removed...)
			resp.Kept = append(resp.Kept, kept...)
			if err != nil {
				resp.Problems = append(resp.Problems, err.Error())
			}
		}
	}

	dbPath := s.store.Path()
	dataDir := filepath.Dir(dbPath)
	if err := s.store.Close(); err != nil {
		s.log.Warn("closing the database before deleting it", "err", err)
	}
	gone := 0
	for _, p := range store.DatabaseFiles(dbPath) {
		err := os.Remove(p)
		switch {
		case err == nil:
			gone++
		case errors.Is(err, os.ErrNotExist):
		default:
			resp.Problems = append(resp.Problems, "could not delete "+p+": "+err.Error())
		}
	}
	if gone > 0 {
		resp.Deleted = append(resp.Deleted, "the advisor's database ("+dbPath+"): this computer's description, the model list and public scores, every test, your settings and the new-model log")
	}
	if err := os.Remove(dataDir); err == nil {
		resp.Deleted = append(resp.Deleted, "the advisor's data folder ("+dataDir+")")
	} else if !errors.Is(err, os.ErrNotExist) {
		resp.Kept = append(resp.Kept, "the data folder ("+dataDir+"), because something other than the advisor's own files is in it")
	}
	s.log.Info("deleted everything the advisor stored; quitting", "deleted", len(resp.Deleted), "problems", len(resp.Problems))

	writeJSON(w, http.StatusOK, resp)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if s.shutdown != nil {
		go func() {
			time.Sleep(shutdownDelay)
			s.shutdown()
		}()
	}
}
