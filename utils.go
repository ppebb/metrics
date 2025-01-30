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

func bin_search[T any](list []T, element T, cmp func(T, T) int) int {
	lo := 0
	hi := len(list) - 1

	for lo <= hi {
		i := (lo + hi) / 2

		c := cmp(element, list[i])

		if c == 0 {
			return i
		}

		if c < 0 {
			lo = i + 1
		} else {
			hi = i - 1
		}
	}

	return ^lo
}

func str_starts_with(str string, sub string) bool {
	subLen := len(sub)

	if len(str) < len(sub) {
		return false
	}

	return str[:subLen] == sub
}
