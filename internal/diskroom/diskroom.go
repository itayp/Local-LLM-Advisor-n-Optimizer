// Package diskroom answers one question before anything is downloaded or
// copied onto this computer: is there room? (build-plan step P2-3,
// ARCHITECTURE.md D-72.)
//
// It is one function, Checker.Check, because the same answer is wanted in
// four places: a model pull (the catalogue's file size), the Ollama
// installer's download into the temporary folder (InstallSizer's size), the
// copy of a local model file into Ollama (P2-10), and the button that
// offers each of them, which shows the answer before it is clicked (product
// rule 5). The sentence that says it is written here once, so the refusal
// and the note beside the button can never disagree.
//
// Two rules shape it:
//
//   - Free space is read now, from the OS, for the folder the download will
//     be written to — never from the hardware profile taken at start-up, and
//     for the folder the runtime reports, not one guessed from a variable.
//   - A value the advisor could not read is unknown, and unknown never
//     blocks (CLAUDE.md, "Unknown is unknown"): the verdict says so and the
//     person may go on.
package diskroom

import (
	"context"
	"math"
	"strconv"
	"strings"

	"advisor/internal/figure"
	"advisor/internal/hardware"
)

// Config holds the check's one constant.
type Config struct {
	// LowAfter is the free space below which a download that fits still
	// gets a warning: after it, less than this is left on the drive.
	//
	// CHOSEN, 10 GB: enough for an operating-system update (Windows stages
	// several GB while it installs) and for a second small model, which is
	// what a person who has just downloaded one is likely to want next. The
	// downloads this app offers run from about 1 GB to tens of GB, so a
	// smaller margin would warn about nothing and a larger one about almost
	// every download on a small laptop disk. What would settle it: how often
	// a download that left 5 to 10 GB ends in a full drive or a failed
	// update on the fleet (nothing has been seen yet), and the size of the
	// smallest model a recommendation card offers once the catalogue grows
	// (it is the "second small model" in the reason above).
	LowAfter uint64
}

// DefaultConfig is the chosen constant. Decimal gigabytes, as downloads
// and drives are counted.
func DefaultConfig() Config { return Config{LowAfter: 10 * 1e9} }

// Where is which folder a download is written to, in the person's words.
type Where string

const (
	// WhereModels: the folder the runtime keeps its models in.
	WhereModels Where = "models"
	// WhereTemp: the operating system's temporary folder, where an installer
	// is downloaded before it is opened.
	WhereTemp Where = "temp"
)

// Verdict is the outcome, a code for the UI's logic; Result.Message is the
// sentence for the person.
type Verdict string

const (
	// Enough: it fits with at least Config.LowAfter to spare.
	Enough Verdict = "enough"
	// Low: it fits, but less than Config.LowAfter would be left.
	Low Verdict = "low"
	// NotEnough: it does not fit.
	NotEnough Verdict = "not_enough"
	// Unknown: free space or the download's size could not be read, so
	// nothing can be said about whether it fits. Never a reason to block.
	Unknown Verdict = "unknown"
)

// Actions the UI can offer beside a message, as codes (the screen owns the
// words and the destination). P2-7 adds "keep_on_another_drive".
const ActionRemoveModels = "remove_models"

// Request is one download to check.
type Request struct {
	// Need is the download's size. Unknown (Known false) when the source
	// does not say; a catalogue file size is Estimated for what Ollama will
	// actually fetch, an installer's length from its host is Measured.
	Need      figure.Bytes
	NeedKnown bool

	// Dir is the folder it will be written to; "" when that is not known.
	Dir string
	// DirReported is true when the runtime itself said Dir is its models
	// folder (backend.ModelsFolder.Known), false when it is only where the
	// runtime will put models: the words change from "saves" to "will save".
	DirReported bool

	Where Where
}

// Result is the check, for the UI (GET .../check) and for the refusal.
type Result struct {
	Verdict Verdict `json:"verdict"`
	// Message is one or two sentences for the person, with both numbers.
	Message string `json:"message"`

	// Need is the download's size with its source; absent when unknown.
	Need *figure.Bytes `json:"need,omitempty"`
	// FreeBytes is the space free now on that drive: read from the OS.
	FreeBytes uint64 `json:"free_bytes" source:"n/a"`
	FreeKnown bool   `json:"free_known"`
	// Left is what remains after the download: free space minus Need, so it
	// inherits Need's source. Absent unless both are known and it fits.
	Left *figure.Bytes `json:"left,omitempty"`

	// Volume is the drive's name where the OS has such names ("C:").
	Volume string `json:"volume,omitempty"`
	// Folder is the folder the check was made for.
	Folder string `json:"folder,omitempty"`
	Where  Where  `json:"where"`
	// Actions are what the UI may offer beside the message.
	Actions []string `json:"actions"`
}

