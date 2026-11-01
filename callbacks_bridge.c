// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

// The C side of Reader.ForEach and Reader.Unpack: callbacks with the
// signatures the library calls, which hand each entry to the exported Go
// functions. The context is a runtime/cgo.Handle, carried as an integer so
// that no Go pointer crosses to C. Kept apart from callbacks.go because a Go
// file that exports to C may only declare, not define, in its preamble.

#include "_cgo_export.h"
#include "rmdoc/rmdoc.h"

static rmd_status rmdgo_select(void *context, const rmd_entry *entry, bool *include) {
    int answer = rmdgoSelect((uintptr_t)context, (rmd_entry *)entry);
    if (answer < 0)
        return RMD_ERR_CALLBACK;
    *include = answer > 0;
    return RMD_OK;
}

static rmd_status rmdgo_body(void *context, const rmd_entry *entry, rmd_bytes payload) {
    int answer =
        rmdgoBody((uintptr_t)context, (rmd_entry *)entry, (uint8_t *)payload.bytes, payload.size);
    return answer == 0 ? RMD_OK : RMD_ERR_CALLBACK;
}

rmd_status rmdgo_foreach(const rmd_reader *reader, uintptr_t handle, bool selecting,
                         rmd_error *error) {
    void *context = (void *)handle;
    return rmd_reader_foreach(reader, selecting ? rmdgo_select : nullptr, context, rmdgo_body,
                              context, error);
}

rmd_status rmdgo_unpack(const rmd_reader *reader, uintptr_t handle, bool selecting,
                        rmd_archive *out, rmd_error *error) {
    return rmd_reader_unpack(reader, selecting ? rmdgo_select : nullptr, (void *)handle, out,
                             error);
}
