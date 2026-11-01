// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include "rmdoc/rmdoc.h"
*/
import "C"

import "runtime"

// Pack writes files into a stored, device-layout archive held in memory, in
// the order given. It is deterministic: the same files give the same bytes,
// with every timestamp at 1980-01-01. There must be at least one file and at
// most 65,535, with valid names (see [ValidateName]), no two canonically
// equivalent, and no file or archive past 4 GiB.
//
// The files are read in place for the length of the call, not copied, and
// must not be changed while Pack runs.
func Pack(files []File) ([]byte, error) {
	var pin runtime.Pinner
	defer pin.Unpin()
	// Go memory handed to C may hold Go pointers only when they are pinned;
	// every name and every payload is.
	cfiles := make([]C.rmd_file, len(files))
	for i, f := range files {
		cfiles[i] = C.rmd_file{name: pinnedStr(&pin, f.Name), data: pinnedBytes(&pin, f.Data)}
	}
	var first *C.rmd_file
	if len(cfiles) > 0 {
		first = &cfiles[0]
	}
	var out C.rmd_buffer
	var cerr C.rmd_error
	if err := errorOf(C.rmd_pack(first, C.size_t(len(cfiles)), &out, &cerr), &cerr); err != nil {
		return nil, err
	}
	defer C.rmd_buffer_free(&out)
	return goBytes(out.bytes, out.size, "archive size")
}
