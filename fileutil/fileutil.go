package fileutil

import (
	"path/filepath"
	"strings"
)

// GetLowerBaseAndExt returns the lowercase base name (without extension)
// and the lowercase extension (including the dot) for the provided path.
func GetLowerBaseAndExt(path string) (base, ext string) {
	file := filepath.Base(path)
	ext = strings.ToLower(filepath.Ext(file))
	base = strings.ToLower(strings.TrimSuffix(file, ext))
	return
}

// HasExtCI reports whether path ends with ext, case-insensitive.
// ext should include the dot (e.g. ".cnv").
func HasExtCI(path, ext string) bool {
	return strings.HasSuffix(strings.ToLower(path), strings.ToLower(ext))
}

// ChangeExt replaces the extension of path with newExt. newExt may be
// provided with or without a leading dot.
func ChangeExt(path, newExt string) string {
	if newExt == "" {
		return path
	}
	if !strings.HasPrefix(newExt, ".") {
		newExt = "." + newExt
	}
	oldExt := filepath.Ext(path)
	if oldExt == "" {
		return path + newExt
	}
	return path[:len(path)-len(oldExt)] + newExt
}

// ChangeExtIfSuffixCI changes the extension to newExt only if the path
// currently has oldExt (case-insensitive).
func ChangeExtIfSuffixCI(path, oldExt, newExt string) string {
	if HasExtCI(path, oldExt) {
		return ChangeExt(path, newExt)
	}
	return path
}
