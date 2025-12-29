package main

import (
	"fmt"
	"os"
	"path"

	"github.com/go-enry/go-enry/v2"
)

type LineBytePair struct {
	lines int
	bytes int
}

type Diff struct {
	File    string
	Added   LineBytePair
	Removed LineBytePair
}

func (diff Diff) shouldSkip(repo *Repo) bool {
	if stored, ok := repo.FileSkipMap[diff.File]; ok {
		return stored
	}

	ret := false

	fpath := path.Join(repo.Path, diff.File)

	fe := fileExists(fpath)
	sy := fe && isSymlink(fpath)
	di := fe && isDirectory(fpath)
	if !fe || sy || di {
		log(Info, repo, fmt.Sprintf("Skipping path %s, exists: %t, symlink: %t, dir: %t", fpath, fe, sy, di))
		// If the file doesn't exist, keep checking because sometimes it shows
		// up later?? May have to do with renames...
		return true
	} else if repo.shouldSkipFileByName(diff.File) {
		ret = true
	} else {
		data, err := os.ReadFile(fpath)
		check(err)

		if repo.skipFileByData(diff.File, data) {
			ret = true
		}
	}

	repo.FileSkipMap[diff.File] = ret
	return ret
}

func (diff Diff) getLanguages(repo *Repo) []string {
	if stored, ok := repo.FileLangMap[diff.File]; ok {
		return stored
	}

	fpath := path.Join(repo.Path, diff.File)

	data, err := os.ReadFile(fpath)
	check(err)

	langs := enry.GetLanguages(diff.File, data)
	repo.FileLangMap[diff.File] = langs

	return langs
}
