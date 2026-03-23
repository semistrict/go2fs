package e2fs

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
)

// BuildExt4FromTar creates an ext4 filesystem image at imgPath populated from
// the tar archive at tarPath. The image file is created with the given size.
// If tarPath is "-", reads from stdin.
func BuildExt4FromTar(imgPath string, tarPath string, sizeBytes uint64) error {
	// Create and truncate the image file.
	img, err := os.Create(imgPath)
	if err != nil {
		return fmt.Errorf("create image: %w", err)
	}
	if err := img.Truncate(int64(sizeBytes)); err != nil {
		return errors.Join(fmt.Errorf("truncate image: %w", err), img.Close())
	}
	if err := img.Close(); err != nil {
		return fmt.Errorf("close image: %w", err)
	}

	// Initialize ext4 filesystem.
	fs, err := Create(imgPath, sizeBytes)
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	// Open tar (auto-detect gzip).
	var r io.Reader
	if tarPath == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(tarPath)
		if err != nil {
			return fmt.Errorf("open tar: %w", err)
		}
		defer func() { _ = f.Close() }()
		r = f
	}

	r, err = maybeGunzip(r)
	if err != nil {
		return err
	}

	return populateFromTar(fs, tar.NewReader(r))
}

// BuildExt4FromTarReader creates an ext4 filesystem image at imgPath populated
// from the given tar reader. The image file is created with the given size.
func BuildExt4FromTarReader(imgPath string, r io.Reader, sizeBytes uint64) error {
	img, err := os.Create(imgPath)
	if err != nil {
		return fmt.Errorf("create image: %w", err)
	}
	if err := img.Truncate(int64(sizeBytes)); err != nil {
		return errors.Join(fmt.Errorf("truncate image: %w", err), img.Close())
	}
	if err := img.Close(); err != nil {
		return fmt.Errorf("close image: %w", err)
	}

	fs, err := Create(imgPath, sizeBytes)
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	r, err = maybeGunzip(r)
	if err != nil {
		return err
	}

	return populateFromTar(fs, tar.NewReader(r))
}

// maybeGunzip peeks at the first two bytes to detect gzip magic (\x1f\x8b)
// and wraps the reader in a gzip.Reader if found.
func maybeGunzip(r io.Reader) (io.Reader, error) {
	br := bufio.NewReader(r)
	peek, err := br.Peek(2)
	if err != nil {
		// Too short to be gzip; return as-is (might be empty tar).
		return br, nil
	}
	if peek[0] == 0x1f && peek[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		return gz, nil
	}
	return br, nil
}

func populateFromTar(fs *FS, tr *tar.Reader) error {
	var dirs, files, symlinks, hardlinks, devs, skipped int

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Clean the path: strip leading "/" and "./" prefixes.
		name := path.Clean(hdr.Name)
		name = strings.TrimPrefix(name, "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" || name == "." {
			continue
		}

		uid := uint32(hdr.Uid)
		gid := uint32(hdr.Gid)
		mtime := hdr.ModTime.Unix()
		mode := uint32(hdr.Mode & 07777)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := fs.Mkdir(name, mode, uid, gid, mtime); err != nil {
				return fmt.Errorf("mkdir %q: %w", name, err)
			}
			dirs++

		case tar.TypeReg:
			data, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("read %q: %w", name, err)
			}
			if err := fs.WriteFile(name, mode, uid, gid, mtime, data); err != nil {
				return fmt.Errorf("write %q: %w", name, err)
			}
			files++

		case tar.TypeSymlink:
			if err := fs.Symlink(name, hdr.Linkname, uid, gid, mtime); err != nil {
				return fmt.Errorf("symlink %q -> %q: %w", name, hdr.Linkname, err)
			}
			symlinks++

		case tar.TypeLink:
			target := path.Clean(hdr.Linkname)
			target = strings.TrimPrefix(target, "/")
			target = strings.TrimPrefix(target, "./")
			if err := fs.Hardlink(name, target); err != nil {
				return fmt.Errorf("hardlink %q -> %q: %w", name, hdr.Linkname, err)
			}
			hardlinks++

		case tar.TypeChar:
			if err := fs.Mknod(name, 0020000|mode, uid, gid, mtime,
				uint32(hdr.Devmajor), uint32(hdr.Devminor)); err != nil {
				return fmt.Errorf("mknod char %q: %w", name, err)
			}
			devs++

		case tar.TypeBlock:
			if err := fs.Mknod(name, 0060000|mode, uid, gid, mtime,
				uint32(hdr.Devmajor), uint32(hdr.Devminor)); err != nil {
				return fmt.Errorf("mknod block %q: %w", name, err)
			}
			devs++

		case tar.TypeFifo:
			if err := fs.Mknod(name, 0010000|mode, uid, gid, mtime, 0, 0); err != nil {
				return fmt.Errorf("mknod fifo %q: %w", name, err)
			}
			devs++

		default:
			slog.Warn("e2fs: skipping tar entry", "name", name, "type", hdr.Typeflag)
			skipped++
		}
	}

	slog.Info("e2fs: tar import complete",
		"dirs", dirs, "files", files, "symlinks", symlinks,
		"hardlinks", hardlinks, "devs", devs, "skipped", skipped)
	return nil
}
