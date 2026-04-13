package e2fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateAndClose(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("created file is empty")
	}
}

func TestBuildExt4FromTarReader(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Directory
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "mydir/",
		Mode:     0755,
		Uid:      1000,
		Gid:      1000,
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}

	// Regular file
	fileContent := []byte("hello world\n")
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "mydir/hello.txt",
		Mode:     0644,
		Size:     int64(len(fileContent)),
		Uid:      1000,
		Gid:      1000,
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(fileContent); err != nil {
		t.Fatal(err)
	}

	// Symlink
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "mydir/link",
		Linkname: "hello.txt",
		Mode:     0777,
		Uid:      1000,
		Gid:      1000,
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}

	// Nested directory
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "mydir/sub/deep/",
		Mode:     0700,
		Uid:      0,
		Gid:      0,
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}

	// File in nested dir
	nestedContent := []byte("nested file content")
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "mydir/sub/deep/nested.txt",
		Mode:     0600,
		Size:     int64(len(nestedContent)),
		Uid:      0,
		Gid:      0,
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(nestedContent); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "test.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 64*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader: %v", err)
	}

	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 64*1024*1024 {
		t.Fatalf("image size = %d, want %d", info.Size(), 64*1024*1024)
	}
}

func TestBuildExt4FromTarReader_Gzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("gzipped content\n")
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "gzfile.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		ModTime:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "test.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 64*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader gzip: %v", err)
	}

	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 64*1024*1024 {
		t.Fatalf("image size = %d, want %d", info.Size(), 64*1024*1024)
	}
}

func TestLargeFile(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 251)
	}

	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "bigfile.bin",
		Mode:     0644,
		Size:     int64(len(data)),
		ModTime:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "test.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 64*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader large file: %v", err)
	}
}

func TestHardlink(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	now := time.Now()

	content := []byte("original content")
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "original.txt",
		Mode:     0644,
		Size:     int64(len(content)),
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}

	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeLink,
		Name:     "hardlink.txt",
		Linkname: "original.txt",
		ModTime:  now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "test.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 64*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader hardlink: %v", err)
	}
}

func TestManyFiles(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	now := time.Now().Truncate(time.Second)

	for d := 0; d < 10; d++ {
		dir := fmt.Sprintf("dir%d/", d)
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir, Name: dir,
			Mode: 0755, ModTime: now,
		}); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 50; f++ {
			name := fmt.Sprintf("dir%d/file%d.txt", d, f)
			content := []byte(fmt.Sprintf("content of %s\n", name))
			if err := tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeReg, Name: name,
				Mode: 0644, Size: int64(len(content)), ModTime: now,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(content); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "many.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 128*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader many files: %v", err)
	}
}

func TestOpenExisting(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	// Create a filesystem and write a file.
	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.WriteFile("hello.txt", 0644, 0, 0, 0, []byte("hello")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-open and write another file.
	fs2, err := Open(imgPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := fs2.WriteFile("world.txt", 0644, 0, 0, 0, []byte("world")); err != nil {
		t.Fatalf("WriteFile after Open: %v", err)
	}
	if err := fs2.Close(); err != nil {
		t.Fatalf("Close after Open: %v", err)
	}

	// Verify both files exist via ReadFS.
	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()
	for _, name := range []string{"hello.txt", "world.txt"} {
		f, err := rfs.Open(name)
		if err != nil {
			t.Errorf("Open(%q): %v", name, err)
			continue
		}
		f.Close()
	}
}

func TestOverwriteFile(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.WriteFile("data.txt", 0644, 0, 0, 0, []byte("original")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Open and overwrite the same file.
	fs2, err := Open(imgPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := fs2.WriteFile("data.txt", 0644, 0, 0, 0, []byte("replaced")); err != nil {
		t.Fatalf("WriteFile overwrite: %v", err)
	}
	if err := fs2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify content was replaced.
	rfs, err := OpenFS(imgPath)
	if err != nil {
		t.Fatalf("OpenFS: %v", err)
	}
	defer rfs.Close()
	f, err := rfs.Open("data.txt")
	if err != nil {
		t.Fatalf("Open data.txt: %v", err)
	}
	got, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("content = %q, want %q", got, "replaced")
	}
}

func TestOverwriteSymlink(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Symlink("mylink", "/target1", 0, 0, 0); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	// Overwrite with different target.
	if err := fs.Symlink("mylink", "/target2", 0, 0, 0); err != nil {
		t.Fatalf("Symlink overwrite: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestManySymlinksExpandDir(t *testing.T) {
	// Create enough symlinks in a single directory to force directory expansion.
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Mkdir("certs", 0755, 0, 0, 0); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("certs/ca-cert-%03d.pem", i)
		target := fmt.Sprintf("/usr/share/ca-certificates/cert-%03d.crt", i)
		if err := fs.Symlink(name, target, 0, 0, 0); err != nil {
			t.Fatalf("Symlink %d: %v", i, err)
		}
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOverwriteHardlink(t *testing.T) {
	imgPath := filepath.Join(t.TempDir(), "test.ext4")

	fs, err := Create(imgPath, 64*1024*1024)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.WriteFile("src.txt", 0644, 0, 0, 0, []byte("data")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := fs.Hardlink("link.txt", "src.txt"); err != nil {
		t.Fatalf("Hardlink: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Open and overwrite the hardlink with a regular file.
	fs2, err := Open(imgPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := fs2.WriteFile("link.txt", 0644, 0, 0, 0, []byte("new")); err != nil {
		t.Fatalf("WriteFile overwrite hardlink: %v", err)
	}
	if err := fs2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestLargeSparseFile(t *testing.T) {
	data := make([]byte, 4*1024*1024)
	copy(data[:4096], bytes.Repeat([]byte("HEAD"), 1024))
	copy(data[len(data)-4096:], bytes.Repeat([]byte("TAIL"), 1024))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg, Name: "sparse.bin",
		Mode: 0644, Size: int64(len(data)), ModTime: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(tw, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(t.TempDir(), "sparse.ext4")
	if err := BuildExt4FromTarReader(imgPath, &buf, 128*1024*1024); err != nil {
		t.Fatalf("BuildExt4FromTarReader sparse: %v", err)
	}
}
