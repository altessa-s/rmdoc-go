// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

// CVersion lets the external tests hold Version against the C library's.
var CVersion = cVersion

// LiveHandles answers how many calls still hold a C callback handle.
func LiveHandles() int64 { return liveHandles.Load() }
