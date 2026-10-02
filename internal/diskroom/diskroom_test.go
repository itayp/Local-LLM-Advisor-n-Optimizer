package diskroom

import (
	"context"
	"errors"
	"strings"
	"testing"

	"advisor/internal/figure"
	"advisor/internal/hardware"
)

const gb = 1_000_000_000

// disk is a fake Free: every folder is on a volume with this much free, named
// the way the operating system names it.
func disk(goos string, free uint64) func(context.Context, string) (hardware.FreeSpace, error) {
	return func(_ context.Context, dir string) (hardware.FreeSpace, error) {
		return hardware.FreeSpace{Dir: dir, ReadOn: dir, Bytes: free, Volume: hardware.VolumeName(dir, goos)}, nil
	}
}

func unreadable(context.Context, string) (hardware.FreeSpace, error) {
	return hardware.FreeSpace{}, errors.New("no such volume")
}

// The folder Ollama keeps models in, on each operating system's path syntax.
var folders = []struct{ goos, dir string }{
	{"windows", `C:\Users\jo\.ollama\models`},
	{"darwin", "/Users/jo/.ollama/models"},
	{"linux", "/usr/share/ollama/.ollama/models"},
}

func pull(dir string, need uint64) Request {
	return Request{Need: figure.EstimatedBytes(need), NeedKnown: true, Dir: dir, DirReported: true, Where: WhereModels}
}

// Enough / just enough / not enough / unknown, on each OS path (P2-3).
func TestCheckVerdictsOnEachOperatingSystem(t *testing.T) {
	for _, f := range folders {
		t.Run(f.goos, func(t *testing.T) {
			check := func(free, need uint64) Result {
				c := Checker{Config: DefaultConfig(), Free: disk(f.goos, free)}
				return c.Check(context.Background(), pull(f.dir, need))
			}

			// Plenty: 40 GB free, 5 GB needed.
			r := check(40*gb, 5*gb)
			if r.Verdict != Enough || r.Refuses() || r.Left == nil || r.Left.Value != 35*gb {
				t.Fatalf("enough: %+v", r)
			}
			if r.Left.Source != figure.Estimated {
				t.Errorf("what is left after an estimated download is estimated, got %q", r.Left.Source)
			}

			// Just enough: exactly the margin is left → still fine; one byte
			// less → a warning.
			if r := check(15*gb, 5*gb); r.Verdict != Enough {
				t.Fatalf("exactly the margin left: %+v", r)
			}
			r = check(15*gb-1, 5*gb)
			if r.Verdict != Low || r.Refuses() {
				t.Fatalf("one byte under the margin: %+v", r)
			}
			// It fits to the byte: allowed, and said to leave almost nothing.
			r = check(5*gb, 5*gb)
			if r.Verdict != Low || !strings.Contains(r.Message, "almost nothing") {
				t.Fatalf("fits exactly: %+v", r)
			}

			// Not enough: one byte over.
			r = check(5*gb-1, 5*gb)
			if r.Verdict != NotEnough || !r.Refuses() || r.Left != nil {
				t.Fatalf("one byte over: %+v", r)
			}
		})
	}
}

// The refusal gives both numbers and what to do (the P2-3 example), and the
// warning says how much is left, on the drive's letter where it has one.
func TestMessagesGiveBothNumbers(t *testing.T) {
	win := Checker{Config: DefaultConfig(), Free: disk("windows", 6_400_000_000)}
	r := win.Check(context.Background(), Request{Need: figure.MeasuredBytes(9_100_000_000), NeedKnown: true, Dir: `C:\Users\jo\.ollama\models`, DirReported: true, Where: WhereModels})
	want := "This needs 9.1 GB and the drive Ollama saves models to (C:) has 6.4 GB free. Remove a model you no longer use, or free up space."
	if r.Message != want {
		t.Errorf("not enough:\n got %q\nwant %q", r.Message, want)
	}
	if len(r.Actions) != 1 || r.Actions[0] != ActionRemoveModels {
		t.Errorf("actions = %v", r.Actions)
	}

	// An estimate is "about"; a measurement is not (rule 4).
	r = win.Check(context.Background(), pull(`C:\Users\jo\.ollama\models`, 9_100_000_000))
	if !strings.HasPrefix(r.Message, "This needs about 9.1 GB and") {
		t.Errorf("estimated need: %q", r.Message)
	}

	low := Checker{Config: DefaultConfig(), Free: disk("windows", 14*gb)}
	r = low.Check(context.Background(), pull(`C:\Users\jo\.ollama\models`, 7*gb))
	if !strings.HasPrefix(r.Message, "After this download, about 7 GB will be left on C:.") || !strings.Contains(r.Message, "10 GB") {
		t.Errorf("low on windows: %q", r.Message)
	}
	if r.Volume != "C:" {
		t.Errorf("volume = %q", r.Volume)
	}

	mac := Checker{Config: DefaultConfig(), Free: disk("darwin", 14*gb)}
	r = mac.Check(context.Background(), pull("/Users/jo/.ollama/models", 7*gb))
	if !strings.HasPrefix(r.Message, "After this download, about 7 GB will be left on the drive Ollama saves models to.") {
		t.Errorf("low on macOS: %q", r.Message)
	}

	// A folder Ollama has not reported yet is where it *will* save.
	req := pull("/Users/jo/.ollama/models", 7*gb)
	req.DirReported = false
	if r = mac.Check(context.Background(), req); !strings.Contains(r.Message, "will save models to") {
		t.Errorf("default folder: %q", r.Message)
	}
}

