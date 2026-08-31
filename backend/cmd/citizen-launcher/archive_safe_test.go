package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarTestEntry struct {
	name string
	link string
	typ  byte
	data []byte
	mode int64
}

func makeTarGz(t *testing.T, entries []tarTestEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h := &tar.Header{Name: e.name, Linkname: e.link, Typeflag: e.typ, Mode: mode, Size: int64(len(e.data))}
		if e.typ == tar.TypeSymlink || e.typ == tar.TypeLink || e.typ == tar.TypeDir {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if len(e.data) != 0 {
			if _, err := tw.Write(e.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractTarArchiveSafeRegularAndSymlink(t *testing.T) {
	archive := makeTarGz(t, []tarTestEntry{
		{name: "runner/bin/wine", typ: tar.TypeReg, data: []byte("wine"), mode: 0o755},
		{name: "runner/bin/wine64", typ: tar.TypeSymlink, link: "wine"},
	})
	dst := t.TempDir()
	if err := extractTarArchiveSafe(archive, dst); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "runner", "bin", "wine")); err != nil || string(b) != "wine" {
		t.Fatalf("regular file missing/corrupt: %q %v", b, err)
	}
	link, err := os.Readlink(filepath.Join(dst, "runner", "bin", "wine64"))
	if err != nil || link != "wine" {
		t.Fatalf("safe symlink missing: %q %v", link, err)
	}
}

func TestExtractTarArchiveSafeRejectsTraversal(t *testing.T) {
	for _, entries := range [][]tarTestEntry{
		{{name: "../escape", typ: tar.TypeReg, data: []byte("x")}},
		{{name: "runner/link", typ: tar.TypeSymlink, link: "../../escape"}},
		{{name: "/absolute", typ: tar.TypeReg, data: []byte("x")}},
	} {
		if err := extractTarArchiveSafe(makeTarGz(t, entries), t.TempDir()); err == nil {
			t.Fatalf("unsafe tar accepted: %#v", entries)
		}
	}
}
