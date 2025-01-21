package main

import (
	"os"
)

func file_exists(path string) bool {
	_, err := os.Stat(path)

	if err == nil {
		return true
	}

	return false
}

func is_symlink(path string) bool {
	info, err := os.Lstat(path)
	check(err)

	return info.Mode() == os.ModeSymlink
}

func is_directory(path string) bool {
	info, err := os.Stat(path)
	check(err)

	return info.IsDir()
}
