package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// D-68: the data folder and the database are this user's alone, and an
// install an older build made more open is tightened on the next start.
func TestTheDataFolderIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps %LOCALAPPDATA% to its user by ACL; file modes say little there")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "Advisor")
	t.Setenv("ADVISOR_DATA_DIR", dir) // the advisor's own folder, as an older build left it
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "advisor.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(context.Background(), "k", "v"); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for p, want := range map[string]os.FileMode{dir: 0o700, path: 0o600} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != want {
			t.Errorf("%s: mode %v, want %v", p, got, want)
		}
	}
	for _, p := range DatabaseFiles(path)[1:] {
		if fi, err := os.Stat(p); err == nil && fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s: mode %v lets other users read it", p, fi.Mode().Perm())
		}
	}
	// A fresh install is created private.
	fresh := filepath.Join(root, "fresh", "advisor.db")
	st2, err := Open(context.Background(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	st2.Close()
	if fi, _ := os.Stat(filepath.Dir(fresh)); fi.Mode().Perm() != 0o700 {
		t.Errorf("a new data folder: mode %v, want 0700", fi.Mode().Perm())
	}
	// A folder that is not the advisor's own keeps its mode; the database
	// in it is still private.
	shared := filepath.Join(root, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	st3, err := Open(context.Background(), filepath.Join(shared, "advisor.db"))
	if err != nil {
		t.Fatal(err)
	}
	st3.Close()
	if fi, _ := os.Stat(shared); fi.Mode().Perm() != 0o755 {
		t.Errorf("a folder the advisor did not make was changed to %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Join(shared, "advisor.db")); fi.Mode().Perm() != 0o600 {
		t.Errorf("the database in it: mode %v, want 0600", fi.Mode().Perm())
	}
}
