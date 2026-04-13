package e2fs

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"modernc.org/libc"
)

// e2err converts an ext2fs errcode_t to a Go error.
func e2err(tls *libc.TLS, code errcode_t, context string) error {
	if code == 0 {
		return nil
	}
	msg := libc.GoString(error_message(tls, code))
	return fmt.Errorf("%s: %s (errcode %d)", context, msg, code)
}

// Ino is an inode number.
type Ino uint32

// FS is a handle to an ext4 filesystem image being constructed.
type FS struct {
	tls *libc.TLS
	h   uintptr // e2fs_t (opaque C handle)
}

// Create creates a new ext4 filesystem image at path with the given size.
func Create(path string, sizeBytes uint64) (*FS, error) {
	// Ensure the image file exists (e2fs_create expects it).
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("create image: %w", err)
		}
		if err := f.Truncate(int64(sizeBytes)); err != nil {
			return nil, errors.Join(fmt.Errorf("truncate image: %w", err), f.Close())
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("close image: %w", err)
		}
	}

	tls := libc.NewTLS()

	cpath, err := libc.CString(path)
	if err != nil {
		tls.Close()
		return nil, fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(tls, cpath)

	var h uintptr
	var pinner runtime.Pinner
	pinner.Pin(&h)
	defer pinner.Unpin()

	ret := e2fs_create(tls, cpath, sizeBytes, uintptr(unsafe.Pointer(&h)))
	if ret != 0 {
		tls.Close()
		return nil, e2err(tls, ret, "e2fs_create")
	}
	if h == 0 {
		tls.Close()
		return nil, fmt.Errorf("e2fs_create returned nil handle")
	}

	return &FS{tls: tls, h: h}, nil
}

// Open opens an existing ext4 filesystem image for read-write access.
func Open(path string) (*FS, error) {
	tls := libc.NewTLS()

	cpath, err := libc.CString(path)
	if err != nil {
		tls.Close()
		return nil, fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(tls, cpath)

	var h uintptr
	var pinner runtime.Pinner
	pinner.Pin(&h)
	defer pinner.Unpin()

	ret := e2fs_open(tls, cpath, uintptr(unsafe.Pointer(&h)))
	if ret != 0 {
		tls.Close()
		return nil, e2err(tls, ret, "e2fs_open")
	}
	if h == 0 {
		tls.Close()
		return nil, fmt.Errorf("e2fs_open returned nil handle")
	}

	return &FS{tls: tls, h: h}, nil
}

// Close flushes and closes the filesystem.
func (fs *FS) Close() error {
	if fs.h == 0 {
		return nil
	}
	ret := e2fs_close(fs.tls, fs.h)
	fs.h = 0
	fs.tls.Close()
	if ret != 0 {
		return fmt.Errorf("e2fs_close: errcode %d", ret)
	}
	return nil
}

func (fs *FS) checkHandle() {
	if fs.h == 0 {
		panic("go2fs: use of closed or nil FS handle")
	}
}

// Mkdir creates a directory at path with the given metadata.
func (fs *FS) Mkdir(path string, mode uint32, uid, gid uint32, mtime int64) error {
	fs.checkHandle()
	cpath, err := libc.CString(path)
	if err != nil {
		return fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(fs.tls, cpath)

	ret := e2fs_mkdir(fs.tls, fs.h, cpath, mode, uid, gid, mtime)
	if ret != 0 {
		return e2err(fs.tls, ret, fmt.Sprintf("mkdir %q", path))
	}
	return nil
}

// WriteFile creates a regular file at path with the given data and metadata.
func (fs *FS) WriteFile(path string, mode uint32, uid, gid uint32, mtime int64, data []byte) error {
	cpath, err := libc.CString(path)
	if err != nil {
		return fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(fs.tls, cpath)

	var dataPtr uintptr
	if len(data) > 0 {
		dataPtr = uintptr(unsafe.Pointer(&data[0]))
	}

	ret := e2fs_write_file(fs.tls, fs.h, cpath, mode, uid, gid, mtime, dataPtr, uint64(len(data)))
	if ret != 0 {
		return e2err(fs.tls, ret, fmt.Sprintf("write_file %q", path))
	}
	return nil
}

// Symlink creates a symbolic link at path pointing to target.
func (fs *FS) Symlink(path, target string, uid, gid uint32, mtime int64) error {
	cpath, err := libc.CString(path)
	if err != nil {
		return fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(fs.tls, cpath)

	ctarget, err := libc.CString(target)
	if err != nil {
		return fmt.Errorf("alloc target: %w", err)
	}
	defer libc.Xfree(fs.tls, ctarget)

	ret := e2fs_symlink(fs.tls, fs.h, cpath, ctarget, uid, gid, mtime)
	if ret != 0 {
		return e2err(fs.tls, ret, fmt.Sprintf("symlink %q -> %q", path, target))
	}
	return nil
}

// Hardlink creates a hard link at path pointing to target.
func (fs *FS) Hardlink(path, target string) error {
	cpath, err := libc.CString(path)
	if err != nil {
		return fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(fs.tls, cpath)

	ctarget, err := libc.CString(target)
	if err != nil {
		return fmt.Errorf("alloc target: %w", err)
	}
	defer libc.Xfree(fs.tls, ctarget)

	ret := e2fs_hardlink(fs.tls, fs.h, cpath, ctarget)
	if ret != 0 {
		return e2err(fs.tls, ret, fmt.Sprintf("hardlink %q -> %q", path, target))
	}
	return nil
}

// Mknod creates a device node, FIFO, or socket at path.
func (fs *FS) Mknod(path string, mode uint32, uid, gid uint32, mtime int64, major, minor uint32) error {
	cpath, err := libc.CString(path)
	if err != nil {
		return fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(fs.tls, cpath)

	ret := e2fs_mknod(fs.tls, fs.h, cpath, mode, uid, gid, mtime, major, minor)
	if ret != 0 {
		return e2err(fs.tls, ret, fmt.Sprintf("mknod %q", path))
	}
	return nil
}
