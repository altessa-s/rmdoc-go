// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc

/*
#include "rmdoc/rmdoc.h"
*/
import "C"

import (
	"strconv"
	"strings"
)

// Status says what went wrong. The numbers are the C library's.
type Status int

// The statuses a call can answer with.
const (
	// StatusOK: no error. No *Error carries it.
	StatusOK Status = iota
	// StatusInvalidArgument: an argument the call cannot be made with, such
	// as an unknown layout or compression, or a reader or writer that is
	// closed.
	StatusInvalidArgument
	// StatusOutOfMemory: an allocation failed, or a count or a length is
	// larger than a Go int can hold.
	StatusOutOfMemory
	// StatusIO: a file could not be opened, mapped, read or written.
	StatusIO
	// StatusInvalidName: a name that is empty, not UTF-8, or unsafe as a path
	// in the layout. [Error.Name] holds it.
	StatusInvalidName
	// StatusDuplicateName: two entries with canonically equivalent names.
	StatusDuplicateName
	// StatusEmptyArchive: an archive with no files to write.
	StatusEmptyArchive
	// StatusFileTooLarge: a file over 4 GiB, which a ZIP without ZIP64 cannot
	// hold.
	StatusFileTooLarge
	// StatusArchiveTooLarge: more than 65,535 entries, or an archive past
	// 4 GiB.
	StatusArchiveTooLarge
	// StatusNotAnArchive: no end of central directory record.
	StatusNotAnArchive
	// StatusMalformedArchive: the ZIP structure does not hold together.
	StatusMalformedArchive
	// StatusUnsupportedCompression: a method other than stored or deflate.
	StatusUnsupportedCompression
	// StatusChecksumMismatch: a payload whose CRC-32 is not the one recorded.
	StatusChecksumMismatch
	// StatusArchiveFinished: a writer that is finished, or that a failed write
	// has made unusable.
	StatusArchiveFinished
	// StatusTooManyEntries: more entries than [Limits.MaximumEntryCount].
	StatusTooManyEntries
	// StatusEntryTooLarge: a payload over [Limits.MaximumEntrySize].
	StatusEntryTooLarge
	// StatusTotalSizeExceeded: payloads over [Limits.MaximumTotalSize].
	StatusTotalSizeExceeded
	// StatusCompressionRatioExceeded: a payload expanding past
	// [Limits.MaximumCompressionRatio].
	StatusCompressionRatioExceeded
	// StatusCompressedEntryTooLarge: a stored payload over
	// [Limits.MaximumCompressedEntrySize].
	StatusCompressedEntryTooLarge
	// StatusCompressedTotalSizeExceeded: stored payloads over
	// [Limits.MaximumCompressedTotalSize].
	StatusCompressedTotalSizeExceeded
	// StatusDirectoryTooLarge: a central directory over
	// [Limits.MaximumDirectorySize].
	StatusDirectoryTooLarge
	// StatusNameTooLong: a name over [Limits.MaximumNameLength] bytes.
	StatusNameTooLong
	// StatusZIP64Unsupported: an archive that needs ZIP64.
	StatusZIP64Unsupported
	// StatusEncryptedEntry: an encrypted entry.
	StatusEncryptedEntry
	// StatusSpannedArchive: an archive split over several disks.
	StatusSpannedArchive
	// StatusCallback: a callback of [Reader.ForEach] or [Reader.Unpack]
	// failed. [errors.Unwrap] answers what it failed with.
	StatusCallback
)

// String answers the status as a bare English phrase, and a phrase of its own
// for a number the C library does not name.
func (s Status) String() string {
	if s < 0 || int64(s) > int64(^uint32(0)) {
		return "unknown status"
	}
	return C.GoString(C.rmd_status_string(C.rmd_status(s)))
}

// Error is what every call of this package fails with. Every field but Status
// is only as meaningful as Status says.
type Error struct {
	// Status says which failure this is.
	Status Status
	// Message is what went wrong in words: the C library's phrase, or what
	// the Go side refused.
	Message string
	// Name is the entry the failure concerns, empty when it concerns none. The
	// C library keeps at most 255 bytes of it.
	Name string
	// NameLength is the full length of the name in bytes, which is more than
	// len(Name) when the name was cut short.
	NameLength int
	// Actual and Limit are the value found and the bound it broke, for the
	// failures of a limit.
	Actual, Limit uint64
	// Err is what a callback failed with, for StatusCallback; nil otherwise.
	Err error
}

