// Copyright 2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package rmdoc_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/altessa-s/rmdoc-go"
)

func TestVersionIsTheLibrarys(t *testing.T) {
	if got := rmdoc.CVersion(); got != rmdoc.Version {
		t.Fatalf("Version is %q, the C library says %q", rmdoc.Version, got)
	}
}

func TestLimits(t *testing.T) {
	d := rmdoc.DefaultLimits()
	want := rmdoc.Limits{
		MaximumEntryCount:          math.MaxUint16,
		MaximumEntrySize:           math.MaxUint32,
		MaximumTotalSize:           math.MaxInt64,
		MaximumCompressionRatio:    200,
		RatioThreshold:             1 << 20,
		MaximumCompressedEntrySize: math.MaxUint32,
		MaximumCompressedTotalSize: math.MaxInt64,
		MaximumDirectorySize:       math.MaxInt64,
		MaximumNameLength:          math.MaxUint16,
	}
	if d != want {
		t.Fatalf("DefaultLimits() = %+v, want %+v", d, want)
	}
	f := rmdoc.FormatLimits()
	want.MaximumCompressionRatio, want.RatioThreshold = 1032, 0
	if f != want {
		t.Fatalf("FormatLimits() = %+v, want %+v", f, want)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name   string
		layout rmdoc.Layout
		want   rmdoc.Status
	}{
		{"a1b2.content", rmdoc.LayoutDevice, rmdoc.StatusOK},
		{"a1b2/p1.rm", rmdoc.LayoutDevice, rmdoc.StatusOK},
		{"a1b2/p1/x.rm", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"a1b2/p1/x.rm", rmdoc.LayoutPackage, rmdoc.StatusOK},
		{".hidden", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"META-INF/.x", rmdoc.LayoutPackage, rmdoc.StatusOK},
		{"../escape", rmdoc.LayoutPackage, rmdoc.StatusInvalidName},
		{"", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"a//b", rmdoc.LayoutPackage, rmdoc.StatusInvalidName},
		{"a\\b", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"nul\x00", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"caf\xe9", rmdoc.LayoutDevice, rmdoc.StatusInvalidName},
		{"caf\u00e9.pdf", rmdoc.LayoutDevice, rmdoc.StatusOK},
		{"a1b2", rmdoc.Layout(7), rmdoc.StatusInvalidArgument},
	}
	for _, tt := range tests {
		err := rmdoc.ValidateName(tt.name, tt.layout)
		if tt.want == rmdoc.StatusOK {
			if err != nil {
				t.Errorf("ValidateName(%q, %v) = %v", tt.name, tt.layout, err)
			}
			continue
		}
		var e *rmdoc.Error
		if !errors.As(err, &e) || e.Status != tt.want {
			t.Errorf("ValidateName(%q, %v) = %v, want %v", tt.name, tt.layout, err, tt.want)
		}
	}
}

func TestValidateNameReportsTheName(t *testing.T) {
	long := strings.Repeat("x", 300) + "/.y"
	e := wantError(t, rmdoc.ValidateName(long, rmdoc.LayoutDevice), rmdoc.StatusInvalidName)
	if e.NameLength != len(long) || e.Name != long[:255] {
		t.Fatalf("name %d bytes of %d, want 255 of %d", len(e.Name), e.NameLength, len(long))
	}
	if !strings.HasSuffix(e.Error(), `...)`) {
		t.Fatalf("a cut name is not marked as cut: %s", e)
	}
}

func TestDocumentID(t *testing.T) {
	tests := map[string]string{
		"a1b2/p1.rm":      "a1b2",
		"a1b2.content":    "a1b2",
		"a1b2.x.y":        "a1b2",
		"a1b2":            "a1b2",
		"a1b2/p.1/x":      "a1b2",
		"":                "",
		"/x":              "",
		".content":        "",
		"\u00e9t\u00e9.x": "\u00e9t\u00e9",
	}
	for name, want := range tests {
		if got := rmdoc.DocumentID(name); got != want {
			t.Errorf("DocumentID(%q) = %q, want %q", name, got, want)
		}
		if got := (rmdoc.Entry{Name: name}).DocumentID(); got != want {
			t.Errorf("Entry{%q}.DocumentID() = %q, want %q", name, got, want)
		}
	}
}

func TestEnumStrings(t *testing.T) {
	tests := map[string]string{
		rmdoc.LayoutDevice.String():       "device",
		rmdoc.LayoutPackage.String():      "package",
		rmdoc.Layout(9).String():          "Layout(9)",
		rmdoc.CompressionStore.String():   "store",
		rmdoc.CompressionDeflate.String(): "deflate",
		rmdoc.CompressionDefault.String(): "default",
		rmdoc.Compression(3).String():     "Compression(3)",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
	if rmdoc.CompressionStore != 0 || rmdoc.CompressionDeflate != 8 || rmdoc.CompressionDefault != 65535 {
		t.Errorf("compression numbers are not the ZIP ones")
	}
}
