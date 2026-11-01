#!/usr/bin/env bash
# Copyright 2026 ALTESSA SOLUTIONS INC.
# Use of this source code is governed by an MIT-style license that can be
# found in the LICENSE file.

# Re-vendor rmdoc-lib into internal/clib: the library sources and headers, the
# miniz codec it compiles in, its licence and notices, the licence of its
# generated Unicode tables, and the tag they were taken at. The tests and tools
# of the C library stay behind; this module tests through its own API.
#
#   scripts/update-clib.sh ../rmdoc-lib
#
# miniz lands in internal/clib/miniz and not under a directory named vendor:
# the Go module zip leaves out every file below a vendor directory, so a
# vendor path would vanish from the published module. The two includes that
# name ../vendor/miniz/ are rewritten to ../miniz/, and nothing else changes.
#
# The new tree is assembled and checked beside the old one and swapped in
# whole: on any failure the previous tree and digest stay as they were.
# clib_digest.h in the package root is rewritten with a digest of the vendored
# files, because Go's build cache tracks the package directory and not
# internal/clib; the digest is what makes a change down there rebuild the
# package.

set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <path-to-rmdoc-lib>" >&2
    exit 2
fi

source_dir=$(cd "$1" && pwd -P)
module_dir=$(cd "$(dirname "$0")/.." && pwd -P)
internal="$module_dir/internal"
clib="$internal/clib"

# The source and the destination must not overlap, or the copy reads what the
# swap removes.
case "$source_dir/" in "$module_dir/"*)
    echo "refusing: $source_dir lies inside $module_dir" >&2
    exit 1 ;;
esac
case "$module_dir/" in "$source_dir/"*)
    echo "refusing: $module_dir lies inside $source_dir" >&2
    exit 1 ;;
esac

if command -v sha256sum > /dev/null; then
    sha256() { sha256sum | cut -d' ' -f1; }
elif command -v shasum > /dev/null; then
    sha256() { shasum -a 256 | cut -d' ' -f1; }
else
    echo "neither sha256sum nor shasum is installed" >&2
    exit 1
fi

sources=(rmd_codec rmd_io rmd_miniz rmd_reader rmd_support rmd_unicode rmd_writer)
private=(rmd_internal rmd_miniz rmd_unicode_data)
headers=(export rmdoc)
codec=(miniz.c miniz.h LICENSE.txt)
# The files that include miniz, each by exactly one ../vendor/miniz/ line.
codec_users=(src/rmd_miniz.c src/rmd_miniz.h)

mkdir -p "$internal"
staging=$(mktemp -d "$internal/.clib-staging.XXXXXX")
retired="$internal/.clib-retired.$$"
digest_tmp="$module_dir/.clib_digest.h.$$"
finished=false

cleanup() {
    local status=$?
    # Stopped anywhere in the swap: the previous tree goes back in place.
    if [[ $finished == false && -d "$retired" ]]; then
        if [[ -e "$clib" ]]; then
            rm -rf "$clib"
        fi
        mv "$retired" "$clib"
    fi
    rm -rf "$staging" "$digest_tmp"
    exit "$status"
}
trap cleanup EXIT

mkdir -p "$staging/src" "$staging/include/rmdoc" "$staging/miniz"
for name in "${sources[@]}"; do
    cp "$source_dir/src/$name.c" "$staging/src/"
done
for name in "${private[@]}"; do
    cp "$source_dir/src/$name.h" "$staging/src/"
done
for name in "${headers[@]}"; do
    cp "$source_dir/include/rmdoc/$name.h" "$staging/include/rmdoc/"
done
for name in "${codec[@]}"; do
    cp "$source_dir/vendor/miniz/$name" "$staging/miniz/"
done
cp "$source_dir/LICENSE" "$source_dir/COPYRIGHT" "$source_dir/THIRD-PARTY.md" \
    "$source_dir/UNICODE-LICENSE.txt" "$staging/"
git -C "$source_dir" describe --tags --always --dirty > "$staging/VERSION"

# Point the codec includes at miniz/. Each file must hold the include exactly
# once, before and none after, or the library has changed how it reaches the
# codec and this script must change with it.
for file in "${codec_users[@]}"; do
    if [[ $(grep -c '"\.\./vendor/miniz/' "$staging/$file") -ne 1 ]]; then
        echo "$file does not include ../vendor/miniz/ exactly once" >&2
        exit 1
    fi
    sed 's|"\.\./vendor/miniz/|"../miniz/|' "$staging/$file" > "$staging/$file.tmp"
    mv "$staging/$file.tmp" "$staging/$file"
    if grep -q 'vendor/miniz' "$staging/$file"; then
        echo "$file still names vendor/miniz" >&2
        exit 1
    fi
done

# Every file this module compiles or ships must be there and not empty; a
# source the library gains or loses means the wrappers change first.
expected=(VERSION LICENSE COPYRIGHT THIRD-PARTY.md UNICODE-LICENSE.txt)
for name in "${sources[@]}"; do expected+=("src/$name.c"); done
for name in "${private[@]}"; do expected+=("src/$name.h"); done
for name in "${headers[@]}"; do expected+=("include/rmdoc/$name.h"); done
for name in "${codec[@]}"; do expected+=("miniz/$name"); done
for file in "${expected[@]}"; do
    if [[ ! -s "$staging/$file" ]]; then
        echo "missing or empty: $file" >&2
        exit 1
    fi
done
for file in "$source_dir"/src/*.c; do
    name=$(basename "$file" .c)
    if [[ ! -f "$module_dir/clib_${name#rmd_}.c" ]]; then
        echo "$name.c has no wrapper; add clib_${name#rmd_}.c and list it here" >&2
        exit 1
    fi
done

version=$(cat "$staging/VERSION")
digest=$(cd "$staging" && find . -type f | LC_ALL=C sort | while read -r file; do
    printf '%s %s\n' "$(sha256 < "$file")" "$file"
done | sha256)

cat > "$digest_tmp" <<EOF
// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

// Generated by scripts/update-clib.sh; do not edit.
//
// The vendored rmdoc-lib, named by its tag and by a digest of every file
// under internal/clib. Each clib_*.c includes this file, so a change to the
// vendored sources changes a file in the package directory, which is what
// Go's build cache tracks.

#ifndef RMDOC_GO_CLIB_DIGEST_H
#define RMDOC_GO_CLIB_DIGEST_H

#define RMDOC_GO_CLIB_VERSION "$version"
#define RMDOC_GO_CLIB_DIGEST "$digest"

#endif
EOF

if [[ -d "$clib" ]]; then
    mv "$clib" "$retired"
fi
mv "$staging" "$clib"
mv "$digest_tmp" "$module_dir/clib_digest.h"
finished=true
rm -rf "$retired"

echo "vendored rmdoc-lib $version ($digest)"
