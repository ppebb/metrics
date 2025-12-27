package main

import (
	"os"
	"slices"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	check(err)

	return info.Mode() == os.ModeSymlink
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	check(err)

	return info.IsDir()
}

func stringBeginsWith(str string, sub string) bool {
	subLen := len(sub)

	if len(str) < len(sub) {
		return false
	}

	return str[:subLen] == sub
}

func shouldSkipLang(lang string) bool {
	return lang == "Unknown" || lang == "Text" || lang == "Markdown" || slices.Contains(config.Ignore.Langs, lang)
}
