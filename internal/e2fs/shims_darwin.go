//go:build darwin

package e2fs

import (
	"unsafe"

	"modernc.org/libc"
)

// Darwin-specific shims for libc functions not in modernc/libc.

func getmntinfo(tls *libc.TLS, mntbufp uintptr, flags int32) int32 {
	// Return 0 entries: nothing is mounted. Correct for image files.
	//nolint:govet // C pointer from ccgo TLS stack
	*(*uintptr)(unsafe.Pointer(mntbufp)) = 0
	return 0
}

func fchflags(tls *libc.TLS, fd int32, flags uint32) int32 {
	panic("go2fs: unexpected call to fchflags")
}

func msync(tls *libc.TLS, addr uintptr, len1 uint64, flags int32) int32 {
	panic("go2fs: unexpected call to msync")
}
