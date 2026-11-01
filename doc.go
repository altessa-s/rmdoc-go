// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

// Package rmdoc reads and writes the .rmdoc container: the ZIP archive a
// reMarkable document is exported and imported as. It reads and writes the
// archive's structure and its files as opaque bytes; it does not parse the
// pages, the metadata or any other file inside.
//
// The package binds rmdoc-lib, a C23 library vendored into this module and
// compiled by cgo together with the miniz deflate codec; building it needs a
// C23 compiler (clang 19 or newer, GCC 13 or newer) and nothing else.
//
// # Reading
//
// [OpenReader] opens an archive in memory and [OpenReaderFile] one on disk.
// Opening validates the whole directory against [Limits]; [Reader.ForEach]
// and [Reader.Unpack] then inflate and check the payloads they select.
// Entries and unpacked files are Go values, copied out of C memory. The one
// exception is the payload [Reader.ForEach] hands its callback, which is
// valid only during the call.
//
// # Writing
//
// [Pack] builds a stored archive in memory, deterministically. [Writer]
// streams one to a file, entry by entry, with stored or deflated entries.
//
// # Names
//
// Names are UTF-8 paths with '/' between components, compared under Unicode
// canonical equivalence. [ValidateName] states which are accepted;
// [DocumentID] answers the document a name belongs to.
//
// # Errors
//
// Every failure is an [*Error]. Its [Status] says which, and the sentinel
// values (ErrInvalidName, ErrChecksumMismatch and the others) match it
// through [errors.Is]. The error of a failed callback unwraps to what the
// callback returned. A count or a length the C library states and a Go int
// cannot hold is [ErrOutOfMemory]: no Go value of that size can be allocated.
//
// # Ownership and concurrency
//
// The C library holds no mutable global state. A [Reader] owns a C copy of
// the bytes it was opened on, or a mapping of its file, until it is closed;
// it may be read from any number of goroutines at once. A [Writer] takes one
// call at a time. Both must be closed to release their C memory: neither has
// a finalizer. Go memory crosses to C only pinned and only for the length of
// a call, and no value this package answers points into C memory.
package rmdoc
