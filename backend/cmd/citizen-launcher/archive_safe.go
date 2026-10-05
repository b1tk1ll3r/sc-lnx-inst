package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxArchiveFileSize = int64(4 << 30) // 4 GiB per file is far above Wine/DXVK needs.
	maxArchiveTotal    = int64(12 << 30)
)

type deferredTarLink struct {
	name     string
	linkname string
	hard     bool
}

// extractTarArchiveSafe extracts trusted release archives without letting archive
// paths, hardlinks or symlinks escape the staging directory. Compression is
// decoded separately; tar entries themselves are always processed in Go.
func extractTarArchiveSafe(path, dir string) error {
	lower := strings.ToLower(path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var reader io.Reader = f
	var closeFns []func() error
	closeFns = append(closeFns, f.Close)

	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return err
		}
		reader = gz
		closeFns = append([]func() error{gz.Close}, closeFns...)
	case strings.HasSuffix(lower, ".tar.xz"):
		_ = f.Close()
		if !commandExists("xz") {
			return errors.New("xz wird zum Entpacken dieses Wine-Archivs benötigt")
		}
		cmd := exec.Command("xz", "-dc", "--", path)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return err
		}
		extractErr := extractTarReaderSafe(stdout, dir)
		if extractErr != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		waitErr := cmd.Wait()
		if extractErr != nil {
			return extractErr
		}
		if waitErr != nil {
			return fmt.Errorf("xz konnte Archiv nicht dekomprimieren: %s", compactDiagnostic(stderr.String()))
		}
		return nil
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tar.zstd"):
		_ = f.Close()
		if !commandExists("zstd") {
			return errors.New("zstd wird zum Entpacken dieses Wine-Archivs benötigt")
		}
		cmd := exec.Command("zstd", "-q", "-dc", "--", path)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return err
		}
		extractErr := extractTarReaderSafe(stdout, dir)
		if extractErr != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		waitErr := cmd.Wait()
		if extractErr != nil {
			return extractErr
		}
		if waitErr != nil {
			return fmt.Errorf("zstd konnte Archiv nicht dekomprimieren: %s", compactDiagnostic(stderr.String()))
		}
		return nil
	case strings.HasSuffix(lower, ".tar"):
		// plain tar
	default:
		for _, closeFn := range closeFns {
			_ = closeFn()
		}
		return fmt.Errorf("nicht unterstütztes Archivformat: %s", filepath.Base(path))
	}

	err = extractTarReaderSafe(reader, dir)
	for _, closeFn := range closeFns {
		if closeErr := closeFn(); err == nil && closeErr != nil {
			err = closeErr
		}
	}
	return err
}

func extractTarReaderSafe(r io.Reader, dir string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(r)
	var links []deferredTarLink
	var total int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name, target, err := safeArchiveTarget(root, h.Name)
		if err != nil {
			return err
		}
		if name == "." {
			continue
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if h.Size < 0 || h.Size > maxArchiveFileSize {
				return fmt.Errorf("Archivdatei zu groß: %q", h.Name)
			}
			total += h.Size
			if total > maxArchiveTotal {
				return errors.New("Archiv überschreitet das sichere Größenlimit")
			}
			if err := ensureNoSymlinkParents(root, target); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(h.Mode) & 0o755
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(out, tr, h.Size)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			links = append(links, deferredTarLink{name: name, linkname: h.Linkname})
		case tar.TypeLink:
			links = append(links, deferredTarLink{name: name, linkname: h.Linkname, hard: true})
		default:
			return fmt.Errorf("nicht unterstützter/unsicherer Tar-Eintrag %q (Typ %d)", h.Name, h.Typeflag)
		}
	}

	// Hardlinks first, then symlinks. Deferring links prevents a later regular
	// archive member from writing through a just-created symlink parent.
	for _, link := range links {
		if !link.hard {
			continue
		}
		_, dst, err := safeArchiveTarget(root, link.name)
		if err != nil {
			return err
		}
		_, src, err := safeArchiveTarget(root, link.linkname)
		if err != nil {
			return fmt.Errorf("unsicherer Hardlink %q -> %q: %w", link.name, link.linkname, err)
		}
		if err := ensureNoSymlinkParents(root, dst); err != nil {
			return err
		}
		if fi, err := os.Stat(src); err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("Hardlink-Ziel ist keine reguläre Datei: %q", link.linkname)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		_ = os.Remove(dst)
		if err := os.Link(src, dst); err != nil {
			return err
		}
	}
	var symlinks []string
	for _, link := range links {
		if link.hard {
			continue
		}
		_, dst, err := safeArchiveTarget(root, link.name)
		if err != nil {
			return err
		}
		if filepath.IsAbs(link.linkname) {
			return fmt.Errorf("absoluter Symlink im Archiv: %q -> %q", link.name, link.linkname)
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(dst), filepath.FromSlash(link.linkname)))
		if !pathInside(root, resolved) {
			return fmt.Errorf("Symlink verlässt Zielverzeichnis: %q -> %q", link.name, link.linkname)
		}
		if err := ensureNoSymlinkParents(root, dst); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		_ = os.Remove(dst)
		if err := os.Symlink(filepath.FromSlash(link.linkname), dst); err != nil {
			return err
		}
		symlinks = append(symlinks, dst)
	}
	// The per-link check above is textual. A chain such as `a -> ..` plus
	// `b -> a/..` passes it but resolves outside root, so re-check every link
	// against the real filesystem once all of them exist.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for _, dst := range symlinks {
		resolved, err := filepath.EvalSymlinks(dst)
		if err != nil {
			continue // dangling link: never followed outside root
		}
		if !pathInside(realRoot, resolved) {
			return fmt.Errorf("Symlink-Kette verlässt Zielverzeichnis: %q", dst)
		}
	}
	return nil
}

func safeArchiveTarget(root, archiveName string) (string, string, error) {
	name := filepath.Clean(filepath.FromSlash(strings.TrimSpace(archiveName)))
	if name == "" || name == "." {
		return ".", root, nil
	}
	if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("unsicherer Archivpfad: %q", archiveName)
	}
	target := filepath.Join(root, name)
	if !pathInside(root, target) {
		return "", "", fmt.Errorf("Archivpfad verlässt Zielverzeichnis: %q", archiveName)
	}
	return name, target, nil
}

func pathInside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func ensureNoSymlinkParents(root, target string) error {
	rel, err := filepath.Rel(root, filepath.Dir(target))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("Zielpfad liegt außerhalb des Entpackverzeichnisses")
	}
	cur := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Symlink als Archiv-Zwischenpfad wird abgelehnt: %s", cur)
		}
	}
	return nil
}
