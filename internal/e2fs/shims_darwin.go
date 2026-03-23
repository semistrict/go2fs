//go:build darwin

package e2fs

import (
	"unsafe"

	"modernc.org/libc"
)

// Darwin-specific shims for libc functions not in modernc/libc.

func posix_memalign(tls *libc.TLS, memptr uintptr, alignment, size uint64) int32 {
	p := libc.Xmalloc(tls, size)
	if p == 0 {
		return -1
	}
	// memptr is a C pointer (from ccgo TLS stack), write through it.
	//nolint:govet // C pointer arithmetic via ccgo
	*(*uintptr)(unsafe.Pointer(memptr)) = p
	return 0
}

func getmntinfo(tls *libc.TLS, mntbufp uintptr, flags int32) int32 {
	// Return 0 entries: nothing is mounted. Correct for image files.
	//nolint:govet // C pointer arithmetic via ccgo
	*(*uintptr)(unsafe.Pointer(mntbufp)) = 0
	return 0
}

func fchflags(tls *libc.TLS, fd int32, flags uint32) int32 {
	panic("go2fs: unexpected call to fchflags")
}

func msync(tls *libc.TLS, addr uintptr, len1 uint64, flags int32) int32 {
	panic("go2fs: unexpected call to msync")
}
