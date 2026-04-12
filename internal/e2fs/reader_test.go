package e2fs

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestSymlink(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "symlink.ext4")
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	wfs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wfs.WriteFile("/target.txt", 0644, 0, 0, now.Unix(), []byte("hello"))
	// Both slash-prefixed and relative paths must create the symlink at the
	// right level. "bin" (no leading slash) is the OCI layer case that was
	// broken: dirname("bin") returned "bin" instead of "." causing the
	// symlink to land at bin/bin instead of bin.
	wfs.Symlink("/link", "target.txt", 0, 0, now.Unix())
	wfs.Symlink("link2", "target.txt", 0, 0, now.Unix())
	if err := wfs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()

	for _, name := range []string{"link", "link2"} {
		info, err := rfs.Stat(name)
		if err != nil {
			t.Fatalf("Stat %s: %v", name, err)
		}
		if info.Mode()&fs.ModeType != fs.ModeSymlink {
			t.Fatalf("%s mode = %v, want symlink", name, info.Mode())
		}
		target, err := rfs.ReadLink(name)
		if err != nil {
			t.Fatalf("ReadLink %s: %v", name, err)
		}
		if target != "target.txt" {
			t.Fatalf("ReadLink %s = %q, want %q", name, target, "target.txt")
		}
	}
}

func TestFSTest(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	// Create and populate an ext4 image.
	wfs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	wfs.Mkdir("/dir", 0755, 0, 0, now.Unix())
	wfs.Mkdir("/dir/sub", 0755, 0, 0, now.Unix())
	wfs.WriteFile("/hello.txt", 0644, 0, 0, now.Unix(), []byte("hello world\n"))
	wfs.WriteFile("/dir/file.txt", 0644, 0, 0, now.Unix(), []byte("in a directory\n"))
	wfs.WriteFile("/dir/sub/deep.txt", 0644, 0, 0, now.Unix(), []byte("deep\n"))
	wfs.WriteFile("/empty.txt", 0644, 0, 0, now.Unix(), nil)

	if err := wfs.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	// Open for reading.
	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()

	// Run the standard fs test suite.
	var testFS fs.FS = rfs
	err = fstest.TestFS(testFS,
		"lost+found",
		"hello.txt",
		"empty.txt",
		"dir",
		"dir/file.txt",
		"dir/sub",
		"dir/sub/deep.txt",
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadFile(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	wfs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	content := []byte("test content here")
	wfs.WriteFile("/test.txt", 0644, 0, 0, time.Now().Unix(), content)
	wfs.Close()

	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()

	got, err := rfs.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("ReadFile = %q, want %q", got, content)
	}
}

func TestReadDir(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	wfs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wfs.Mkdir("/mydir", 0755, 0, 0, time.Now().Unix())
	wfs.WriteFile("/mydir/a.txt", 0644, 0, 0, time.Now().Unix(), []byte("a"))
	wfs.WriteFile("/mydir/b.txt", 0644, 0, 0, time.Now().Unix(), []byte("b"))
	wfs.Close()

	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()

	entries, err := rfs.ReadDir("mydir")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	if !names["a.txt"] || !names["b.txt"] {
		t.Fatalf("ReadDir = %v, want a.txt and b.txt", names)
	}
}
