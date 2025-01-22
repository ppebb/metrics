package main

import (
	"os"
	"path"

	"github.com/go-enry/go-enry/v2"
)

type Diff struct {
	File    string
	Added   uint
	Removed uint
}

func diff_should_skip(repo Repo, diff Diff) bool {
	if stored, ok := repo.FileSkipMap[diff.File]; ok {
		return stored
	}

	ret := false

	fpath := path.Join(repo.Path, diff.File)
	if repo_skip_file_name(repo, diff.File, fpath) {
		ret = true
	} else {
		data, err := os.ReadFile(fpath)
		check(err)

		if repo_skip_file_data(diff.File, data) {
			ret = true
		}
	}

	repo.FileSkipMap[diff.File] = ret
	return ret
}

func diff_get_language(repo Repo, diff Diff) string {
	if stored, ok := repo.FileLangMap[diff.File]; ok {
		return stored
	}

	fpath := path.Join(repo.Path, diff.File)

	data, err := os.ReadFile(fpath)
	check(err)

	lang := enry.GetLanguage(diff.File, data)
	repo.FileLangMap[diff.File] = lang

	return lang
}
