# go2fs

Pure Go ext4 filesystem image creation — no CGO required.

Uses [e2fsprogs](https://github.com/tytso/e2fsprogs) libext2fs transpiled to Go
via [ccgo](https://pkg.go.dev/modernc.org/ccgo/v4), with
[modernc.org/libc](https://pkg.go.dev/modernc.org/libc) as the C runtime.

## Usage

```go
import "github.com/semistrict/go2fs"

// Create ext4 image from a tar (optionally gzipped)
err := go2fs.BuildExt4FromTarReader("rootfs.ext4", tarReader, 256*1024*1024)

// Or build from a host directory
err := go2fs.BuildExt4FromDir("rootfs.ext4", "/path/to/rootfs", 256*1024*1024)

// Or use the low-level API
fs, err := go2fs.Create("rootfs.ext4", 256*1024*1024)
fs.Mkdir("/etc", 0755, 0, 0, time.Now().Unix())
fs.WriteFile("/etc/hostname", 0644, 0, 0, time.Now().Unix(), []byte("myhost\n"))
fs.Symlink("/etc/localtime", "/usr/share/zoneinfo/UTC", 0, 0, time.Now().Unix())
fs.Close()
```

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
2. A thin [C wrapper](_e2fs_impl.c) provides the high-level API (create, mkdir, write, symlink, etc.)
3. A [Go wrapper](internal/e2fs/wrapper.go) exposes an idiomatic Go API
4. [Go shims](internal/e2fs/shims.go) fill in libc functions missing from modernc/libc on darwin/arm64

The generated code (~120k lines) lives in `internal/e2fs/e2fs.go`.

## Supported platforms

Currently tested on darwin/arm64 (macOS Apple Silicon). The generated code is
platform-specific due to ccgo's build constraint (`//go:build darwin && arm64`).

## License

[LGPL-2.0](LICENSE) — same as libext2fs and libe2p from e2fsprogs.
