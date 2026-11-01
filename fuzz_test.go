// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/altessa-s/rmdoc-go"
)

var errUnchecked = errors.New("payload delivered unchecked")

// fuzzLimits keep one input cheap: arbitrary bytes may declare payloads of
// gigabytes.
func fuzzLimits() *rmdoc.Limits {
	l := rmdoc.FormatLimits()
	l.MaximumEntrySize = 1 << 20
	l.MaximumTotalSize = 4 << 20
	return &l
}

// FuzzReader opens arbitrary bytes as an archive, seeded with
// testdata/fuzz-seeds. It must answer a reader or an error and never crash.
// What a reader answers must hold together: every entry has a valid device
// name, ForEach and Unpack agree with each other and with the checksums, and
// the files pack into an archive that reads back as the same files.
func FuzzReader(f *testing.F) {
	seeds, err := filepath.Glob(filepath.Join("testdata", "fuzz-seeds", "*"))
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r, err := rmdoc.OpenReader(data, fuzzLimits())
		if err != nil {
			if r != nil {
				t.Fatal("a reader and an error together")
			}
			var e *rmdoc.Error
			if !errors.As(err, &e) || e.Status == rmdoc.StatusOK {
				t.Fatalf("not an *Error: %v", err)
			}
			return
		}
		defer r.Close()
		entries := r.Entries()
		for _, e := range entries {
			if err := rmdoc.ValidateName(e.Name, rmdoc.LayoutDevice); err != nil {
				t.Fatalf("an entry the reader took has an invalid name: %v", err)
			}
		}
		if _, err := r.DocumentIDs(); err != nil {
			t.Fatal(err)
		}

		var walked []rmdoc.File
		walkErr := r.ForEach(nil, func(e rmdoc.Entry, payload []byte) error {
			// No t.Fatal here: it would leave the goroutine through C.
			if crc32.ChecksumIEEE(payload) != e.CRC32 || len(payload) != int(e.UncompressedSize) {
				t.Errorf("%q was delivered unchecked", e.Name)
				return errUnchecked
			}
			walked = append(walked, rmdoc.File{Name: e.Name, Data: bytes.Clone(payload)})
			return nil
		})
		files, unpackErr := r.Unpack(nil)
		if (walkErr == nil) != (unpackErr == nil) {
			t.Fatalf("ForEach: %v, Unpack: %v", walkErr, unpackErr)
		}
		if unpackErr != nil {
			return
		}
		if !equalFiles(walked, files) || len(files) != len(entries) {
			t.Fatal("ForEach and Unpack disagree")
		}
		if len(files) == 0 {
			return // Pack refuses an empty archive
		}

		archive, err := rmdoc.Pack(files)
		if err != nil {
			t.Fatalf("the files of an archive do not pack: %v", err)
		}
		again, err := rmdoc.OpenReader(archive, nil)
		if err != nil {
			t.Fatalf("a packed archive does not open: %v", err)
		}
		defer again.Close()
		back, err := again.Unpack(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !equalFiles(back, files) {
			t.Fatal("packed files read back differently")
		}
	})
}

// FuzzName holds ValidateName and DocumentID to what they promise: a valid
// name is well-formed UTF-8 without NUL, a device name is valid in the
// package layout too, and a document ID is a prefix of its name that holds
// no '/'.
func FuzzName(f *testing.F) {
	for _, s := range []string{"a1b2/p1.rm", "a1b2.content", ".x", "a/b/c", "../x", "é", "caf\xe9", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		device := rmdoc.ValidateName(name, rmdoc.LayoutDevice)
		pkg := rmdoc.ValidateName(name, rmdoc.LayoutPackage)
		for _, err := range []error{device, pkg} {
			if err != nil && !errors.Is(err, rmdoc.ErrInvalidName) {
				t.Fatalf("%q: %v", name, err)
			}
		}
		if device == nil {
			if pkg != nil {
				t.Fatalf("%q is a device name but not a package name", name)
			}
			if !utf8.ValidString(name) || bytes.IndexByte([]byte(name), 0) >= 0 {
				t.Fatalf("%q was accepted", name)
			}
		}
		id := rmdoc.DocumentID(name)
		if len(id) > len(name) || name[:len(id)] != id || bytes.IndexByte([]byte(id), '/') >= 0 {
			t.Fatalf("DocumentID(%q) = %q", name, id)
		}
	})
}
