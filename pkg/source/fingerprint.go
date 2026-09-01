// Package source fingerprints workflow source trees for resumable runs.
package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Fingerprint describes a workflow source tree at one point in time.
type Fingerprint struct {
	Root      string
	Digest    string
	FileCount int
}

// Tree fingerprints every regular file and symlink underneath sourcePath. A
// directory source uses itself as the root; a single workflow file uses its
// containing directory so scripts and other sibling source files are covered.
// Excluded paths (and their SQLite journal sidecars) are omitted.
func Tree(sourcePath string, excludedPaths []string) (*Fingerprint, error) {
	if sourcePath == "" {
		return nil, fmt.Errorf("workflow source path is required")
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolving workflow source path: %w", err)
	}
	resolvedSource, err := filepath.EvalSymlinks(absSource)
	if err != nil {
		return nil, fmt.Errorf("resolving workflow source path: %w", err)
	}
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return nil, fmt.Errorf("stating workflow source path: %w", err)
	}
	root := resolvedSource
	if !info.IsDir() {
		root = filepath.Dir(resolvedSource)
	}

	excluded, err := normalizeExclusions(excludedPaths)
	if err != nil {
		return nil, err
	}

	type entry struct {
		path string
		info os.FileInfo
	}
	var entries []entry
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if excludedPath(path, excluded) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("unsupported workflow source entry %q (%s)", path, info.Mode().Type())
		}
		entries = append(entries, entry{path: path, info: info})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking workflow source tree: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	hash := sha256.New()
	for _, entry := range entries {
		rel, err := filepath.Rel(root, entry.path)
		if err != nil {
			return nil, fmt.Errorf("finding source relative path: %w", err)
		}
		rel = filepath.ToSlash(rel)
		if entry.info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(entry.path)
			if err != nil {
				return nil, fmt.Errorf("reading source symlink %q: %w", rel, err)
			}
			fmt.Fprintf(hash, "symlink\x00%s\x00%#o\x00%s\x00", rel, entry.info.Mode(), target)
			continue
		}

		fileHash := sha256.New()
		file, err := os.Open(entry.path)
		if err != nil {
			return nil, fmt.Errorf("opening source file %q: %w", rel, err)
		}
		_, copyErr := io.Copy(fileHash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("hashing source file %q: %w", rel, copyErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("closing source file %q: %w", rel, closeErr)
		}
		fmt.Fprintf(hash, "file\x00%s\x00%#o\x00%s\x00", rel, entry.info.Mode().Perm(), hex.EncodeToString(fileHash.Sum(nil)))
	}

	return &Fingerprint{Root: root, Digest: hex.EncodeToString(hash.Sum(nil)), FileCount: len(entries)}, nil
}

func normalizeExclusions(paths []string) ([]string, error) {
	excluded := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving source exclusion %q: %w", path, err)
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		excluded = append(excluded, filepath.Clean(abs))
	}
	return excluded, nil
}

func excludedPath(path string, excluded []string) bool {
	path = filepath.Clean(path)
	for _, candidate := range excluded {
		if path == candidate || strings.HasPrefix(path, candidate+"-wal") || strings.HasPrefix(path, candidate+"-shm") || strings.HasPrefix(path, candidate+"-journal") {
			return true
		}
	}
	return false
}
