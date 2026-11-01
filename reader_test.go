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
	"slices"
	"strings"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

func TestPackIsDeterministic(t *testing.T) {
	a, b := packed(t), packed(t)
	if !bytes.Equal(a, b) {
		t.Fatal("the same files packed twice differ")
	}
	if !bytes.HasPrefix(a, []byte("PK\x03\x04")) {
		t.Fatal("not a ZIP")
	}
}

func TestPackRefuses(t *testing.T) {
	tests := []struct {
		files []rmdoc.File
		want  rmdoc.Status
	}{
		{nil, rmdoc.StatusEmptyArchive},
		{[]rmdoc.File{}, rmdoc.StatusEmptyArchive},
		{[]rmdoc.File{{Name: "a/b/c"}}, rmdoc.StatusInvalidName},
		{[]rmdoc.File{{Name: "x.rm"}, {Name: "x.rm"}}, rmdoc.StatusDuplicateName},
		// Canonically equivalent: é precomposed, and e with a combining acute.
		{[]rmdoc.File{{Name: "\u00e9.rm"}, {Name: "e\u0301.rm"}}, rmdoc.StatusDuplicateName},
	}
	for _, tt := range tests {
		_, err := rmdoc.Pack(tt.files)
		wantStatus(t, err, tt.want)
	}
}

func TestReadBack(t *testing.T) {
	files := document()
	r := open(t, packed(t), nil)
	if r.Len() != len(files) {
		t.Fatalf("%d entries, want %d", r.Len(), len(files))
	}
	for i, e := range r.Entries() {
		f := files[i]
		if e.Name != f.Name || e.Method != 0 || int(e.UncompressedSize) != len(f.Data) ||
			e.CompressedSize != e.UncompressedSize || e.CRC32 != crc32.ChecksumIEEE(f.Data) {
			t.Errorf("entry %d is %+v for %q, %d bytes", i, e, f.Name, len(f.Data))
		}
	}
	ids, err := r.DocumentIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{"a1b2", "c3d4"}) {
		t.Fatalf("document IDs %q", ids)
	}
	got, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, files)
	if got[3].Data == nil {
		t.Errorf("an empty file unpacks as nil")
	}
}

func TestEntriesAreCopies(t *testing.T) {
	r := open(t, packed(t), nil)
	entries := r.Entries()
	entries[0].Name = "changed"
	if r.Entries()[0].Name == "changed" {
		t.Fatal("Entries answers the reader's own slice")
	}
}

func TestOpenReaderCopiesItsInput(t *testing.T) {
	data := packed(t)
	r := open(t, data, nil)
	for i := range data {
		data[i] = 0
	}
	got, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, document())
}

func TestUnpackSelects(t *testing.T) {
	r := open(t, packed(t), nil)
	var seen []string
	got, err := r.Unpack(func(e rmdoc.Entry) bool {
		seen = append(seen, e.Name)
		return e.DocumentID() == "a1b2" && strings.HasSuffix(e.Name, ".rm")
	})
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, document()[2:3])
	if len(seen) != r.Len() {
		t.Errorf("selection saw %d of %d entries", len(seen), r.Len())
	}
	none, err := r.Unpack(func(rmdoc.Entry) bool { return false })
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("selecting nothing: %v, %v", none, err)
	}
}

