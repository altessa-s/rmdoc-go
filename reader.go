// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include <stdlib.h>
#include <string.h>
#include "rmdoc/rmdoc.h"

// In callbacks_bridge.c.
rmd_status rmdgo_foreach(const rmd_reader *reader, uintptr_t handle, bool selecting,
                         rmd_error *error);
rmd_status rmdgo_unpack(const rmd_reader *reader, uintptr_t handle, bool selecting,
                        rmd_archive *out, rmd_error *error);
*/
import "C"

import (
	"slices"
	"sync"
	"unsafe"
)

// Reader reads the entries of an archive. Opening it validates the whole
// central directory and every local header, without inflating anything; the
// payloads are inflated and checked against their CRC-32 only as they are
// read.
//
// A Reader may be used from any number of goroutines at once: reading never
// changes it. Close releases it, and may be called at any time, also while
// reads are in flight or from inside a callback: those reads finish first,
// and the memory goes when the last of them returns.
type Reader struct {
	mu sync.Mutex
	// c is the C reader, nil once it is released.
	c *C.rmd_reader
	// data is the C copy of the archive OpenReader keeps, nil for a reader
	// of a file.
	data unsafe.Pointer
	// active counts the reads in flight; closed is set by Close.
	active int
	closed bool
	// entries is the central directory, copied once at open.
	entries []Entry
}

// OpenReader opens an archive held in memory. The bytes are copied, so data
// may be changed or dropped as soon as OpenReader returns. A nil limits
// selects [DefaultLimits].
func OpenReader(data []byte, limits *Limits) (*Reader, error) {
	var copied unsafe.Pointer
	if len(data) > 0 {
		copied = C.malloc(C.size_t(len(data)))
		if copied == nil {
			return nil, &Error{Status: StatusOutOfMemory, Message: "C memory for the archive"}
		}
		copy(unsafe.Slice((*byte)(copied), len(data)), data)
	}
	in := C.rmd_bytes{bytes: (*C.uint8_t)(copied), size: C.size_t(len(data))}
	var climits *C.rmd_limits
	if limits != nil {
		l := limits.toC()
		climits = &l
	}
	var c *C.rmd_reader
	var cerr C.rmd_error
	if err := errorOf(C.rmd_reader_open(in, climits, &c, &cerr), &cerr); err != nil {
		C.free(copied)
		return nil, err
	}
	r, err := newReader(c, copied)
	if err != nil {
		C.rmd_reader_close(c)
		C.free(copied)
		return nil, err
	}
	return r, nil
}

// OpenReaderFile opens the archive at path. The file is mapped, not read:
// keep it unchanged until the reader is closed, since a mapping is not a
// snapshot. A nil limits selects [DefaultLimits].
func OpenReaderFile(path string, limits *Limits) (*Reader, error) {
	cpath, err := cPath(path, "path")
	if err != nil {
		return nil, err
	}
	defer freePath(cpath)
	var climits *C.rmd_limits
	if limits != nil {
		l := limits.toC()
		climits = &l
	}
	var c *C.rmd_reader
	var cerr C.rmd_error
	if err := errorOf(C.rmd_reader_open_file(cpath, climits, &c, &cerr), &cerr); err != nil {
		return nil, err
	}
	r, err := newReader(c, nil)
	if err != nil {
		C.rmd_reader_close(c)
		return nil, err
	}
	return r, nil
}

// newReader copies the central directory of an open C reader.
func newReader(c *C.rmd_reader, data unsafe.Pointer) (*Reader, error) {
	n, err := goLength(C.rmd_reader_count(c), "entry count")
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, n)
	for i := range entries {
		e := C.rmd_reader_entry(c, C.size_t(i))
		if e == nil {
			return nil, &Error{Status: StatusMalformedArchive, Message: "entry missing from the directory"}
		}
		if entries[i], err = entryFromC(e); err != nil {
			return nil, err
		}
	}
	return &Reader{c: c, data: data, entries: entries}, nil
}

// Close releases the reader. Reads already in flight finish first; reads
// begun after it fail with [ErrInvalidArgument]. Closing again does nothing.
// It always answers nil.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.active == 0 {
		r.release()
	}
	return nil
}

// release frees the C reader and the archive copy; r.mu is held.
func (r *Reader) release() {
	C.rmd_reader_close(r.c)
	C.free(r.data)
	r.c, r.data = nil, nil
}

