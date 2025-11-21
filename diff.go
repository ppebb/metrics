package main

import (
	"fmt"
	"os"
	"path"

	"github.com/go-enry/go-enry/v2"
)

type IntIntPair struct {
	lines int
	bytes int
}

type Diff struct {
	File    string
	Added   IntIntPair
	Removed IntIntPair
}

func (diff Diff) should_skip(repo *Repo) bool {
	if stored, ok := repo.FileSkipMap[diff.File]; ok {
		return stored
	}

	ret := false

	fpath := path.Join(repo.Path, diff.File)

	fe := file_exists(fpath)
	sy := fe && is_symlink(fpath)
	di := fe && is_directory(fpath)
	if !fe || sy || di {
		log(LOG_INFO, repo, fmt.Sprintf("Skipping path %s, exists: %t, symlink: %t, dir: %t", fpath, fe, sy, di))
		// If the file doesn't exist, keep checking because sometimes it shows
		// up later?? May have to do with renames...
		return true
	} else if repo.skip_file_name(diff.File) {
		ret = true
	} else {
		data, err := os.ReadFile(fpath)
		check(err)

		if repo.skip_file_data(diff.File, data) {
			ret = true
		}
	}

	repo.FileSkipMap[diff.File] = ret
	return ret
}

func (diff Diff) get_languages(repo *Repo) []string {
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