func TestForEach(t *testing.T) {
	r := open(t, packed(t), nil)
	var got []rmdoc.File
	err := r.ForEach(func(e rmdoc.Entry) bool { return e.Name != "a1b2.content" },
		func(e rmdoc.Entry, data []byte) error {
			got = append(got, rmdoc.File{Name: e.Name, Data: bytes.Clone(data)})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	want := document()
	sameFiles(t, got, append(want[:1:1], want[2:]...))

	got = got[:0]
	if err := r.ForEach(nil, func(e rmdoc.Entry, data []byte) error {
		got = append(got, rmdoc.File{Name: e.Name, Data: bytes.Clone(data)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, document())

	wantStatus(t, r.ForEach(nil, nil), rmdoc.StatusInvalidArgument)
}

func TestForEachReturnsTheCallbacksError(t *testing.T) {
	r := open(t, packed(t), nil)
	stop := errors.New("stop here")
	calls := 0
	err := r.ForEach(nil, func(rmdoc.Entry, []byte) error {
		calls++
		if calls == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || !errors.Is(err, rmdoc.ErrCallback) {
		t.Fatalf("got %v", err)
	}
	e := wantError(t, err, rmdoc.StatusCallback)
	if e.Name != "a1b2.content" {
		t.Errorf("the failure names %q", e.Name)
	}
	if calls != 2 {
		t.Errorf("%d calls after the failure", calls)
	}
}

func TestCallbackPanicsAreRaisedAgain(t *testing.T) {
	r := open(t, packed(t), nil)
	type marker struct{}
	for name, run := range map[string]func(){
		"select": func() { _, _ = r.Unpack(func(rmdoc.Entry) bool { panic(marker{}) }) },
		"body": func() {
			_ = r.ForEach(nil, func(rmdoc.Entry, []byte) error { panic(marker{}) })
		},
	} {
		func() {
			defer func() {
				if p := recover(); p != (marker{}) {
					t.Errorf("%s: recovered %v", name, p)
				}
			}()
			run()
			t.Errorf("%s: no panic", name)
		}()
	}
	// The reader is intact and was released by neither panic.
	if _, err := r.Unpack(nil); err != nil {
		t.Fatal(err)
	}
}

func TestChecksumMismatch(t *testing.T) {
	data := packed(t)
	// The first payload follows its 30-byte local header and name.
	data[30+len("a1b2.metadata")] ^= 0xff
	r := open(t, data, nil)
	var seen int
	err := r.ForEach(nil, func(rmdoc.Entry, []byte) error { seen++; return nil })
	e := wantError(t, err, rmdoc.StatusChecksumMismatch)
	if e.Name != "a1b2.metadata" || seen != 0 {
		t.Fatalf("%v after %d payloads", e, seen)
	}
	// Skipped payloads are not checked.
	if _, err := r.Unpack(func(e rmdoc.Entry) bool { return e.Name != "a1b2.metadata" }); err != nil {
		t.Fatal(err)
	}
}

func TestReaderLimits(t *testing.T) {
	data := packed(t)
	limits := rmdoc.DefaultLimits()
	limits.MaximumEntryCount = 3
	_, err := rmdoc.OpenReader(data, &limits)
	e := wantError(t, err, rmdoc.StatusTooManyEntries)
	if e.Actual != 4 || e.Limit != 3 {
		t.Errorf("actual %d, limit %d", e.Actual, e.Limit)
	}

	limits = rmdoc.DefaultLimits()
	limits.MaximumEntrySize = 100
	r := open(t, data, &limits)
	_, err = r.Unpack(nil)
	wantStatus(t, err, rmdoc.StatusEntryTooLarge)
	// Budgets apply only to the selected payloads.
	if _, err := r.Unpack(func(e rmdoc.Entry) bool { return e.UncompressedSize <= 100 }); err != nil {
		t.Fatal(err)
	}
	// Budgets fail before any body call.
	calls := 0
	err = r.ForEach(nil, func(rmdoc.Entry, []byte) error { calls++; return nil })
	wantStatus(t, err, rmdoc.StatusEntryTooLarge)
	if calls != 0 {
		t.Errorf("%d body calls before a budget failure", calls)
	}
}

func TestNotAnArchive(t *testing.T) {
	for _, data := range [][]byte{nil, {}, []byte("PK"), bytes.Repeat([]byte{0}, 100)} {
		_, err := rmdoc.OpenReader(data, nil)
		wantStatus(t, err, rmdoc.StatusNotAnArchive)
	}
}

func TestClosedReader(t *testing.T) {
	r, err := rmdoc.OpenReader(packed(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if r.Len() != 4 || len(r.Entries()) != 4 {
		t.Errorf("the entries went with the reader")
	}
	_, err = r.Unpack(nil)
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
	_, err = r.DocumentIDs()
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
	wantStatus(t, r.ForEach(nil, func(rmdoc.Entry, []byte) error { return nil }),
		rmdoc.StatusInvalidArgument)
}

// Closing from inside a callback lets the walk finish; the reader goes when
// it returns.
func TestCloseInsideCallback(t *testing.T) {
	r, err := rmdoc.OpenReader(packed(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []rmdoc.File
	err = r.ForEach(nil, func(e rmdoc.Entry, data []byte) error {
		_ = r.Close()
		got = append(got, rmdoc.File{Name: e.Name, Data: bytes.Clone(data)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, document())
	_, err = r.Unpack(nil)
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
}

// A callback may read the same reader again.
func TestNestedReads(t *testing.T) {
	r := open(t, packed(t), nil)
	err := r.ForEach(nil, func(e rmdoc.Entry, data []byte) error {
		inner, err := r.Unpack(func(x rmdoc.Entry) bool { return x.Name == e.Name })
		if err != nil {
			return err
		}
		if len(inner) != 1 || !bytes.Equal(inner[0].Data, data) {
			return errors.New("the inner read differs")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenReaderFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.rmdoc")
	if err := os.WriteFile(path, packed(t), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := rmdoc.OpenReaderFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, document())

	_, err = rmdoc.OpenReaderFile(filepath.Join(dir, "missing.rmdoc"), nil)
	wantStatus(t, err, rmdoc.StatusIO)
	_, err = rmdoc.OpenReaderFile(dir, nil)
	wantStatus(t, err, rmdoc.StatusIO)
	_, err = rmdoc.OpenReaderFile("doc\x00.rmdoc", nil)
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
	empty := filepath.Join(dir, "empty.rmdoc")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = rmdoc.OpenReaderFile(empty, nil)
	wantStatus(t, err, rmdoc.StatusNotAnArchive)
}

// The seeds of the C library: archives in the shapes it reads, each with a
// payload that checks out.
func TestSeedsRead(t *testing.T) {
	seeds, err := filepath.Glob(filepath.Join("testdata", "fuzz-seeds", "*"))
	if err != nil || len(seeds) == 0 {
		t.Fatalf("no seeds: %v", err)
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		r, err := rmdoc.OpenReader(data, nil)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
			continue
		}
		files, err := r.Unpack(nil)
		_ = r.Close()
		if err != nil || len(files) == 0 {
			t.Errorf("%s: %d files, %v", filepath.Base(path), len(files), err)
		}
	}
}
