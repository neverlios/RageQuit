package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findImage returns the path to the first image file found in dir.
// Supports ~/ prefix for the home directory.
func findImage(dir string) (string, error) {
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}

	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
			found = path
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scanning %s: %w", dir, err)
	}
	if found == "" {
		return "", fmt.Errorf("no image found in %s", dir)
	}
	return found, nil
}

func main() {}
