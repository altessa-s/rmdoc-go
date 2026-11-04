// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

// -std=c2x and not -std=c23: GCC 13 knows the standard only by its draft
// name, while GCC 14 and clang 19 or newer take both. The library is written
// with nullptr, typed enums and constexpr, which clang 18 does not read, so no
// flag rescues an older compiler.

/*
#cgo CFLAGS: -std=c2x -I${SRCDIR}/internal/clib/include -I${SRCDIR}/internal/clib/src

#include "rmdoc/rmdoc.h"
*/
import "C"

// Version is the version of the vendored rmdoc-lib.
const Version = "1.1.0"

// cVersion answers the C library's own version, so that a test can hold
// [Version] to it.
func cVersion() string {
	return C.GoString(C.rmd_version())
}
