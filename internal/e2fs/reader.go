package e2fs

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"runtime"
	"slices"
	"strings"
	"time"
	"unsafe"

	"modernc.org/libc"
)

// ReadFS is a read-only fs.FS backed by an ext4 filesystem image.
type ReadFS struct {
	tls *libc.TLS
	fs  ext2_filsys
}

// Interface assertions.
var (
	_ fs.FS         = (*ReadFS)(nil)
	_ fs.StatFS     = (*ReadFS)(nil)
	_ fs.ReadDirFS  = (*ReadFS)(nil)
	_ fs.ReadFileFS = (*ReadFS)(nil)
)

// OpenFS opens an existing ext4 filesystem image for reading.
func OpenFS(path string) (*ReadFS, error) {
	tls := libc.NewTLS()

	cpath, err := libc.CString(path)
	if err != nil {
		tls.Close()
		return nil, fmt.Errorf("alloc path: %w", err)
	}
	defer libc.Xfree(tls, cpath)

	var fsys ext2_filsys
	var pinner runtime.Pinner
	pinner.Pin(&fsys)
	defer pinner.Unpin()

	ret := ext2fs_open(tls, cpath, 0, 0, 0, unix_io_manager, uintptr(unsafe.Pointer(&fsys)))
	if ret != 0 {
		tls.Close()
		return nil, e2err(tls, ret, "ext2fs_open")
	}
	if fsys == 0 {
		tls.Close()
		return nil, fmt.Errorf("ext2fs_open returned nil handle")
	}

	// Read bitmaps so inode/block allocation info is available.
	ret = ext2fs_read_inode_bitmap(tls, fsys)
	if ret != 0 {
		ext2fs_close(tls, fsys)
		tls.Close()
		return nil, e2err(tls, ret, "ext2fs_read_inode_bitmap")
	}
	ret = ext2fs_read_block_bitmap(tls, fsys)
	if ret != 0 {
		ext2fs_close(tls, fsys)
		tls.Close()
		return nil, e2err(tls, ret, "ext2fs_read_block_bitmap")
	}

	return &ReadFS{tls: tls, fs: fsys}, nil
}

// Close closes the filesystem.
func (r *ReadFS) Close() error {
	if r.fs == 0 {
		return nil
	}
	ret := ext2fs_close(r.tls, r.fs)
	r.fs = 0
	r.tls.Close()
	if ret != 0 {
		return fmt.Errorf("ext2fs_close: errcode %d", ret)
	}
	return nil
}

// resolve converts an fs.FS path (no leading slash, "." for root) to an inode.
func (r *ReadFS) resolve(name string) (ext2_ino_t, error) {
	if !fs.ValidPath(name) {
		return 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return ext2_ino_t(EXT2_ROOT_INO), nil
	}

	cname, err := libc.CString("/" + name)
	if err != nil {
		return 0, err
	}
	defer libc.Xfree(r.tls, cname)

	var ino ext2_ino_t
	var pinner runtime.Pinner
	pinner.Pin(&ino)
	defer pinner.Unpin()

	ret := ext2fs_namei(r.tls, r.fs, ext2_ino_t(EXT2_ROOT_INO), ext2_ino_t(EXT2_ROOT_INO), cname, uintptr(unsafe.Pointer(&ino)))
	if ret != 0 {
		return 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return ino, nil
}

// readInode reads the inode struct for the given inode number.
func (r *ReadFS) readInode(ino ext2_ino_t) (ext2_inode, error) {
	var inode ext2_inode
	var pinner runtime.Pinner
	pinner.Pin(&inode)
	defer pinner.Unpin()

	ret := ext2fs_read_inode(r.tls, r.fs, ino, uintptr(unsafe.Pointer(&inode)))
	if ret != 0 {
		return inode, e2err(r.tls, ret, "ext2fs_read_inode")
	}
	return inode, nil
}

// Open implements fs.FS.
func (r *ReadFS) Open(name string) (fs.File, error) {
	ino, err := r.resolve(name)
	if err != nil {
		return nil, err
	}

	inode, err := r.readInode(ino)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	info := inodeToFileInfo(path.Base(name), ino, inode)

	return &ext4File{
		rfs:  r,
		ino:  ino,
		info: info,
		name: name,
	}, nil
}

// Stat implements fs.StatFS.
func (r *ReadFS) Stat(name string) (fs.FileInfo, error) {
	ino, err := r.resolve(name)
	if err != nil {
		return nil, err
	}

	inode, err := r.readInode(ino)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	return inodeToFileInfo(path.Base(name), ino, inode), nil
}

// ReadDir implements fs.ReadDirFS.
func (r *ReadFS) ReadDir(name string) ([]fs.DirEntry, error) {
	ino, err := r.resolve(name)
	if err != nil {
		return nil, err
	}

	return r.readDirEntries(ino, name, -1)
}

// ReadFile implements fs.ReadFileFS.
func (r *ReadFS) ReadFile(name string) ([]byte, error) {
	ino, err := r.resolve(name)
	if err != nil {
		return nil, err
	}

	inode, err := r.readInode(ino)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}

	size := inodeSize(inode)

	var file ext2_file_t
	var pinner runtime.Pinner
	pinner.Pin(&file)
	defer pinner.Unpin()

	ret := ext2fs_file_open(r.tls, r.fs, ino, 0, uintptr(unsafe.Pointer(&file)))
	if ret != 0 {
		return nil, &fs.PathError{Op: "read", Path: name, Err: e2err(r.tls, ret, "ext2fs_file_open")}
	}
	defer ext2fs_file_close(r.tls, file)

	buf := make([]byte, size)
	if size == 0 {
		return buf, nil
	}

	var got uint32
	pinner.Pin(&got)
	ret = ext2fs_file_read(r.tls, file, uintptr(unsafe.Pointer(&buf[0])), uint32(size), uintptr(unsafe.Pointer(&got)))
	if ret != 0 {
		return nil, &fs.PathError{Op: "read", Path: name, Err: e2err(r.tls, ret, "ext2fs_file_read")}
	}

	return buf[:got], nil
}

