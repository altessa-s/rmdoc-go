# rmdoc-go

The `.rmdoc` container, from Go: the ZIP archive a reMarkable document is exported and imported as. A reader and writer for its structure and its
files as opaque bytes; it does not parse the pages, the metadata JSON or anything else inside.

Idiomatic cgo bindings over **[rmdoc-lib](https://github.com/altessa-s/rmdoc-lib)**, the portable C23 library, whose sources are vendored into
this module together with the miniz deflate codec: `go get` builds it with no system library to install, not even zlib. The same container is
available for Swift as **[rmdoc-swift](https://github.com/altessa-s/rmdoc-swift)**.

Part of **[Slate Drive](https://slatedrive.app)** ([repository](https://github.com/altessa-s/slatedrive)), the macOS app that shows a reMarkable
library in Finder.

## Requirements

- Go 1.24 or newer, with cgo enabled (`CGO_ENABLED=1`, the default wherever a C compiler is found).
- A **C23 compiler**: clang 19 or newer, or GCC 13 or newer. Not every compiler that takes a C23 flag is one: the library is written with
  `nullptr`, typed enums and `constexpr`, which clang 18 does not fully read. Apple's clang in a current Xcode qualifies. MSVC does not, so on
  Windows that means clang.

The C is compiled with `-std=c2x`, the one spelling GCC 13, GCC 14 and clang 19 all take. Point `CC` at the compiler when the default one is too
old:

```sh
CC=clang-19 go build ./...
```

## Installing

```sh
go get github.com/altessa-s/rmdoc-go@v1.1.2
```

```go
import "github.com/altessa-s/rmdoc-go"   // package rmdoc
```

## Packing and listing

```go
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
```

`Pack` writes a stored, device-layout archive, deterministically: the same files give the same bytes, every timestamp is 1980-01-01, and entry
order is kept. There must be at least one file and at most 65,535, with valid and unique names, and nothing past 4 GiB (there is no ZIP64).

`OpenReader` copies the bytes it is given into C memory the reader owns, so the slice may be reused at once. `OpenReaderFile` maps the file
instead: keep it unchanged until `Close`, since a mapping is not a snapshot. Opening validates the whole central directory and every local
header, without inflating anything. `Entries` and `Len` answer that directory, copied into Go at open, and still answer it after `Close`.

## Reading payloads

```go
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
```

`ForEach` calls the selector, when there is one, for **every** entry first; then checks the reader's `Limits` against the selected entries; only
then inflates, checksums and hands over one payload at a time. Skipped payloads are neither inflated nor checked. A payload that fails its CRC-32
or does not inflate stops the walk, so the body may have seen earlier entries; a budget failure always comes before the first body call.

The slice the body receives points into C memory released as the body returns: copy what you keep. When the body returns an error, `ForEach`
stops and returns an `*Error` of `StatusCallback` that unwraps to it, so `errors.Is` and `errors.As` find your error. A panic in the selector or
the body stops C and is raised again on the calling goroutine after C returns and the handle is deleted. `runtime.Goexit` (including
`t.FailNow` in a test) stops C the same way and exits the calling goroutine after cleanup, running its deferred functions. The selector
and body run on separate goroutines, synchronously with `ForEach` or `Unpack`, without inheriting the calling goroutine's state.

```go
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
```

`Unpack` gathers the selected files into Go memory; they outlive the reader. `DocumentIDs` answers each document once, in order of first
appearance. A document ID is everything before a name's first `/`, or, for a name without one, before its first `.`.

## Writing a file

```go
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
```

`CreateWriter` creates or truncates the file and takes `CompressionStore` or `CompressionDeflate` as its own mode. `Add` takes
`CompressionDefault` for that mode, or either one for the entry; deflate is kept only when it makes the bytes smaller. `AddFile` streams a file
from disk in 1 MiB chunks, always stored. `Finish` writes the central directory and closes the file; `Close` releases the writer and never
finishes the archive, so write to a staging path and rename only after `Finish` succeeds.

A refused name or compression, or an empty `Finish`, leaves the writer usable. A failure once writing has begun makes it unusable: every later
call answers `ErrArchiveFinished`, and the partial file is to be discarded.

`LayoutDevice` allows at most two path components, none empty or starting with a dot; it is the only layout a reader accepts. `LayoutPackage`
allows deeper and dotted paths, for EPUB or Office ZIP output, but never an empty component or `..`.

## Limits

```go
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
```

A reader snapshots the `Limits` it was opened with; `nil` selects `DefaultLimits()`: 65,535 entries, 32-bit entry sizes, a 200:1 expansion above
1 MiB, 65,535 bytes per name, and no practical bound on the totals or the directory. `FormatLimits()` allows 1032:1, the most deflate reaches,
from the first byte. Listing applies only the entry count, directory size and name length; the other budgets apply to the selected payloads.

The reader refuses encryption, spanning, ZIP64, methods other than stored and deflate, local headers that disagree with the directory,
overlapping entries, trailing deflate bytes and checksum mismatches. Names are compared under Unicode canonical equivalence, so `é` and `e` with
a combining acute are the same name.

## Errors

```go
for _, name := range []string{"a1b2/p1.rm", "a1b2/p1/x.rm", "../escape", ".hidden"} {
	err := rmdoc.ValidateName(name, rmdoc.LayoutDevice)
	fmt.Println(name, errors.Is(err, rmdoc.ErrInvalidName))
}
```

Every failure is an `*Error` whose `Status` mirrors the C library's, matched with `errors.Is` against the sentinels: `ErrInvalidArgument`,
`ErrOutOfMemory`, `ErrIO`, `ErrInvalidName`, `ErrDuplicateName`, `ErrEmptyArchive`, `ErrFileTooLarge`, `ErrArchiveTooLarge`, `ErrNotAnArchive`,
`ErrMalformedArchive`, `ErrUnsupportedCompression`, `ErrChecksumMismatch`, `ErrArchiveFinished`, the limit failures (`ErrTooManyEntries`,
`ErrEntryTooLarge`, `ErrTotalSizeExceeded`, `ErrCompressionRatioExceeded`, `ErrCompressedEntryTooLarge`, `ErrCompressedTotalSizeExceeded`,
`ErrDirectoryTooLarge`, `ErrNameTooLong`), `ErrZIP64Unsupported`, `ErrEncryptedEntry`, `ErrSpannedArchive` and `ErrCallback`.

Beside the status, an `*Error` carries the C library's message, the entry `Name` it concerns (at most 255 bytes of it, with the full
`NameLength`), the `Actual` value and `Limit` of a limit failure, and, for `StatusCallback`, the callback's own error in `Err`. A closed reader or
writer answers `ErrInvalidArgument`. A count or a length the C library states and a Go `int` cannot hold is `ErrOutOfMemory`.

## Design

**Values out, handles only where C keeps state.** Entries, document IDs and unpacked files are plain Go data, copied out of C memory and released
before the call returns. `Reader` and `Writer` are handles because the C library keeps the archive and the open file between calls; both must be
closed, and neither has a finalizer.

**Borrowing is explicit.** C borrows the bytes of a reader until it is closed, so `OpenReader` gives it a C copy the reader owns. The payload of
a `ForEach` callback is the one value that points into C memory, and only for the length of the call.

**Pinned, not copied, on the way in.** `Pack` and `Writer.Add` hand C the caller's names and bytes in place, pinned with `runtime.Pinner` for the
length of the call; the array of records that points at them holds only pinned pointers. Paths cross as C strings, and a path holding a NUL byte
is refused before C sees it. The tests run clean under `GOEXPERIMENT=cgocheck2`.

**Callbacks through handles.** The selector and the body reach C as a `runtime/cgo.Handle` carried as an integer, and come back through two
exported Go functions. Caller code runs on separate goroutines while those functions wait. A Go error, panic or exit tells C to stop;
the error is returned, the panic raised again or `runtime.Goexit` called on the calling goroutine once C returns and the handle is deleted.

**Concurrency.** The C library holds no mutable global state. A `Reader` only reads, so any number of goroutines may use one at once, and
`Close` may be called at any time, even from a callback: reads in flight finish first, and the C reader is released after the last of them. A
`Writer` serializes its calls with a mutex.

**The whole contract.** All twenty-three functions rmdoc-lib exports are called. The version is held against the C library's own by a test,
and so are the status names.

## Layout

| Path | What is in it |
|---|---|
| `*.go` | The API: `model.go` (names, limits, entries), `reader.go`, `pack.go`, `writer.go`, `callbacks.go` (the exported callbacks), `errors.go`, `cgo.go`, `memory.go` |
| `callbacks_bridge.c` | The C callbacks that hand each entry to Go |
| `clib_*.c` | One line each, compiling a vendored C source into the package: cgo builds only the C files of the package directory |
| `clib_digest.h` | Generated: the vendored tag and a digest of `internal/clib`, so that a change there rebuilds the package |
| `internal/clib/` | The vendored rmdoc-lib: `src/`, `include/rmdoc/`, `miniz/`, its `LICENSE`, third-party notices, the Unicode data licence and the tag it was taken at in `VERSION` |
| `scripts/update-clib.sh` | Re-vendors rmdoc-lib from a checkout, swapping the tree in whole and rewriting the digest: `scripts/update-clib.sh ../rmdoc-lib` |
| `testdata/fuzz-seeds/` | Seeds for `FuzzReader`, the archives rmdoc-lib fuzzes from |

miniz is vendored under `internal/clib/miniz/` rather than a `vendor/` directory, because the Go module zip leaves out files below any directory
named `vendor`; the update script points the library's two codec includes there.

```sh
go test -race ./...
GOEXPERIMENT=cgocheck2 go test ./...
go test -run='^$' -fuzz=FuzzReader -fuzztime=30s .
go test -run='^$' -fuzz=FuzzName -fuzztime=30s .
```

## Bindings

- **[rmdoc-lib](https://github.com/altessa-s/rmdoc-lib)** — the C23 library these bindings compile.
- **[rmdoc-swift](https://github.com/altessa-s/rmdoc-swift)** — the same container as a Swift package.

## See also

- **[rmlines-go](https://github.com/altessa-s/rmlines-go)** — the `.rm` v6 page format for Go: pages, strokes, typed text and highlights, read and
  written.
- **[rmmarks-go](https://github.com/altessa-s/rmmarks-go)** — marked text for Go: which words a highlight covers, and how OCR from cropped strips
  returns to the page.
- **[rmrender-go](https://github.com/altessa-s/rmrender-go)** — reMarkable pages and documents drawn from Go to PNG and PDF.

## Acknowledgements

reMarkable does not document the `.rmdoc` container. This package is built on openly available information, the public ZIP specification among
it, and on the research of third-party developers who documented reMarkable's file formats.

## License

MIT — see [LICENSE](LICENSE). The vendored C under `internal/clib/` is rmdoc-lib, under its own MIT licence beside it; the miniz codec keeps its
[MIT licence](internal/clib/miniz/LICENSE.txt), and the generated Unicode tables carry the [Unicode data licence](internal/clib/UNICODE-LICENSE.txt).
See the [third-party notices](internal/clib/THIRD-PARTY.md).

reMarkable is a trademark of reMarkable AS. This project is not affiliated with, endorsed by or sponsored by reMarkable AS.