// Sentinels for [errors.Is]: an *Error matches the sentinel of its Status,
// whatever its other fields hold.
var (
	ErrInvalidArgument             = &Error{Status: StatusInvalidArgument}
	ErrOutOfMemory                 = &Error{Status: StatusOutOfMemory}
	ErrIO                          = &Error{Status: StatusIO}
	ErrInvalidName                 = &Error{Status: StatusInvalidName}
	ErrDuplicateName               = &Error{Status: StatusDuplicateName}
	ErrEmptyArchive                = &Error{Status: StatusEmptyArchive}
	ErrFileTooLarge                = &Error{Status: StatusFileTooLarge}
	ErrArchiveTooLarge             = &Error{Status: StatusArchiveTooLarge}
	ErrNotAnArchive                = &Error{Status: StatusNotAnArchive}
	ErrMalformedArchive            = &Error{Status: StatusMalformedArchive}
	ErrUnsupportedCompression      = &Error{Status: StatusUnsupportedCompression}
	ErrChecksumMismatch            = &Error{Status: StatusChecksumMismatch}
	ErrArchiveFinished             = &Error{Status: StatusArchiveFinished}
	ErrTooManyEntries              = &Error{Status: StatusTooManyEntries}
	ErrEntryTooLarge               = &Error{Status: StatusEntryTooLarge}
	ErrTotalSizeExceeded           = &Error{Status: StatusTotalSizeExceeded}
	ErrCompressionRatioExceeded    = &Error{Status: StatusCompressionRatioExceeded}
	ErrCompressedEntryTooLarge     = &Error{Status: StatusCompressedEntryTooLarge}
	ErrCompressedTotalSizeExceeded = &Error{Status: StatusCompressedTotalSizeExceeded}
	ErrDirectoryTooLarge           = &Error{Status: StatusDirectoryTooLarge}
	ErrNameTooLong                 = &Error{Status: StatusNameTooLong}
	ErrZIP64Unsupported            = &Error{Status: StatusZIP64Unsupported}
	ErrEncryptedEntry              = &Error{Status: StatusEncryptedEntry}
	ErrSpannedArchive              = &Error{Status: StatusSpannedArchive}
	ErrCallback                    = &Error{Status: StatusCallback}
)

// Error spells the status, then the message when it says more, the name it
// concerns, and the cause of a failed callback.
func (e *Error) Error() string {
	if e == nil {
		return "rmdoc: <nil>"
	}
	var b strings.Builder
	b.WriteString("rmdoc: ")
	status := e.Status.String()
	b.WriteString(status)
	if e.Message != "" && e.Message != status {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Name != "" {
		// A name may hold anything; quoted, it stays one line.
		b.WriteString(" (")
		b.WriteString(strconv.Quote(e.Name))
		if e.NameLength > len(e.Name) {
			b.WriteString("...")
		}
		b.WriteString(")")
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Is reports whether target is an *Error of the same Status. A nil *Error,
// on either side, matches nothing.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t != nil && e != nil && t.Status == e.Status
}

// Unwrap answers what a callback failed with, nil for every other failure.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// errorOf turns a C status and the error record beside it into a Go error,
// nil for success.
func errorOf(status C.rmd_status, cerr *C.rmd_error) error {
	if status == C.RMD_OK {
		return nil
	}
	return cError(status, cerr)
}

// cError turns a failed C status and the error record beside it into an
// *Error.
func cError(status C.rmd_status, cerr *C.rmd_error) *Error {
	e := &Error{
		Status:  Status(status),
		Message: C.GoString(&cerr.message[0]),
		Actual:  uint64(cerr.actual),
		Limit:   uint64(cerr.limit),
	}
	// The record holds the first bytes of the name, at most one fewer than
	// its array, and the full length beside them.
	if cerr.name_length > 0 {
		kept := uint64(len(cerr.name) - 1)
		full := uint64(cerr.name_length)
		e.Name = C.GoStringN(&cerr.name[0], C.int(min(full, kept)))
		e.NameLength = len(e.Name)
		if n, err := goLength(cerr.name_length, "name length"); err == nil {
			e.NameLength = n
		}
	}
	return e
}

// errInvalid answers an argument the Go side refuses before C sees it.
func errInvalid(message string) error {
	return &Error{Status: StatusInvalidArgument, Message: message}
}

// errTooLarge answers a count or a length C states and a Go int cannot hold.
// No Go value of that size can be allocated, so it is reported the way a
// failed allocation is.
func errTooLarge(what string) error {
	return &Error{Status: StatusOutOfMemory, Message: what + " too large for a Go int"}
}
