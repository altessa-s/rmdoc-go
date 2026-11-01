// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/altessa-s/rmdoc-go"
)

func ExamplePack() {
	archive, err := rmdoc.Pack([]rmdoc.File{
		{Name: "a1b2.metadata", Data: []byte(`{"visibleName":"Notes"}`)},
		{Name: "a1b2/p1.rm", Data: []byte("page")},
	})
	if err != nil {
		log.Fatal(err)
	}
	reader, err := rmdoc.OpenReader(archive, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	for _, e := range reader.Entries() {
		fmt.Printf("%s: %d bytes, document %s\n", e.Name, e.UncompressedSize, e.DocumentID())
	}
	// Output:
	// a1b2.metadata: 23 bytes, document a1b2
	// a1b2/p1.rm: 4 bytes, document a1b2
}

func ExampleReader_ForEach() {
	archive, err := rmdoc.Pack([]rmdoc.File{
		{Name: "a1b2.content", Data: []byte(`{"pageCount":1}`)},
		{Name: "a1b2/p1.rm", Data: []byte("page one")},
	})
	if err != nil {
		log.Fatal(err)
	}
	reader, err := rmdoc.OpenReader(archive, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	pages := map[string][]byte{}
	onlyPages := func(e rmdoc.Entry) bool { return filepath.Ext(e.Name) == ".rm" }
	err = reader.ForEach(onlyPages, func(e rmdoc.Entry, data []byte) error {
		// data is valid only during this call: keep a copy.
		pages[e.Name] = bytes.Clone(data)
		return nil
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q\n", pages["a1b2/p1.rm"])
	// Output:
	// "page one"
}

func ExampleReader_Unpack() {
	archive, err := rmdoc.Pack([]rmdoc.File{
		{Name: "a1b2.metadata", Data: []byte("{}")},
		{Name: "c3d4.metadata", Data: []byte("{}")},
		{Name: "c3d4/p1.rm", Data: []byte("page")},
	})
	if err != nil {
		log.Fatal(err)
	}
	reader, err := rmdoc.OpenReader(archive, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	ids, err := reader.DocumentIDs()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ids)
	files, err := reader.Unpack(func(e rmdoc.Entry) bool { return e.DocumentID() == "c3d4" })
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range files {
		fmt.Println(f.Name)
	}
	// Output:
	// [a1b2 c3d4]
	// c3d4.metadata
	// c3d4/p1.rm
}

func ExampleCreateWriter() {
	dir, err := os.MkdirTemp("", "rmdoc")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	staging := filepath.Join(dir, "notes.rmdoc.partial")
	writer, err := rmdoc.CreateWriter(staging, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer writer.Close()
	page := bytes.Repeat([]byte("stroke "), 1000)
	if err := writer.Add(rmdoc.File{Name: "a1b2/p1.rm", Data: page}, rmdoc.CompressionDefault); err != nil {
		fmt.Println(err)
		return
	}
	if err := writer.Finish(); err != nil {
		fmt.Println(err)
		return
	}
	// Only a finished archive takes the final name.
	final := filepath.Join(dir, "notes.rmdoc")
	if err := os.Rename(staging, final); err != nil {
		fmt.Println(err)
		return
	}
	reader, err := rmdoc.OpenReaderFile(final, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer reader.Close()
	e := reader.Entries()[0]
	fmt.Println(e.Name, e.Method, e.UncompressedSize, e.CompressedSize < e.UncompressedSize)
	// Output:
	// a1b2/p1.rm 8 7000 true
}

func ExampleValidateName() {
	for _, name := range []string{"a1b2/p1.rm", "a1b2/p1/x.rm", "../escape", ".hidden"} {
		err := rmdoc.ValidateName(name, rmdoc.LayoutDevice)
		fmt.Println(name, errors.Is(err, rmdoc.ErrInvalidName))
	}
	// Output:
	// a1b2/p1.rm false
	// a1b2/p1/x.rm true
	// ../escape true
	// .hidden true
}

func ExampleDocumentID() {
	fmt.Println(rmdoc.DocumentID("a1b2/p1.rm"), rmdoc.DocumentID("a1b2.content"))
	// Output:
	// a1b2 a1b2
}

func ExampleError() {
	_, err := rmdoc.OpenReader([]byte("not a zip"), nil)
	if errors.Is(err, rmdoc.ErrNotAnArchive) {
		fmt.Println(err)
	}
	// Output:
	// rmdoc: not an archive
}

func ExampleLimits() {
	limits := rmdoc.DefaultLimits()
	limits.MaximumEntrySize = 1 << 20
	archive, err := rmdoc.Pack([]rmdoc.File{{Name: "a1b2/p1.rm", Data: make([]byte, 2<<20)}})
	if err != nil {
		log.Fatal(err)
	}
	reader, err := rmdoc.OpenReader(archive, &limits)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	_, err = reader.Unpack(nil)
	var e *rmdoc.Error
	if errors.As(err, &e) && e.Status == rmdoc.StatusEntryTooLarge {
		fmt.Println(e.Name, e.Actual, e.Limit)
	}
	// Output:
	// a1b2/p1.rm 2097152 1048576
}
