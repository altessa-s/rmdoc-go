// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

// document answers the files of a small document: its metadata, content and
// one page, with a payload long and regular enough for deflate to shrink.
func document() []rmdoc.File {
	return []rmdoc.File{
		{Name: "a1b2.metadata", Data: []byte(`{"visibleName":"Notes"}`)},
		{Name: "a1b2.content", Data: []byte(`{"pageCount":1}`)},
		{Name: "a1b2/p1.rm", Data: bytes.Repeat([]byte("reMarkable .lines file, version=6 "), 64)},
		{Name: "c3d4.metadata", Data: []byte{}},
	}
}

// packed answers document packed into an archive.
func packed(t testing.TB) []byte {
	t.Helper()
	archive, err := rmdoc.Pack(document())
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

// open answers a reader of data, closed when the test ends.
func open(t testing.TB, data []byte, limits *rmdoc.Limits) *rmdoc.Reader {
	t.Helper()
	r, err := rmdoc.OpenReader(data, limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// wantStatus fails the test unless err is an *Error of the given status.
func wantStatus(t testing.TB, err error, want rmdoc.Status) {
	t.Helper()
	_ = wantError(t, err, want)
}

// wantError is wantStatus, answering the *Error.
func wantError(t testing.TB, err error, want rmdoc.Status) *rmdoc.Error {
	t.Helper()
	var e *rmdoc.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want an *Error of %v", err, want)
	}
	if e.Status != want {
		t.Fatalf("got %v, want %v", err, want)
	}
	return e
}

// sameFiles fails the test unless got and want hold the same names and bytes
// in the same order.
func sameFiles(t testing.TB, got, want []rmdoc.File) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d files, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || !bytes.Equal(got[i].Data, want[i].Data) {
			t.Fatalf("file %d is %q (%d bytes), want %q (%d bytes)",
				i, got[i].Name, len(got[i].Data), want[i].Name, len(want[i].Data))
		}
	}
}
