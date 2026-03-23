// Package go2fs creates ext4 filesystem images from tarballs or directories.
// It uses libext2fs transpiled to pure Go via ccgo — no CGO required.
package go2fs

import (
	"io"

	"github.com/semistrict/go2fs/internal/e2fs"
)

// FS is a handle to an ext4 filesystem image being constructed.
type FS = e2fs.FS

// Create creates a new ext4 filesystem image at path with the given size in bytes.
// The file is created and truncated automatically if it doesn't exist.
var Create = e2fs.Create

// BuildExt4FromTar creates an ext4 filesystem image at imgPath populated from
// the tar archive at tarPath. If tarPath is "-", reads from stdin.
var BuildExt4FromTar = e2fs.BuildExt4FromTar

// BuildExt4FromTarReader creates an ext4 filesystem image at imgPath populated
// from the given reader (tar, optionally gzipped).
var BuildExt4FromTarReader = e2fs.BuildExt4FromTarReader

// BuildExt4FromDir creates an ext4 filesystem image at imgPath populated from
// the host directory at dirPath.
var BuildExt4FromDir = e2fs.BuildExt4FromDir

// Ensure io import is used (for godoc linking).
var _ io.Reader
