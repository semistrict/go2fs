//go:build darwin && arm64

package e2fs

import (
	"math/rand"
	"unsafe"

	"modernc.org/libc"
)

// Shim implementations for libc functions that modernc/libc doesn't
// provide on darwin/arm64 but e2fsprogs references.

func difftime(tls *libc.TLS, time1, time0 int64) float64 {
	return float64(time1 - time0)
}

func posix_memalign(tls *libc.TLS, memptr uintptr, alignment, size uint64) int32 {
	p := libc.Xmalloc(tls, size)
	if p == 0 {
		return -1
	}
	*(*uintptr)(unsafe.Pointer(memptr)) = p
	return 0
}

func srandom(tls *libc.TLS, seed uint32) {
	rand.Seed(int64(seed))
}

func getmntinfo(tls *libc.TLS, mntbufp uintptr, flags int32) int32 {
	// Return 0 entries: nothing is mounted. Correct for image files.
	*(*uintptr)(unsafe.Pointer(mntbufp)) = 0
	return 0
}

func fchflags(tls *libc.TLS, fd int32, flags uint32) int32 {
	panic("go2fs: unexpected call to fchflags")
}

func msync(tls *libc.TLS, addr uintptr, len1 uint64, flags int32) int32 {
	panic("go2fs: unexpected call to msync")
}

func fscanf(tls *libc.TLS, stream, format, args uintptr) int32 {
	// Return -1 (EOF) — the uuid state file read is not needed.
	return -1
}