// Refuses is true when the download must not start.
func (r Result) Refuses() bool { return r.Verdict == NotEnough }

// Checker reads the disk and judges. Free defaults to hardware.FreeSpaceOn;
// tests replace it.
type Checker struct {
	Config Config
	Free   func(ctx context.Context, dir string) (hardware.FreeSpace, error)
}

// New is a Checker with the chosen constant that reads the real disk.
func New() Checker { return Checker{Config: DefaultConfig()} }

// Check reads the free space on req.Dir's drive now and compares it with
// req.Need.
func (c Checker) Check(ctx context.Context, req Request) Result {
	res := Result{Verdict: Unknown, Where: req.Where, Folder: req.Dir, Actions: []string{}}
	if req.NeedKnown {
		need := req.Need
		res.Need = &need
	}
	var space hardware.FreeSpace
	if req.Dir != "" {
		read := c.Free
		if read == nil {
			read = hardware.FreeSpaceOn
		}
		if fs, err := read(ctx, req.Dir); err == nil {
			space = fs
			res.FreeBytes, res.FreeKnown, res.Volume = fs.Bytes, true, fs.Volume
		}
	}
	w := words{req: req, volume: res.Volume}

	switch {
	case !res.FreeKnown && !req.NeedKnown:
		res.Message = "The advisor could not read how much space is left on " + w.drive() + ", and does not know the size of this download, so it cannot check that it fits."
		return res
	case !res.FreeKnown:
		res.Message = "The advisor could not read how much space is left on " + w.drive() + ", so it cannot check that this fits. The download is " + w.need() + "."
		return res
	case !req.NeedKnown:
		res.Message = "The size of this download is not known, so the advisor cannot check that it fits. " + capitalise(w.drive()) + " has " + size(space.Bytes) + " free."
		return res
	}

	free, need := space.Bytes, req.Need.Value
	if need > free {
		res.Verdict = NotEnough
		res.Message = "This needs " + w.need() + " and " + w.drive() + " has " + size(free) + " free. " + w.advice()
		res.Actions = w.actions()
		return res
	}
	left := figure.Bytes{Value: free - need, Source: req.Need.Source}
	res.Left = &left
	if left.Value < c.Config.LowAfter {
		res.Verdict = Low
		res.Message = w.leftSentence(left.Value, "After this download, ") +
			" Keeping about " + size(c.Config.LowAfter) + " free leaves room for system updates and another small model."
		res.Actions = w.actions()
		return res
	}
	res.Verdict = Enough
	res.Message = w.leftSentence(left.Value, "There is room for this: ")
	return res
}

// words builds the phrases a message is made of.
type words struct {
	req    Request
	volume string
}

// drive is where the download goes, in words: "the drive Ollama saves
// models to (C:)".
func (w words) drive() string {
	var s string
	switch w.req.Where {
	case WhereTemp:
		s = "the drive this computer uses for temporary files"
	default:
		if w.req.DirReported {
			s = "the drive Ollama saves models to"
		} else {
			s = "the drive Ollama will save models to"
		}
	}
	if w.volume != "" {
		s += " (" + w.volume + ")"
	}
	return s
}

// need is the size, "about" it when it is an estimate (product rule 4).
func (w words) need() string {
	if w.req.Need.Source == figure.Estimated {
		return "about " + size(w.req.Need.Value)
	}
	return size(w.req.Need.Value)
}

func (w words) advice() string {
	if w.req.Where == WhereTemp {
		return "Free up space on that drive, then try again."
	}
	return "Remove a model you no longer use, or free up space."
}

func (w words) actions() []string {
	if w.req.Where == WhereTemp {
		return []string{}
	}
	return []string{ActionRemoveModels}
}

// leftSentence: "After this download, about 7 GB will be left on C:."
func (w words) leftSentence(left uint64, lead string) string {
	where := w.volume
	if where == "" {
		where = w.drive()
	}
	if left < 1e8 {
		return lead + "almost nothing will be left on " + where + "."
	}
	return lead + "about " + size(left) + " will be left on " + where + "."
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// size writes a size the way download pages do, in decimal units: "9.1 GB",
// "6.4 GB", "850 MB"; one decimal below 100 GB, never a trailing ".0".
func size(b uint64) string {
	gb := float64(b) / 1e9
	switch {
	case gb < 0.95:
		return strconv.Itoa(int(math.Max(1, math.Round(gb*1000)))) + " MB"
	case gb < 100:
		return strings.TrimSuffix(strconv.FormatFloat(gb, 'f', 1, 64), ".0") + " GB"
	}
	return strconv.Itoa(int(math.Round(gb))) + " GB"
}