// readDirEntries uses ext2fs_dir_iterate2 to list directory entries.
func (r *ReadFS) readDirEntries(dirIno ext2_ino_t, dirName string, n int) ([]fs.DirEntry, error) {
	// Verify it's actually a directory.
	inode, err := r.readInode(dirIno)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: dirName, Err: err}
	}
	if uint32(inode.Fi_mode)&0xF000 != 0x4000 {
		return nil, &fs.PathError{Op: "readdir", Path: dirName, Err: fmt.Errorf("not a directory (mode=0%o)", inode.Fi_mode)}
	}

	type dirCollector struct {
		rfs     *ReadFS
		entries []fs.DirEntry
	}

	collector := &dirCollector{rfs: r}

	// The callback is called by the transpiled C code.
	callback := func(tls *libc.TLS, dir ext2_ino_t, entryType int32, dirent uintptr, offset int32, blocksize int32, buf uintptr, privData uintptr) int32 {
		de := (*ext2_dir_entry_2)(unsafe.Pointer(dirent)) //nolint:govet // dirent is a C pointer from ccgo
		if de.Finode == 0 {
			return 0 // skip empty entries
		}

		nameLen := int(de.Fname_len)
		nameBytes := unsafe.Slice((*byte)(unsafe.Pointer(&de.Fname[0])), nameLen)
		entryName := string(nameBytes)

		// Skip . and ..
		if entryName == "." || entryName == ".." {
			return 0
		}

		inode, err := collector.rfs.readInode(ext2_ino_t(de.Finode))
		if err != nil {
			return 0 // skip unreadable entries
		}

		info := inodeToFileInfo(entryName, ext2_ino_t(de.Finode), inode)
		collector.entries = append(collector.entries, &ext4DirEntry{info: info})

		return 0
	}

	ret := ext2fs_dir_iterate2(r.tls, r.fs, dirIno, 0, 0, __ccgo_fp(callback), 0)
	if ret != 0 {
		return nil, &fs.PathError{Op: "readdir", Path: dirName, Err: e2err(r.tls, ret, "ext2fs_dir_iterate2")}
	}

	entries := collector.entries
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	if n > 0 && n < len(entries) {
		entries = entries[:n]
	}

	return entries, nil
}

// ext4File implements fs.File and fs.ReadDirFile.
type ext4File struct {
	rfs    *ReadFS
	ino    ext2_ino_t
	info   *ext4FileInfo
	name   string
	file   ext2_file_t // lazily opened for reads
	opened bool
	dirPos int // position for ReadDir iteration
}

func (f *ext4File) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *ext4File) Read(buf []byte) (int, error) {
	if f.info.IsDir() {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fmt.Errorf("is a directory")}
	}

	if len(buf) == 0 {
		return 0, nil
	}

	if !f.opened {
		var pinner runtime.Pinner
		pinner.Pin(&f.file)
		defer pinner.Unpin()

		ret := ext2fs_file_open(f.rfs.tls, f.rfs.fs, f.ino, 0, uintptr(unsafe.Pointer(&f.file)))
		if ret != 0 {
			return 0, &fs.PathError{Op: "read", Path: f.name, Err: e2err(f.rfs.tls, ret, "ext2fs_file_open")}
		}
		f.opened = true
	}

	var got uint32
	var pinner runtime.Pinner
	pinner.Pin(&got)
	defer pinner.Unpin()

	ret := ext2fs_file_read(f.rfs.tls, f.file, uintptr(unsafe.Pointer(&buf[0])), uint32(len(buf)), uintptr(unsafe.Pointer(&got)))
	if ret != 0 {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: e2err(f.rfs.tls, ret, "ext2fs_file_read")}
	}

	if got == 0 {
		return 0, io.EOF
	}
	return int(got), nil
}

