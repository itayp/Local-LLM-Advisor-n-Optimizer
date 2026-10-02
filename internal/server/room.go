package server

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"advisor/internal/backend"
	"advisor/internal/diskroom"
	"advisor/internal/figure"
	"advisor/internal/hardware"
	"advisor/internal/recommend"
)

// Is there room? (build-plan step P2-3, ARCHITECTURE.md D-72.)
//
//	GET /api/models/folder                       the folder the runtime reports, and the space free there
//	GET /api/models/pull/check?ollama_tag=…      the answer for one model download, before its button is clicked
//	GET /api/backends/{name}/install/check       the same for the runtime's own installer
//
// POST /api/models/pull and POST /api/backends/{name}/install make the same
// check themselves and refuse (507 not_enough_room) before they touch the
// runtime. All of it is diskroom.Checker.Check: one function, one sentence.

// roomTimeout bounds the reads a check makes (the runtime's folder, a HEAD
// for the installer's size). A check that cannot finish says "unknown", it
// does not hold a button back.
const roomTimeout = 10 * time.Second

// ModelsFolderResponse is GET /api/models/folder: where the runtime keeps
// models, as the runtime says (D-72), and what is free on that drive now.
type ModelsFolderResponse struct {
	Backend string `json:"backend"`
	// Path is the folder; "" when not even a default could be worked out.
	Path string `json:"path"`
	// Known is true when the runtime itself reported Path. False: Path is
	// where it will put models, and How says so.
	Known   bool                  `json:"known"`
	How     string                `json:"how"`
	Control backend.FolderControl `json:"control"`
	// FreeBytes is read from the OS, now, for that folder's drive.
	FreeBytes uint64 `json:"free_bytes" source:"n/a"`
	FreeKnown bool   `json:"free_known"`
	Volume    string `json:"volume,omitempty"`
}

// folderFor is where b keeps its models, as b says; where b has no folder to
// give (not running, never run), the hardware profile's reading, said so.
// Nothing here reads OLLAMA_MODELS: the profile's reading is only the
// fallback for "before the runtime has run" (D-72).
func (s *Server) folderFor(ctx context.Context, b backend.Backend) backend.ModelsFolder {
	ctx, cancel := context.WithTimeout(ctx, roomTimeout)
	defer cancel()
	f, err := b.ModelsFolder(ctx)
	if err == nil && f.Path != "" {
		return f
	}
	if f.Control == "" {
		f.Control = backend.FolderUnknown
	}
	f.Known = false
	select {
	case <-s.hw.ready:
		if s.hw.err == nil {
			st := s.hw.resp.Profile.Storage
			if st.ModelsDir != "" && st.ModelsDir != hardware.Unknown {
				f.Path = st.ModelsDir
				f.How = "from this computer's settings (" + st.ModelsDirSource + "): Ollama has not said where it keeps models"
			}
		}
	default:
	}
	return f
}

func (s *Server) handleModelsFolder(w http.ResponseWriter, r *http.Request) {
	b, ok := s.pullBackend(w)
	if !ok {
		return
	}
	f := s.folderFor(r.Context(), b)
	out := ModelsFolderResponse{Backend: b.Name(), Path: f.Path, Known: f.Known, How: f.How, Control: f.Control}
	if f.Path != "" {
		if fs, err := s.freeSpace(r.Context(), f.Path); err == nil {
			out.FreeBytes, out.FreeKnown, out.Volume = fs.Bytes, true, fs.Volume
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) freeSpace(ctx context.Context, dir string) (hardware.FreeSpace, error) {
	if s.room.Free != nil {
		return s.room.Free(ctx, dir)
	}
	return hardware.FreeSpaceOn(ctx, dir)
}

// pullNeed is the download size of a curated tag: the weights file the tag
// pulls plus the image reader, from the catalogue's own listing. The
// listing is the file in the Hugging Face repository the advisor read the
// header of, not the file Ollama's registry will serve for the tag, so it is
// an estimate of what will be downloaded (rule 4), however exact the Hub's
// number is about its own file.
func (s *Server) pullNeed(ctx context.Context, tag string) (figure.Bytes, bool) {
	if s.store == nil {
		return figure.Bytes{}, false
	}
	entries, err := s.catalogueEntries(ctx)
	if err != nil {
		return figure.Bytes{}, false
	}
	engine, err := recommend.New(entries)
	if err != nil {
		return figure.Bytes{}, false
	}
	for _, e := range entries {
		if e.Model.Size.OllamaTag != tag {
			continue
		}
		file, projector, ok := engine.DefaultFile(e)
		if !ok || file.Bytes == 0 {
			return figure.Bytes{}, false
		}
		n := file.Bytes
		if projector != nil {
			n += projector.Bytes
		}
		return figure.EstimatedBytes(n), true
	}
	return figure.Bytes{}, false
}

// pullRoom checks a curated model download against the drive of the folder
// the runtime reports.
func (s *Server) pullRoom(ctx context.Context, b backend.Backend, tag string) diskroom.Result {
	f := s.folderFor(ctx, b)
	need, known := s.pullNeed(ctx, tag)
	return s.room.Check(ctx, diskroom.Request{Need: need, NeedKnown: known, Dir: f.Path, DirReported: f.Known, Where: diskroom.WhereModels})
}

// installRoom checks the runtime's installer download, which goes to the
// temporary folder, using InstallSizer's size (a HEAD against the host, so
// the length of the very file: Measured).
func (s *Server) installRoom(ctx context.Context, b backend.Backend) diskroom.Result {
	ctx, cancel := context.WithTimeout(ctx, roomTimeout)
	defer cancel()
	req := diskroom.Request{Dir: s.tempFolder(), DirReported: true, Where: diskroom.WhereTemp}
	if sizer, ok := b.(backend.InstallSizer); ok {
		if n, known, err := sizer.InstallSize(ctx); err == nil && known && n > 0 {
			req.Need, req.NeedKnown = figure.MeasuredBytes(uint64(n)), true
		}
	}
	return s.room.Check(ctx, req)
}

func (s *Server) tempFolder() string {
	if s.tempDir != nil {
		return s.tempDir()
	}
	return os.TempDir()
}

// refuseForRoom writes the 507 a download that does not fit gets. The
// message is the check's own sentence, so the button's note and the refusal
// say the same thing.
func refuseForRoom(w http.ResponseWriter, res diskroom.Result) {
	writeError(w, http.StatusInsufficientStorage, "not_enough_room", res.Message)
}

// handlePullCheck is GET /api/models/pull/check?ollama_tag=…
func (s *Server) handlePullCheck(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimSpace(r.URL.Query().Get("ollama_tag"))
	if tag == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "ollama_tag must not be empty")
		return
	}
	curated, err := s.curatedTag(r.Context(), tag)
	if err != nil {
		s.log.Error("reading the catalogue", "err", err)
		writeError(w, http.StatusInternalServerError, "store", "the model list could not be read")
		return
	}
	if !curated {
		writeError(w, http.StatusBadRequest, "not_in_catalogue", "the advisor only downloads models from its own list; "+tag+" is not on it")
		return
	}
	b, ok := s.pullBackend(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.pullRoom(r.Context(), b, tag))
}

// handleBackendInstallCheck is GET /api/backends/{name}/install/check.
func (s *Server) handleBackendInstallCheck(w http.ResponseWriter, r *http.Request) {
	b, ok := s.backendByName(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.installRoom(r.Context(), b))
}
