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
	"strconv"
)

// Layout says which names an archive may hold.
type Layout uint32

// The layouts.
const (
	// LayoutDevice is the layout of a document: at most two path components,
	// none of them empty or starting with a dot. Readers accept only this
	// layout.
	LayoutDevice Layout = C.RMD_DEVICE
	// LayoutPackage allows deeper and dotted paths, for EPUB and Office ZIP
	// output, but never an empty component or "..".
	LayoutPackage Layout = C.RMD_PACKAGE
)

// String answers "device", "package", or the number of a layout the library
// does not know.
func (l Layout) String() string {
	switch l {
	case LayoutDevice:
		return "device"
	case LayoutPackage:
		return "package"
	}
	return "Layout(" + strconv.FormatUint(uint64(l), 10) + ")"
}

// Compression is how an entry's bytes are stored. The numbers are the ZIP
// method numbers, but for CompressionDefault.
type Compression uint16

// The compressions.
const (
	// CompressionStore keeps the bytes as they are.
	CompressionStore Compression = C.RMD_STORE
	// CompressionDeflate deflates the bytes, and keeps them as they are when
	// deflating does not make them smaller.
	CompressionDeflate Compression = C.RMD_DEFLATE
	// CompressionDefault, for [Writer.Add], is the writer's own compression.
	// A writer cannot be created with it.
	CompressionDefault Compression = C.RMD_DEFAULT_COMPRESSION
)

// String answers "store", "deflate", "default", or the number of a
// compression the library does not know.
func (c Compression) String() string {
	switch c {
	case CompressionStore:
		return "store"
	case CompressionDeflate:
		return "deflate"
	case CompressionDefault:
		return "default"
	}
	return "Compression(" + strconv.FormatUint(uint64(c), 10) + ")"
}

// Limits bound what reading an archive may cost. Listing applies only
// MaximumEntryCount, MaximumDirectorySize and MaximumNameLength; the others
// apply to the payloads a read selects.
type Limits struct {
	// MaximumEntryCount bounds the entries of the archive.
	MaximumEntryCount uint64
	// MaximumEntrySize bounds each payload, uncompressed.
	MaximumEntrySize uint64
	// MaximumTotalSize bounds the selected payloads together, uncompressed.
	MaximumTotalSize uint64
	// MaximumCompressionRatio bounds how far a payload larger than
	// RatioThreshold may expand. Zero is read as one.
	MaximumCompressionRatio uint64
	// RatioThreshold is the uncompressed size past which the ratio applies.
	RatioThreshold uint64
	// MaximumCompressedEntrySize bounds each payload as stored.
	MaximumCompressedEntrySize uint64
	// MaximumCompressedTotalSize bounds the selected payloads together, as
	// stored.
	MaximumCompressedTotalSize uint64
	// MaximumDirectorySize bounds the central directory in bytes.
	MaximumDirectorySize uint64
	// MaximumNameLength bounds each name in UTF-8 bytes.
	MaximumNameLength uint64
}

// DefaultLimits answers the limits a reader opened without any uses: 65,535
// entries, 32-bit entry sizes, a 200:1 expansion above 1 MiB, 65,535 bytes per
// name and no practical bound on the totals or the directory.
func DefaultLimits() Limits {
	return limitsFromC(C.rmd_limits_default())
}

// FormatLimits answers the limits of the ZIP format itself: the defaults, but
// with a 1032:1 expansion, the most deflate can reach, from the first byte.
func FormatLimits() Limits {
	return limitsFromC(C.rmd_limits_format())
}

func limitsFromC(l C.rmd_limits) Limits {
	return Limits{
		MaximumEntryCount:          uint64(l.maximum_entry_count),
		MaximumEntrySize:           uint64(l.maximum_entry_size),
		MaximumTotalSize:           uint64(l.maximum_total_size),
		MaximumCompressionRatio:    uint64(l.maximum_compression_ratio),
		RatioThreshold:             uint64(l.ratio_threshold),
		MaximumCompressedEntrySize: uint64(l.maximum_compressed_entry_size),
		MaximumCompressedTotalSize: uint64(l.maximum_compressed_total_size),
		MaximumDirectorySize:       uint64(l.maximum_directory_size),
		MaximumNameLength:          uint64(l.maximum_name_length),
	}
}

func (l *Limits) toC() C.rmd_limits {
	return C.rmd_limits{
		maximum_entry_count:           C.uint64_t(l.MaximumEntryCount),
		maximum_entry_size:            C.uint64_t(l.MaximumEntrySize),
		maximum_total_size:            C.uint64_t(l.MaximumTotalSize),
		maximum_compression_ratio:     C.uint64_t(l.MaximumCompressionRatio),
		ratio_threshold:               C.uint64_t(l.RatioThreshold),
		maximum_compressed_entry_size: C.uint64_t(l.MaximumCompressedEntrySize),
		maximum_compressed_total_size: C.uint64_t(l.MaximumCompressedTotalSize),
		maximum_directory_size:        C.uint64_t(l.MaximumDirectorySize),
		maximum_name_length:           C.uint64_t(l.MaximumNameLength),
	}
}

// File is a name and the bytes stored under it.
type File struct {
	// Name is the entry's path inside the archive, UTF-8, with '/' between
	// components.
	Name string
	// Data is the file's content.
	Data []byte
}

// Entry describes one entry of an archive as its central directory records
// it.
type Entry struct {
	// Name is the entry's path inside the archive.
	Name string
	// CompressedSize is the size of the payload as stored.
	CompressedSize uint32
	// UncompressedSize is the size of the payload once inflated.
	UncompressedSize uint32
	// CRC32 is the checksum of the uncompressed payload.
	CRC32 uint32
	// Method is the ZIP compression method: 0 for stored, 8 for deflate.
	Method uint16
}

// DocumentID answers the document the entry belongs to; see [DocumentID].
func (e Entry) DocumentID() string {
	return DocumentID(e.Name)
}

func entryFromC(e *C.rmd_entry) (Entry, error) {
	name, err := goString(e.name, "name length")
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		Name:             name,
		CompressedSize:   uint32(e.compressed_size),
		UncompressedSize: uint32(e.uncompressed_size),
		CRC32:            uint32(e.crc32),
		Method:           uint16(e.method),
	}, nil
}

// ValidateName reports whether name may be stored in an archive of the given
// layout: nonempty, well-formed UTF-8 of at most 65,535 bytes, free of control
// and format characters and backslashes, and a safe relative path. It fails
// with [ErrInvalidName], or [ErrInvalidArgument] for an unknown layout.
func ValidateName(name string, layout Layout) error {
	var pin runtime.Pinner
	defer pin.Unpin()
	var cerr C.rmd_error
	status := C.rmd_validate_name(pinnedStr(&pin, name), C.rmd_layout(layout), &cerr)
	return errorOf(status, &cerr)
}

// DocumentID answers the document a name belongs to: everything before the
// first '/', or, for a name without one, everything before its first '.'.
// So "abc/0.rm", "abc.content" and "abc" all belong to "abc". The name is not
// validated.
func DocumentID(name string) string {
	var pin runtime.Pinner
	defer pin.Unpin()
	id := C.rmd_document_id(pinnedStr(&pin, name))
	// The answer is a prefix of the name: only its length is read, and the
	// prefix is taken from the Go string.
	n := int(id.length)
	if id.bytes == nil || n < 0 || n > len(name) {
		return ""
	}
	return name[:n]
}