func (f *ext4File) Close() error {
	if f.opened && f.file != 0 {
		ext2fs_file_close(f.rfs.tls, f.file)
		f.file = 0
		f.opened = false
	}
	return nil
}

// ReadDir implements fs.ReadDirFile.
func (f *ext4File) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.info.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: fmt.Errorf("not a directory")}
	}

	// Fetch all entries (we don't have seek support for dir iteration).
	all, err := f.rfs.readDirEntries(f.ino, f.name, -1)
	if err != nil {
		return nil, err
	}

	// Apply position for incremental reads.
	remaining := all[f.dirPos:]

	if n <= 0 {
		f.dirPos = len(all)
		if len(remaining) == 0 {
			return nil, nil
		}
		return remaining, nil
	}

	if len(remaining) == 0 {
		return nil, io.EOF
	}

	if n > len(remaining) {
		n = len(remaining)
	}
	f.dirPos += n
	return remaining[:n], nil
}

// ext4FileInfo implements fs.FileInfo.
type ext4FileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	ino     ext2_ino_t
}

func (fi *ext4FileInfo) Name() string               { return fi.name }
func (fi *ext4FileInfo) Size() int64                { return fi.size }
func (fi *ext4FileInfo) Mode() fs.FileMode          { return fi.mode }
func (fi *ext4FileInfo) ModTime() time.Time         { return fi.modTime }
func (fi *ext4FileInfo) IsDir() bool                { return fi.mode.IsDir() }
func (fi *ext4FileInfo) Sys() any                   { return nil }
func (fi *ext4FileInfo) Type() fs.FileMode          { return fi.mode.Type() }
func (fi *ext4FileInfo) Info() (fs.FileInfo, error) { return fi, nil }

// ext4DirEntry implements fs.DirEntry.
type ext4DirEntry struct {
	info *ext4FileInfo
}

func (de *ext4DirEntry) Name() string               { return de.info.Name() }
func (de *ext4DirEntry) IsDir() bool                { return de.info.IsDir() }
func (de *ext4DirEntry) Type() fs.FileMode          { return de.info.Type() }
func (de *ext4DirEntry) Info() (fs.FileInfo, error) { return de.info, nil }

// inodeToFileInfo converts an ext2_inode to fs.FileInfo.
func inodeToFileInfo(name string, ino ext2_ino_t, inode ext2_inode) *ext4FileInfo {
	mode := linuxModeToGoMode(uint32(inode.Fi_mode))

	// For "." name, use the directory name convention.
	if name == "" || name == "/" {
		name = "."
	}

	return &ext4FileInfo{
		name:    name,
		size:    inodeSize(inode),
		mode:    mode,
		modTime: time.Unix(int64(inode.Fi_mtime), 0),
		ino:     ino,
	}
}

// inodeSize returns the 64-bit file size from an ext2_inode.
func inodeSize(inode ext2_inode) int64 {
	return int64(inode.Fi_size) | (int64(inode.Fi_size_high) << 32)
}

// linuxModeToGoMode converts a Linux mode_t to Go's fs.FileMode.
func linuxModeToGoMode(m uint32) fs.FileMode {
	mode := fs.FileMode(m & 0777)

	// Special permission bits.
	if m&0o4000 != 0 {
		mode |= fs.ModeSetuid
	}
	if m&0o2000 != 0 {
		mode |= fs.ModeSetgid
	}
	if m&0o1000 != 0 {
		mode |= fs.ModeSticky
	}

	// File type.
	switch m & 0xF000 {
	case 0x4000: // S_IFDIR
		mode |= fs.ModeDir
	case 0xA000: // S_IFLNK
		mode |= fs.ModeSymlink
	case 0x2000: // S_IFCHR
		mode |= fs.ModeCharDevice | fs.ModeDevice
	case 0x6000: // S_IFBLK
		mode |= fs.ModeDevice
	case 0x1000: // S_IFIFO
		mode |= fs.ModeNamedPipe
	case 0xC000: // S_IFSOCK
		mode |= fs.ModeSocket
	}

	return mode
}

// ReadLink reads the target of a symlink. Not part of fs.FS but useful.
func (r *ReadFS) ReadLink(name string) (string, error) {
	ino, err := r.resolve(name)
	if err != nil {
		return "", err
	}

	inode, err := r.readInode(ino)
	if err != nil {
		return "", err
	}

	size := inodeSize(inode)

	// Short symlinks are stored inline in i_block.
	if size < 60 && inode.Fi_blocks == 0 {
		target := unsafe.Slice((*byte)(unsafe.Pointer(&inode.Fi_block[0])), size)
		// Find null terminator.
		s := string(target)
		if idx := strings.IndexByte(s, 0); idx >= 0 {
			s = s[:idx]
		}
		return s, nil
	}

	// Long symlinks stored in a data block.
	data, err := r.ReadFile(name)
	if err != nil {
		return "", err
	}
	s := string(data)
	if idx := strings.IndexByte(s, 0); idx >= 0 {
		s = s[:idx]
	}
	return s, nil
}
