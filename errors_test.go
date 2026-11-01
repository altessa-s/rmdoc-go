// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

// sentinels lists every sentinel in Status order, from StatusInvalidArgument.
var sentinels = []*rmdoc.Error{
	rmdoc.ErrInvalidArgument, rmdoc.ErrOutOfMemory, rmdoc.ErrIO, rmdoc.ErrInvalidName,
	rmdoc.ErrDuplicateName, rmdoc.ErrEmptyArchive, rmdoc.ErrFileTooLarge,
	rmdoc.ErrArchiveTooLarge, rmdoc.ErrNotAnArchive, rmdoc.ErrMalformedArchive,
	rmdoc.ErrUnsupportedCompression, rmdoc.ErrChecksumMismatch, rmdoc.ErrArchiveFinished,
	rmdoc.ErrTooManyEntries, rmdoc.ErrEntryTooLarge, rmdoc.ErrTotalSizeExceeded,
	rmdoc.ErrCompressionRatioExceeded, rmdoc.ErrCompressedEntryTooLarge,
	rmdoc.ErrCompressedTotalSizeExceeded, rmdoc.ErrDirectoryTooLarge, rmdoc.ErrNameTooLong,
	rmdoc.ErrZIP64Unsupported, rmdoc.ErrEncryptedEntry, rmdoc.ErrSpannedArchive,
	rmdoc.ErrCallback,
}

// The Go statuses are the C library's, one sentinel each, and the first
// number past them is one the library does not name.
func TestStatusesAreTheLibrarys(t *testing.T) {
	want := []string{
		"ok", "invalid argument", "out of memory", "I/O error", "invalid name",
		"duplicate name", "empty archive", "file too large", "archive too large",
		"not an archive", "malformed archive", "unsupported compression",
		"checksum mismatch", "archive finished", "too many entries", "entry too large",
		"total size exceeded", "compression ratio exceeded", "compressed entry too large",
		"compressed total size exceeded", "directory too large", "name too long",
		"ZIP64 unsupported", "encrypted entry", "spanned archive", "callback failed",
	}
	if rmdoc.StatusCallback != rmdoc.Status(len(want)-1) {
		t.Fatalf("StatusCallback is %d, want %d", rmdoc.StatusCallback, len(want)-1)
	}
	for i, w := range want {
		if got := rmdoc.Status(i).String(); got != w {
			t.Errorf("Status(%d) = %q, want %q", i, got, w)
		}
	}
	for _, s := range []rmdoc.Status{rmdoc.Status(len(want)), -1} {
		if got := s.String(); got != "unknown status" {
			t.Errorf("Status(%d) = %q, want unknown", int(s), got)
		}
	}
	if len(sentinels) != len(want)-1 {
		t.Fatalf("%d sentinels for %d failures", len(sentinels), len(want)-1)
	}
	for i, s := range sentinels {
		if s.Status != rmdoc.Status(i+1) {
			t.Errorf("sentinel %d has %v", i, s.Status)
		}
	}
}

func TestErrorIs(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &rmdoc.Error{Status: rmdoc.StatusIO, Message: "disk"})
	if !errors.Is(err, rmdoc.ErrIO) {
		t.Errorf("does not match its sentinel")
	}
	if errors.Is(err, rmdoc.ErrInvalidName) {
		t.Errorf("matches another sentinel")
	}
	var nilErr *rmdoc.Error
	if nilErr.Is(rmdoc.ErrIO) || rmdoc.ErrIO.Is(nilErr) {
		t.Errorf("a nil *Error matches")
	}
	if nilErr.Error() != "rmdoc: <nil>" || nilErr.Unwrap() != nil {
		t.Errorf("a nil *Error spells %q", nilErr.Error())
	}
}

func TestErrorText(t *testing.T) {
	tests := []struct {
		err  *rmdoc.Error
		want string
	}{
		{&rmdoc.Error{Status: rmdoc.StatusIO, Message: "I/O error"}, "rmdoc: I/O error"},
		{&rmdoc.Error{Status: rmdoc.StatusIO, Message: "cannot open output file"},
			"rmdoc: I/O error: cannot open output file"},
		{&rmdoc.Error{Status: rmdoc.StatusInvalidName, Name: "a\nb", NameLength: 3},
			`rmdoc: invalid name ("a\nb")`},
		{&rmdoc.Error{Status: rmdoc.StatusCallback, Message: "file callback failed", Name: "x",
			NameLength: 1, Err: errors.New("stop")},
			`rmdoc: callback failed: file callback failed ("x"): stop`},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}
