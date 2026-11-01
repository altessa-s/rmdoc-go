// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

// One reader is read from many goroutines at once, beside packing and
// writing of their own, and every call answers what it answers alone. Under
// -race this also shows reads leave the reader as it was.
func TestReaderIsSharedAcrossGoroutines(t *testing.T) {
	data := packed(t)
	r := open(t, data, nil)
	want := document()
	dir := t.TempDir()

	const goroutines, rounds = 16, 25
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range rounds {
				if err := oneRound(r, data, want, filepath.Join(dir, fmt.Sprintf("%d-%d.rmdoc", g, i))); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func oneRound(r *rmdoc.Reader, data []byte, want []rmdoc.File, path string) error {
	files, err := r.Unpack(nil)
	if err != nil {
		return err
	}
	if !equalFiles(files, want) {
		return fmt.Errorf("a concurrent Unpack answered differently")
	}
	var walked []rmdoc.File
	if err := r.ForEach(nil, func(e rmdoc.Entry, payload []byte) error {
		walked = append(walked, rmdoc.File{Name: e.Name, Data: bytes.Clone(payload)})
		return nil
	}); err != nil {
		return err
	}
	if !equalFiles(walked, want) {
		return fmt.Errorf("a concurrent ForEach answered differently")
	}
	ids, err := r.DocumentIDs()
	if err != nil {
		return err
	}
	if !slices.Equal(ids, []string{"a1b2", "c3d4"}) {
		return fmt.Errorf("concurrent document IDs %q", ids)
	}
	again, err := rmdoc.Pack(want)
	if err != nil {
		return err
	}
	if !bytes.Equal(again, data) {
		return fmt.Errorf("a concurrent Pack answered differently")
	}
	w, err := rmdoc.CreateWriter(path, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	if err != nil {
		return err
	}
	defer w.Close()
	for _, f := range want {
		if err := w.Add(f, rmdoc.CompressionDefault); err != nil {
			return err
		}
	}
	return w.Finish()
}

func equalFiles(a, b []rmdoc.File) bool {
	return slices.EqualFunc(a, b, func(x, y rmdoc.File) bool {
		return x.Name == y.Name && bytes.Equal(x.Data, y.Data)
	})
}

// Close races reads in flight: each read either finishes whole or is refused
// as closed, and the reader is released once, after the last of them.
func TestCloseRacesReads(t *testing.T) {
	data := packed(t)
	want := document()
	for range 20 {
		r, err := rmdoc.OpenReader(data, nil)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				files, err := r.Unpack(nil)
				switch {
				case err != nil:
					if !errors.Is(err, rmdoc.ErrInvalidArgument) {
						errs <- err
					}
				case !equalFiles(files, want):
					errs <- fmt.Errorf("a read racing Close answered differently")
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Close()
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err)
		}
	}
}

// One writer is shared: its calls take turns.
func TestWriterIsSharedAcrossGoroutines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.rmdoc")
	w, err := rmdoc.CreateWriter(path, rmdoc.CompressionDeflate, rmdoc.LayoutDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("doc/%02d.rm", i)
			if err := w.Add(rmdoc.File{Name: name, Data: bytes.Repeat([]byte{byte(i)}, 1000)}, rmdoc.CompressionDefault); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	r, err := rmdoc.OpenReaderFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	files, err := r.Unpack(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 32 {
		t.Fatalf("%d files", len(files))
	}
}
