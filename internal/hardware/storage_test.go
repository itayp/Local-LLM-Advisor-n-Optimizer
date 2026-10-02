package hardware

import (
	"context"
	"testing"
	"testing/fstest"
)

// freeSpaceOn reads the nearest existing parent's volume, on each OS's path
// syntax, and reports the drive's name where the OS has one.
func TestFreeSpaceOnReadsTheNearestExistingParent(t *testing.T) {
	for _, c := range []struct {
		goos, dir, exists, volume string
	}{
		{"windows", `D:\Ollama models\deep`, `D:\`, "D:"},
		{"windows", `C:\Users\jo\.ollama\models`, `C:\Users\jo\.ollama\models`, "C:"},
		{"darwin", "/Users/jo/.ollama/models", "/Users/jo", ""},
		{"linux", "/mnt/data/ollama/models", "/mnt", ""},
		{"linux", "/usr/share/ollama/.ollama/models", "/usr/share/ollama/.ollama/models", ""},
	} {
		t.Run(c.goos+" "+c.dir, func(t *testing.T) {
			f := newFakeEnv(c.goos, "amd64")
			f.free = 123
			if c.goos == "linux" {
				f.fsys = fstest.MapFS{c.exists[1:] + "/x": &fstest.MapFile{}}
			} else {
				f.existing[c.exists] = true
			}
			got, err := freeSpaceOn(context.Background(), f, c.dir)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bytes != 123 || got.ReadOn != c.exists || got.Volume != c.volume || got.DirExists != (c.exists == c.dir) {
				t.Fatalf("got %+v", got)
			}
			if f.diskPath != c.exists {
				t.Errorf("the disk was read on %q, want %q", f.diskPath, c.exists)
			}
		})
	}
	for _, dir := range []string{"", Unknown, "  "} {
		if _, err := freeSpaceOn(context.Background(), newFakeEnv("linux", "amd64"), dir); err == nil {
			t.Errorf("folder %q: want an error", dir)
		}
	}
}

func TestVolumeName(t *testing.T) {
	for _, c := range []struct{ dir, goos, want string }{
		{`c:\Users\jo`, "windows", "C:"}, {`D:\x`, "windows", "D:"}, {`\\nas\share\x`, "windows", ""},
		{"/Users/jo", "darwin", ""}, {"C:/x", "linux", ""},
	} {
		if got := VolumeName(c.dir, c.goos); got != c.want {
			t.Errorf("VolumeName(%q, %q) = %q, want %q", c.dir, c.goos, got, c.want)
		}
	}
}