// The installer goes to the temporary folder; Remove-a-model is no advice
// there.
func TestTheInstallerCheckNamesTheTemporaryFolder(t *testing.T) {
	c := Checker{Config: DefaultConfig(), Free: disk("windows", 500_000_000)}
	r := c.Check(context.Background(), Request{Need: figure.MeasuredBytes(1_800_000_000), NeedKnown: true, Dir: `C:\Users\jo\AppData\Local\Temp`, Where: WhereTemp})
	if r.Verdict != NotEnough || !strings.Contains(r.Message, "temporary files") || strings.Contains(r.Message, "Remove a model") {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Message, "1.8 GB") || !strings.Contains(r.Message, "500 MB") {
		t.Errorf("both numbers expected: %q", r.Message)
	}
	if len(r.Actions) != 0 {
		t.Errorf("actions = %v", r.Actions)
	}
}

// Never block on a value the advisor could not read.
func TestUnknownNeverRefuses(t *testing.T) {
	ctx := context.Background()
	// Free space unreadable.
	r := Checker{Config: DefaultConfig(), Free: unreadable}.Check(ctx, pull("/x", 5*gb))
	if r.Verdict != Unknown || r.Refuses() || r.FreeKnown || !strings.Contains(r.Message, "could not read how much space") || !strings.Contains(r.Message, "5 GB") {
		t.Fatalf("free unknown: %+v", r)
	}
	// No folder at all.
	r = Checker{Config: DefaultConfig(), Free: disk("linux", 1)}.Check(ctx, pull("", 5*gb))
	if r.Verdict != Unknown || r.Refuses() {
		t.Fatalf("no folder: %+v", r)
	}
	// Size unknown.
	r = Checker{Config: DefaultConfig(), Free: disk("linux", 6*gb)}.Check(ctx, Request{Dir: "/x", Where: WhereModels, DirReported: true})
	if r.Verdict != Unknown || r.Refuses() || r.Need != nil || !strings.Contains(r.Message, "6 GB free") {
		t.Fatalf("size unknown: %+v", r)
	}
	// Both unknown.
	r = Checker{Config: DefaultConfig(), Free: unreadable}.Check(ctx, Request{Dir: "/x", Where: WhereModels})
	if r.Verdict != Unknown || r.Refuses() {
		t.Fatalf("both unknown: %+v", r)
	}
}

// Free space is read at each call, not remembered.
func TestFreeSpaceIsReadFreshEachTime(t *testing.T) {
	free := uint64(40 * gb)
	reads := 0
	c := Checker{Config: DefaultConfig(), Free: func(_ context.Context, dir string) (hardware.FreeSpace, error) {
		reads++
		return hardware.FreeSpace{Dir: dir, Bytes: free}, nil
	}}
	if r := c.Check(context.Background(), pull("/x", 20*gb)); r.Verdict != Enough {
		t.Fatal(r)
	}
	free = 10 * gb // something else filled the drive
	if r := c.Check(context.Background(), pull("/x", 20*gb)); r.Verdict != NotEnough {
		t.Fatal(r)
	}
	if reads != 2 {
		t.Errorf("reads = %d", reads)
	}
}

func TestSize(t *testing.T) {
	for b, want := range map[uint64]string{
		850_000_000: "850 MB", 1_000_000: "1 MB", 1_000_000_000: "1 GB", 9_100_000_000: "9.1 GB",
		6_400_000_000: "6.4 GB", 12_000_000_000: "12 GB", 250_400_000_000: "250 GB",
	} {
		if got := size(b); got != want {
			t.Errorf("size(%d) = %q, want %q", b, got, want)
		}
	}
}

func TestTheMarginIsTheChosenTenGB(t *testing.T) {
	if DefaultConfig().LowAfter != 10*gb {
		t.Fatal("the margin is CHOSEN at 10 GB (config comment); change the comment with the number")
	}
}
