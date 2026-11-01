// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include <stdlib.h>
#include "rmdoc/rmdoc.h"
*/
import "C"

import (
	"math"
	"runtime"
	"strings"
	"unsafe"
)

// goLength converts a C count or length to a Go int, refusing one past
// math.MaxInt.
func goLength(n C.size_t, what string) (int, error) {
	if uint64(n) > math.MaxInt {
		return 0, errTooLarge(what)
	}
	return int(n), nil
}

// goString copies counted C text into a Go string, by its length and not up
// to a NUL.
func goString(s C.rmd_str, what string) (string, error) {
	n, err := goLength(s.length, what)
	if err != nil || n == 0 || s.bytes == nil {
		return "", err
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(s.bytes)), n)), nil
}

// goBytes copies a C byte range into Go memory. An empty range answers an
// empty, non-nil slice, so that a file of no bytes reads as one.
func goBytes(p *C.uint8_t, size C.size_t, what string) ([]byte, error) {
	n, err := goLength(size, what)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	if n > 0 {
		copy(out, unsafe.Slice((*byte)(unsafe.Pointer(p)), n))
	}
	return out, nil
}

// borrowedBytes answers a Go slice over C memory, without copying. It is
// valid only as long as the C memory is, and must not escape that.
func borrowedBytes(p *C.uint8_t, size C.size_t) ([]byte, error) {
	n, err := goLength(size, "payload size")
	if err != nil || n == 0 || p == nil {
		return []byte{}, err
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(p)), n), nil
}

// pinnedStr answers s as counted C text over its own bytes, pinned by p so
// that C memory or Go memory handed to C may hold the pointer for the length
// of a call. C reads it and keeps nothing.
func pinnedStr(p *runtime.Pinner, s string) C.rmd_str {
	if len(s) == 0 {
		return C.rmd_str{}
	}
	data := unsafe.StringData(s)
	p.Pin(data)
	return C.rmd_str{bytes: (*C.char)(unsafe.Pointer(data)), length: C.size_t(len(s))}
}

// pinnedBytes answers b as a C byte range over its own memory, pinned by p.
func pinnedBytes(p *runtime.Pinner, b []byte) C.rmd_bytes {
	if len(b) == 0 {
		return C.rmd_bytes{}
	}
	p.Pin(&b[0])
	return C.rmd_bytes{bytes: (*C.uint8_t)(unsafe.Pointer(&b[0])), size: C.size_t(len(b))}
}

// cPath copies a path into C memory as a NUL-terminated string, refusing one
// that holds a NUL, which C would read as its end. The caller frees it.
func cPath(path, what string) (*C.char, error) {
	if strings.IndexByte(path, 0) >= 0 {
		return nil, errInvalid(what + " holds a NUL byte")
	}
	return C.CString(path), nil
}

func freePath(p *C.char) {
	C.free(unsafe.Pointer(p))
}