// acquire answers the C reader for one read, or fails when r is closed.
func (r *Reader) acquire() (*C.rmd_reader, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errInvalid("reader is closed")
	}
	r.active++
	return r.c, nil
}

// done ends a read begun by acquire, releasing the reader when it was closed
// meanwhile and this was the last read.
func (r *Reader) done() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active--
	if r.closed && r.active == 0 {
		r.release()
	}
}

// Entries answers the archive's entries in directory order. They were copied
// at open, so they stay readable after Close.
func (r *Reader) Entries() []Entry {
	return slices.Clone(r.entries)
}

// Len answers how many entries the archive has.
func (r *Reader) Len() int {
	return len(r.entries)
}

// DocumentIDs answers the documents the entries belong to, each once, in the
// order they first appear; see [DocumentID]. Canonically equivalent IDs
// count as one.
func (r *Reader) DocumentIDs() ([]string, error) {
	c, err := r.acquire()
	if err != nil {
		return nil, err
	}
	defer r.done()
	var ids C.rmd_strings
	var cerr C.rmd_error
	if err := errorOf(C.rmd_reader_document_ids(c, &ids, &cerr), &cerr); err != nil {
		return nil, err
	}
	defer C.rmd_strings_free(&ids)
	n, err := goLength(ids.count, "document count")
	if err != nil || n == 0 {
		return []string{}, err
	}
	out := make([]string, n)
	for i, s := range unsafe.Slice(ids.values, n) {
		if out[i], err = goString(s, "document ID length"); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ForEach reads the entries selectFn chooses, in directory order, and calls
// body with each and its uncompressed bytes. A nil selectFn chooses every
// entry.
//
// selectFn is called for every entry before any payload is read, and the
// limits of the reader are checked against the chosen entries before body is
// first called. A payload that fails its checksum or does not inflate stops
// the walk, so body may have seen earlier entries when ForEach fails.
//
// The bytes body receives are valid only during the call: they are released
// as it returns, so body must copy whatever it keeps. When body returns an
// error, ForEach stops and fails with an *Error of [StatusCallback] that
// unwraps to it. A panic in either function stops the walk and is raised
// again from ForEach after C returns and the handle is deleted. A
// runtime.Goexit stops the walk and exits the calling goroutine after the
// same cleanup, running its deferred functions.
//
// Both functions run on separate goroutines, synchronously with ForEach;
// they do not inherit the calling goroutine's state, such as LockOSThread.
func (r *Reader) ForEach(selectFn func(Entry) bool, body func(Entry, []byte) error) error {
	if body == nil {
		return errInvalid("ForEach needs a body")
	}
	c, err := r.acquire()
	if err != nil {
		return err
	}
	defer r.done()
	v := &visit{selectFn: selectFn, body: body}
	var cerr C.rmd_error
	status := v.run(func(handle C.uintptr_t) C.rmd_status {
		return C.rmdgo_foreach(c, handle, C.bool(selectFn != nil), &cerr)
	})
	return v.callbackError(status, &cerr)
}

// Unpack reads the entries selectFn chooses into memory, in directory order.
// A nil selectFn chooses every entry; choosing none answers no files and no
// error. selectFn is called for every entry first, as in [Reader.ForEach].
// The files are Go memory and outlive the reader. selectFn runs on a
// separate goroutine, synchronously with Unpack. As with [Reader.ForEach],
// a panic or runtime.Goexit stops C, deletes the handle, and then panics or
// exits on the calling goroutine, running its deferred functions.
func (r *Reader) Unpack(selectFn func(Entry) bool) ([]File, error) {
	c, err := r.acquire()
	if err != nil {
		return nil, err
	}
	defer r.done()
	v := &visit{selectFn: selectFn}
	var archive C.rmd_archive
	var cerr C.rmd_error
	status := v.run(func(handle C.uintptr_t) C.rmd_status {
		return C.rmdgo_unpack(c, handle, C.bool(selectFn != nil), &archive, &cerr)
	})
	if err := v.callbackError(status, &cerr); err != nil {
		return nil, err
	}
	defer C.rmd_archive_free(&archive)
	n, err := goLength(archive.count, "file count")
	if err != nil || n == 0 {
		return []File{}, err
	}
	files := make([]File, n)
	for i, f := range unsafe.Slice(archive.files, n) {
		if files[i].Name, err = goString(f.name, "name length"); err != nil {
			return nil, err
		}
		if files[i].Data, err = goBytes(f.data.bytes, f.data.size, "file size"); err != nil {
			return nil, err
		}
	}
	return files, nil
}
