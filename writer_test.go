// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

// create answers a writer of a staging file in a temporary directory, closed
// when the test ends, and the file's path.
func create(t *testing.T, compression rmdoc.Compression, layout rmdoc.Layout) (*rmdoc.Writer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.rmdoc")
	w, err := rmdoc.CreateWriter(path, compression, layout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, path
}

func readFile(t *testing.T, path string) *rmdoc.Reader {
	t.Helper()
	r, err := rmdoc.OpenReaderFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestWriterRoundTrip(t *testing.T) {
	files := document()
	w, path := create(t, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	compressions := []rmdoc.Compression{
		rmdoc.CompressionDefault, rmdoc.CompressionStore, rmdoc.CompressionDefault, rmdoc.CompressionDeflate,
	}
	for i, f := range files {
		if err := w.Add(f, compressions[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	r := readFile(t, path)
	entries := r.Entries()
	// The page is long and regular: the writer's deflate keeps it. The
	// stored entry stays stored, and the empty file cannot shrink.
	if entries[2].Method != 8 || entries[2].CompressedSize >= entries[2].UncompressedSize {
		t.Errorf("the page is %+v", entries[2])
	}
	if entries[1].Method != 0 || entries[3].Method != 0 {
		t.Errorf("methods %d and %d, want stored", entries[1].Method, entries[3].Method)
	}
	got, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, files)
}

func TestWriterAddFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "page.rm")
	data := bytes.Repeat([]byte{1, 2, 3, 4, 5}, 300_000) // past one chunk
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	w, path := create(t, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	if err := w.AddFile("a1b2/p1.rm", source); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, w.AddFile("a1b2/p2.rm", filepath.Join(dir, "missing")), rmdoc.StatusIO)
	wantStatus(t, w.AddFile("a1b2/p2.rm", "x\x00"), rmdoc.StatusInvalidArgument)
	if err := w.Finish(); err != nil {
		t.Fatalf("finish after a missing source: %v", err)
	}
	r := readFile(t, path)
	if e := r.Entries()[0]; e.Method != 0 {
		t.Errorf("a file source is %+v, want stored", e)
	}
	got, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, got, []rmdoc.File{{Name: "a1b2/p1.rm", Data: data}})
}

func TestWriterRecoversFromRefusedNames(t *testing.T) {
	w, path := create(t, rmdoc.CompressionStore, rmdoc.LayoutDevice)
	wantStatus(t, w.Finish(), rmdoc.StatusEmptyArchive)
	wantStatus(t, w.Add(rmdoc.File{Name: "a/b/c"}, rmdoc.CompressionDefault), rmdoc.StatusInvalidName)
	wantStatus(t, w.Add(rmdoc.File{Name: "a"}, rmdoc.Compression(3)), rmdoc.StatusInvalidArgument)
	if err := w.Add(rmdoc.File{Name: "a", Data: []byte("x")}, rmdoc.CompressionDefault); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, w.Add(rmdoc.File{Name: "a"}, rmdoc.CompressionDefault), rmdoc.StatusDuplicateName)
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, w.Finish(), rmdoc.StatusArchiveFinished)
	wantStatus(t, w.Add(rmdoc.File{Name: "b"}, rmdoc.CompressionDefault), rmdoc.StatusArchiveFinished)
	if r := readFile(t, path); r.Len() != 1 {
		t.Fatalf("%d entries", r.Len())
	}
}

func TestPackageLayout(t *testing.T) {
	w, path := create(t, rmdoc.CompressionDeflate, rmdoc.LayoutPackage)
	if err := w.Add(rmdoc.File{Name: "META-INF/container.xml", Data: []byte("<x/>")}, rmdoc.CompressionDefault); err != nil {
		t.Fatal(err)
	}
	if err := w.Add(rmdoc.File{Name: "OEBPS/text/ch1.xhtml", Data: []byte("<p/>")}, rmdoc.CompressionDefault); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	// A reader takes the device layout only.
	_, err := rmdoc.OpenReaderFile(path, nil)
	wantStatus(t, err, rmdoc.StatusInvalidName)
}

func TestCreateWriterRefuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.rmdoc")
	_, err := rmdoc.CreateWriter(path, rmdoc.CompressionDefault, rmdoc.LayoutDevice)
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
	_, err = rmdoc.CreateWriter(path, rmdoc.CompressionStore, rmdoc.Layout(5))
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
	_, err = rmdoc.CreateWriter(filepath.Join(dir, "no", "such", "dir.rmdoc"), rmdoc.CompressionStore, rmdoc.LayoutDevice)
	wantStatus(t, err, rmdoc.StatusIO)
	_, err = rmdoc.CreateWriter("out\x00.rmdoc", rmdoc.CompressionStore, rmdoc.LayoutDevice)
	wantStatus(t, err, rmdoc.StatusInvalidArgument)
}

func TestClosedWriter(t *testing.T) {
	w, _ := create(t, rmdoc.CompressionStore, rmdoc.LayoutDevice)
	for range 2 {
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	}
	wantStatus(t, w.Add(rmdoc.File{Name: "a"}, rmdoc.CompressionDefault), rmdoc.StatusInvalidArgument)
	wantStatus(t, w.AddFile("a", "b"), rmdoc.StatusInvalidArgument)
	wantStatus(t, w.Finish(), rmdoc.StatusInvalidArgument)
}

// The writer's deflated output and Pack's stored output hold the same files.
func TestWriterAgreesWithPack(t *testing.T) {
	w, path := create(t, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	for _, f := range document() {
		if err := w.Add(f, rmdoc.CompressionDefault); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	written, err := readFile(t, path).Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	inMemory, err := open(t, packed(t), nil).Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	sameFiles(t, written, inMemory)
}
