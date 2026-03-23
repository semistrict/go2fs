# go2fs

[![CI](https://github.com/semistrict/go2fs/actions/workflows/ci.yml/badge.svg)](https://github.com/semistrict/go2fs/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/semistrict/go2fs.svg)](https://pkg.go.dev/github.com/semistrict/go2fs)

Pure Go ext4 filesystem image creation and reading — no CGO required.

Uses [e2fsprogs](https://github.com/tytso/e2fsprogs) libext2fs transpiled to Go
via [ccgo](https://pkg.go.dev/modernc.org/ccgo/v4), with
[modernc.org/libc](https://pkg.go.dev/modernc.org/libc) as the C runtime.

## Usage

### Create ext4 images from tarballs

```go
import "github.com/semistrict/go2fs"

// From a tar reader (auto-detects gzip)
err := go2fs.BuildExt4FromTarReader("rootfs.ext4", tarReader, 256*1024*1024)

// From a tar file path
err := go2fs.BuildExt4FromTar("rootfs.ext4", "rootfs.tar.gz", 256*1024*1024)

// From a host directory
err := go2fs.BuildExt4FromDir("rootfs.ext4", "/path/to/rootfs", 256*1024*1024)
```

### Low-level write API

```go
fs, err := go2fs.Create("rootfs.ext4", 256*1024*1024)
fs.Mkdir("/etc", 0755, 0, 0, time.Now().Unix())
fs.WriteFile("/etc/hostname", 0644, 0, 0, time.Now().Unix(), []byte("myhost\n"))
fs.Symlink("/etc/localtime", "/usr/share/zoneinfo/UTC", 0, 0, time.Now().Unix())
fs.Close()
```

### Read ext4 images via io/fs.FS

```go
fsys, err := go2fs.OpenFS("rootfs.ext4")
defer fsys.Close()

// Use standard fs.FS interfaces
data, err := fs.ReadFile(fsys, "etc/hostname")

// Walk the filesystem
fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
    fmt.Println(path)
    return nil
})
```

`ReadFS` implements `fs.FS`, `fs.StatFS`, `fs.ReadDirFS`, and `fs.ReadFileFS`.
Passes [`testing/fstest.TestFS`](https://pkg.go.dev/testing/fstest#TestFS).

## Building

The transpiled Go code is generated from e2fsprogs C sources and checked in.
To regenerate after modifying the C sources:

```
git submodule update --init
make        # transpiles C → Go via ccgo
make test   # runs tests with race detector
```

Requires [ccgo v4](https://pkg.go.dev/modernc.org/ccgo/v4) (`go install modernc.org/ccgo/v4@latest`).

## How it works

1. A [Makefile](Makefile) compiles ~100 e2fsprogs C source files to Go using ccgo
2. A thin [C wrapper](_e2fs_impl.c) provides the high-level write API (create, mkdir, write, symlink, etc.)
3. A [Go wrapper](internal/e2fs/wrapper.go) exposes an idiomatic Go write API
4. A [Go reader](internal/e2fs/reader.go) implements `io/fs.FS` for reading images
5. [Go shims](internal/e2fs/shims.go) fill in libc functions missing from modernc/libc on darwin/arm64

The generated code (~120k lines) lives in `internal/e2fs/e2fs.go`.

## Supported platforms

Currently tested on darwin/arm64 (macOS Apple Silicon). The generated code is
platform-specific due to ccgo's build constraint (`//go:build darwin && arm64`).

## License

[LGPL-2.1](LICENSE) — compatible with libext2fs (LGPL-2.0-or-later) from e2fsprogs.
