package hardware

import (
	"context"
	"errors"
	"path"
	"strings"
)

// detectStorage reads the free space on the volume that holds (or will
// hold) the models. The OS paths chose Storage.ModelsDir; when the folder
// does not exist yet — Ollama is not installed — the space is read on its
// nearest existing parent, which is the volume the folder will be created on.
//
// This is the profile's reading, taken once at daemon start. The runtime
// itself is the authority on which folder its models are in (D-72), and a
// check made before a download asks FreeSpaceOn for that folder, fresh.
func detectStorage(ctx context.Context, e env, p *Profile) {
	_ = ctx
	dir := p.Storage.ModelsDir
	if dir == "" || dir == Unknown {
		p.problem("the models folder is unknown, so free disk space was not read")
		return
	}
	target, exists := nearestExisting(e, p.OS, dir)
	p.Storage.ModelsDirExists = exists
	free, err := e.diskFree(target)
	if err != nil {
		p.problem("free disk space could not be read for %s: %v", target, err)
		return
	}
	p.Storage.FreeBytes, p.Storage.FreeKnown = free, true
}

// nearestExisting is dir when it exists, else its nearest existing parent —
// the volume a folder that is not there yet will be created on.
func nearestExisting(e env, goos, dir string) (target string, exists bool) {
	has := func(x string) bool {
		if goos == "linux" && e.files() != nil {
			return fsExists(e.files(), x)
		}
		return e.exists(x)
	}
	if has(dir) {
		return dir, true
	}
	target = dir
	for {
		parent := parentDir(target, goos)
		if parent == target {
			break
		}
		target = parent
		if has(target) {
			break
		}
	}
	return target, false
}

// FreeSpace is a reading of the space available to this user on one volume.
type FreeSpace struct {
	// Dir is the folder that was asked about.
	Dir string
	// ReadOn is the folder the volume was actually read on: Dir, or its
	// nearest existing parent when Dir has not been created yet.
	ReadOn    string
	DirExists bool
	// Bytes is what this user can write there now (quotas included). It is
	// a value read from the OS: never an estimate.
	Bytes uint64
	// Volume is the drive's name where the OS has such names ("C:" on
	// Windows), and "" where it does not.
	Volume string
}

// FreeSpaceOn reads, now, the space available on the volume that holds
// dir (or will hold it, when it is not created yet). It reads the disk
// each time it is called and never the profile: a download's check has to
// see what is free at the moment of the click.
func FreeSpaceOn(ctx context.Context, dir string) (FreeSpace, error) {
	return freeSpaceOn(ctx, newSystemEnv(), dir)
}

func freeSpaceOn(ctx context.Context, e env, dir string) (FreeSpace, error) {
	_ = ctx
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == Unknown {
		return FreeSpace{}, errors.New("hardware: no folder to read free space on")
	}
	goos := e.goos()
	target, exists := nearestExisting(e, goos, dir)
	free, err := e.diskFree(target)
	if err != nil {
		return FreeSpace{}, err
	}
	return FreeSpace{Dir: dir, ReadOn: target, DirExists: exists, Bytes: free, Volume: VolumeName(dir, goos)}, nil
}

// VolumeName is the drive a path is on, for the operating systems that name
// drives: "C:" for `C:\Users\me`. It is "" for a path without a drive
// letter (a network share, or any path on macOS and Linux, where a volume
// has no name a person would recognise from the path alone).
func VolumeName(dir, goos string) string {
	if goos != "windows" {
		return ""
	}
	if len(dir) >= 2 && dir[1] == ':' && isLetter(dir[0]) {
		return strings.ToUpper(dir[:1]) + ":"
	}
	return ""
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// DefaultModelsDir is where Ollama will put models when it has not said
// where they are: the same reading the hardware profile makes (the
// OLLAMA_MODELS variable if one is set, else the OS default), for a caller
// that needs it without waiting for the whole profile. It is a fallback for
// "before Ollama has run" (D-72), never the folder Ollama reports.
func DefaultModelsDir(ctx context.Context) (dir, source string) {
	e := newSystemEnv()
	switch e.goos() {
	case "darwin":
		return darwinModelsDir(ctx, e)
	case "windows":
		user, machine := savedEnvVar("OLLAMA_MODELS")
		return windowsModelsDir(e, winProbe{OllamaModelsUser: user, OllamaModelsMachine: machine})
	default:
		return linuxModelsDir(e, e.files())
	}
}

// parentDir is filepath.Dir for the target OS's syntax, so the Windows path
// logic is tested on every runner.
func parentDir(dir, goos string) string {
	if goos == "windows" {
		d := strings.TrimRight(dir, `\/`)
		i := strings.LastIndexAny(d, `\/`)
		switch {
		case i < 0:
			return dir
		case i == 2 && len(d) > 1 && d[1] == ':': // "C:\Users" -> "C:\"
			return d[:3]
		case i < 2:
			return dir
		}
		return d[:i]
	}
	return path.Dir(dir)
}
