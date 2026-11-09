package info

import "path/filepath"

func globPaths(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
