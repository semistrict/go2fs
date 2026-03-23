package e2fs

import (
	"modernc.org/libc"
)

// Shim implementations for libc functions that modernc/libc doesn't
// provide but e2fsprogs references. These are platform-independent.

func difftime(tls *libc.TLS, time1, time0 int64) float64 {
	return float64(time1 - time0)
}

func srandom(tls *libc.TLS, seed uint32) {
	// No-op: the seed is only used by uuid gen_uuid.c which we've
	// patched to use e2fs_fill_random instead.
}

func fscanf(tls *libc.TLS, stream, format, args uintptr) int32 {
	// Return -1 (EOF) — the uuid state file read is not needed.
	return -1
}
