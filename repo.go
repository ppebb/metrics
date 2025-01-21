package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

func repository_path(repository string) string {
	splits := strings.Split(repository, "/")

	if len(splits) != 2 {
		panic(
			fmt.Sprintf(
				"Improper repository provided: %s. Ensure repositories follow the format author/repo",
				repository,
			),
		)
	}

	return path.Join(config.Location, splits[1])
}

func check_repository(repository string) map[string]int {
	repo_path := repository_path(repository)

	if !file_exists(repo_path) {
		fmt.Printf("Cloning repository %s\n", repository)
		run_git_sync(config.Location, "clone", "https://github.com/"+repository)
	} else {
		fmt.Printf("Pulling repository %s at %s\n", repository, repo_path)
		run_git_sync(repo_path, "pull")
	}

	vendored_filters := repo_vendored_filters(repo_path)
	repo_files := repo_files(repo_path)

	ret := map[string]int{}
	for _, repo_file := range repo_files {
		fpath := path.Join(repo_path, repo_file)

		if !file_exists(fpath) || is_symlink(fpath) || is_directory(fpath) {
			continue
		}

		if config.Ignore.Vendor && repo_check_path_vendored(repo_file, vendored_filters) {
			fmt.Printf("Skipping vendored file %s\n", repo_file)
			continue
		}

		if config.Ignore.Dotfiles && enry.IsDotFile(repo_file) {
			fmt.Printf("Skipping dotfile file %s\n", repo_file)
			continue
		}

		if config.Ignore.Configuration && enry.IsConfiguration(repo_file) {
			fmt.Printf("Skipping config file %s\n", repo_file)
			continue
		}

		if config.Ignore.Image && enry.IsImage(repo_file) {
			fmt.Printf("Skipping image file %s\n", repo_file)
			continue
		}

		if config.Ignore.Test && enry.IsTest(repo_file) {
			fmt.Printf("Skipping test file %s\n", repo_file)
			continue
		}

		data, err := os.ReadFile(fpath)
		check(err)

		if config.Ignore.Binary && enry.IsBinary(data) {
			fmt.Printf("Skipping binary file %s\n", repo_file)
			continue
		}

		if config.Ignore.Generated && enry.IsGenerated(repo_file, data) {
			fmt.Printf("Skipping generated file %s\n", repo_file)
			continue
		}

		langs := enry.GetLanguages(repo_file, data)
		if len(langs) > 1 {
			fmt.Printf("Potentially multiple languages found for file %s: %s\n", fpath, langs)
		}

		ret[langs[0]] += bytes.Count(data, []byte{'\n'})
	}

	return ret
}
