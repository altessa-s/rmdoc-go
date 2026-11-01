// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

// Compiles the vendored rmd_io.c into this package: cgo builds only the C
// files of the package directory itself. The digest ties the object to the
// vendored tree; see clib_digest.h.

#include "clib_digest.h"
#include "internal/clib/src/rmd_io.c"
