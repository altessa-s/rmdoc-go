// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include "rmdoc/rmdoc.h"
*/
import "C"

import (
	"runtime"
	"sync"
)

// Writer streams an archive to a file, one entry at a time, for output too
// large to hold in memory. Write to a staging path, call Finish, and only
// then move the file into place: Close never finishes the archive, and
// leaves a partial file where the writer stopped.
//
// A failure after writing has begun makes the writer unusable: every later
// call fails with [ErrArchiveFinished], and the partial file is to be
// discarded. A name or a compression refused before anything is written
// leaves the writer as it was.
//
// A Writer may be shared between goroutines; its calls take turns.
type Writer struct {
	mu sync.Mutex
	// c is the C writer, nil once closed.
	c *C.rmd_writer
}

// CreateWriter creates or truncates the file at path and starts an archive
// in it. compression is the writer's own, [CompressionStore] or
// [CompressionDeflate]; layout decides which names it takes.
func CreateWriter(path string, compression Compression, layout Layout) (*Writer, error) {
	cpath, err := cPath(path, "path")
	if err != nil {
		return nil, err
	}
	defer freePath(cpath)
	var c *C.rmd_writer
	var cerr C.rmd_error
	status := C.rmd_writer_open(cpath, C.rmd_compression(compression), C.rmd_layout(layout), &c, &cerr)
	if err := errorOf(status, &cerr); err != nil {
		return nil, err
	}
	return &Writer{c: c}, nil
}

// Add writes one file from memory. [CompressionDefault] uses the writer's
// compression; [CompressionStore] and [CompressionDeflate] choose for this
// entry. Deflate is kept only when it makes the bytes smaller.
func (w *Writer) Add(file File, compression Compression) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.c == nil {
		return errInvalid("writer is closed")
	}
	var pin runtime.Pinner
	defer pin.Unpin()
	cfile := C.rmd_file{name: pinnedStr(&pin, file.Name), data: pinnedBytes(&pin, file.Data)}
	var cerr C.rmd_error
	return errorOf(C.rmd_writer_add(w.c, cfile, C.rmd_compression(compression), &cerr), &cerr)
}

// AddFile writes the file at sourcePath under name, reading it in bounded
// chunks. File sources are always stored.
func (w *Writer) AddFile(name, sourcePath string) error {
	cpath, err := cPath(sourcePath, "source path")
	if err != nil {
		return err
	}
	defer freePath(cpath)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.c == nil {
		return errInvalid("writer is closed")
	}
	var pin runtime.Pinner
	defer pin.Unpin()
	var cerr C.rmd_error
	return errorOf(C.rmd_writer_add_file(w.c, pinnedStr(&pin, name), cpath, &cerr), &cerr)
}

// Finish writes the central directory and closes the file. An archive with no
// entries is refused with [ErrEmptyArchive], and the writer may still take
// some. After Finish, every call but Close fails with [ErrArchiveFinished].
func (w *Writer) Finish() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.c == nil {
		return errInvalid("writer is closed")
	}
	var cerr C.rmd_error
	return errorOf(C.rmd_writer_finish(w.c, &cerr), &cerr)
}

// Close releases the writer and closes its file if Finish has not. It does
// not finish the archive or remove a partial file. Closing again does
// nothing. It always answers nil.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.c != nil {
		C.rmd_writer_close(w.c)
		w.c = nil
	}
	return nil
}
